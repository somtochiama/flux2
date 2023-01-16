/*
Copyright 2023 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	automationv1beta1 "github.com/fluxcd/image-automation-controller/api/v1beta1"
	reflectorv1beta1 "github.com/fluxcd/image-reflector-controller/api/v1beta1"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
)

func TestImageRepositoryAndAutomation(t *testing.T) {
	g := NewWithT(t)
	ctx := context.TODO()
	name := "image-repository-acr"
	repoUrl := cfg.applicationRepository.http
	oldVersion := "1.0.0"
	newVersion := "1.0.1"

	imageURL := fmt.Sprintf("%s/container/podinfo", cfg.dockerCred.url)
	// push the podinfo image to the container registry
	err := pushImagesFromURL(imageURL, "ghcr.io/stefanprodan/podinfo", []string{oldVersion, newVersion})
	g.Expect(err).ToNot(HaveOccurred())

	manifest := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: podinfo
  namespace: {{ .ns }}
spec:
  selector:
    matchLabels:
      app: podinfo
  template:
    metadata:
      labels:
        app: podinfo
    spec:
      containers:
      - name: podinfod
        image: stefanprodan/container/podinfo:1.0.0 # {"$imagepolicy": "{{ .name }}:podinfo"}
        readinessProbe:
          exec:
            command:
            - podcli
            - check
            - http
            - localhost:9898/readyz
          initialDelaySeconds: 5
          timeoutSeconds: 5
`

	c, _, err := getRepository(repoUrl, name, true, cfg.gitPat)
	g.Expect(err).ToNot(HaveOccurred())
	files := make(map[string]io.Reader)
	files["podinfo.yaml"] = strings.NewReader(manifest)
	g.Expect(err).ToNot(HaveOccurred())
	err = commitAndPushAll(c, files, name)
	g.Expect(err).ToNot(HaveOccurred())

	modifyKsSpec := func(spec *kustomizev1.KustomizationSpec) {
		spec.Images = []kustomize.Image{
			{
				Name:    "stefanprodan/container/podinfo",
				NewName: imageURL,
			},
		}
	}

	err = setupNamespace(ctx, name, nsConfig{
		repoURL:      repoUrl,
		path:         "./",
		modifyKsSpec: modifyKsSpec,
	})
	g.Expect(err).ToNot(HaveOccurred())

	g.Eventually(func() bool {
		err := verifyGitAndKustomization(ctx, testEnv.Client, name, name)
		if err != nil {
			return false
		}
		return true
	}, 60*time.Second, 5*time.Second)

	imageRepository := reflectorv1beta1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podinfo",
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, cfg.client, &imageRepository, func() error {
		imageRepository.Spec = reflectorv1beta1.ImageRepositorySpec{
			Image: fmt.Sprintf("%s/container/podinfo", cfg.dockerCred.url),
			Interval: metav1.Duration{
				Duration: 1 * time.Minute,
			},
		}
		return nil
	})
	g.Expect(err).ToNot(HaveOccurred())

	imagePolicy := reflectorv1beta1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podinfo",
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, cfg.client, &imagePolicy, func() error {
		imagePolicy.Spec = reflectorv1beta1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: imageRepository.Name,
			},
			Policy: reflectorv1beta1.ImagePolicyChoice{
				SemVer: &reflectorv1beta1.SemVerPolicy{
					Range: "1.0.x",
				},
			},
		}
		return nil
	})
	g.Expect(err).ToNot(HaveOccurred())

	imageAutomation := automationv1beta1.ImageUpdateAutomation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podinfo",
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, cfg.client, &imageAutomation, func() error {
		imageAutomation.Spec = automationv1beta1.ImageUpdateAutomationSpec{
			Interval: metav1.Duration{
				Duration: 1 * time.Minute,
			},
			SourceRef: automationv1beta1.CrossNamespaceSourceReference{
				Kind: "GitRepository",
				Name: name,
			},
			GitSpec: &automationv1beta1.GitSpec{
				Checkout: &automationv1beta1.GitCheckoutSpec{
					Reference: sourcev1.GitRepositoryRef{
						Branch: name,
					},
				},
				Commit: automationv1beta1.CommitSpec{
					Author: automationv1beta1.CommitUser{
						Email: "imageautomation@example.com",
						Name:  "imageautomation",
					},
				},
			},
		}
		return nil
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Wait for image repository to be ready
	g.Eventually(func() bool {
		_, repoDir, err := getRepository(repoUrl, name, false, cfg.gitPat)
		if err != nil {
			return false
		}

		b, err := os.ReadFile(filepath.Join(repoDir, "podinfo.yaml"))
		if err != nil {
			return false
		}
		if bytes.Contains(b, []byte(newVersion)) == false {
			return false
		}
		return true
	}, 120*time.Second, 5*time.Second)
}
