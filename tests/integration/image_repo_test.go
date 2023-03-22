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
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
)

func TestImageRepositoryAndAutomation(t *testing.T) {
	g := NewWithT(t)
	ctx := context.TODO()
	name := "image-repository"

	fullImageURL, ok := testRepos["podinfo:6.0.0"]
	if !ok {
		t.Fatal("no image present for podinfo")
	}

	fmt.Println(fullImageURL)
	imgArr := strings.Split(fullImageURL, ":")
	imageURL := imgArr[0]

	manifest := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: podinfo
  namespace: image-repository
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
        image: %s # {"$imagepolicy": "%s:podinfo"}
        readinessProbe:
          exec:
            command:
            - podcli
            - check
            - http
            - localhost:9898/readyz
          initialDelaySeconds: 5
          timeoutSeconds: 5
`, fullImageURL, name)

	repoUrl := getTransportURL(cfg.applicationRepository)
	client, err := getRepository(ctx, repoUrl, defaultBranch, cfg.defaultAuthOpts)
	g.Expect(err).ToNot(HaveOccurred())
	files := make(map[string]io.Reader)
	files["image-repository/podinfo.yaml"] = strings.NewReader(manifest)
	g.Expect(err).ToNot(HaveOccurred())
	err = commitAndPushAll(ctx, client, files, name)
	g.Expect(err).ToNot(HaveOccurred())

	err = setupNamespace(ctx, name, nsConfig{
		repoURL: repoUrl,
		path:    "./image-repository",
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer deleteNamespace(ctx, name)

	g.Eventually(func() bool {
		err := verifyGitAndKustomization(ctx, testEnv.Client, name, name)
		if err != nil {
			return false
		}
		return true
	}, 60*time.Second, 5*time.Second).Should(BeTrue())

	imageRepository := reflectorv1beta1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podinfo",
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, &imageRepository, func() error {
		imageRepository.Spec = reflectorv1beta1.ImageRepositorySpec{
			Image: imageURL,
			Interval: metav1.Duration{
				Duration: 1 * time.Minute,
			},
		}
		return nil
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer testEnv.Client.Delete(ctx, &imageRepository)

	imagePolicy := reflectorv1beta1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podinfo",
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, &imagePolicy, func() error {
		imagePolicy.Spec = reflectorv1beta1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: imageRepository.Name,
			},
			Policy: reflectorv1beta1.ImagePolicyChoice{
				SemVer: &reflectorv1beta1.SemVerPolicy{
					Range: "6.0.x",
				},
			},
		}
		return nil
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer testEnv.Client.Delete(ctx, &imagePolicy)

	imageAutomation := automationv1beta1.ImageUpdateAutomation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podinfo",
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, &imageAutomation, func() error {
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
			Update: &automationv1beta1.UpdateStrategy{
				Path:     "./image-repository",
				Strategy: automationv1beta1.UpdateStrategySetters,
			},
		}
		return nil
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer testEnv.Client.Delete(ctx, &imageAutomation)

	// Wait for image repository to be ready
	g.Eventually(func() bool {
		client, err := getRepository(ctx, repoUrl, name, cfg.defaultAuthOpts)
		if err != nil {
			return false
		}

		b, err := os.ReadFile(filepath.Join(client.Path(), name, "podinfo.yaml"))
		if err != nil {
			return false
		}
		if bytes.Contains(b, []byte(newVersion)) == false {
			return false
		}
		return true
	}, 120*time.Second, 5*time.Second).Should(BeTrue())
}
