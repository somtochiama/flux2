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
	"context"
	"fmt"
	"github.com/fluxcd/pkg/git"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"strings"
	"testing"
	"time"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	. "github.com/onsi/gomega"
)

func TestFluxInstallation(t *testing.T) {
	g := NewWithT(t)
	ctx := context.TODO()
	g.Eventually(func() bool {
		err := verifyGitAndKustomization(ctx, testEnv.Client, "flux-system", "flux-system")
		if err != nil {
			return false
		}
		return true
	}, 60*time.Second, 5*time.Second)
}

func TestRepositoryCloning(t *testing.T) {
	ctx := context.TODO()
	branchName := "feature/branch"
	tagName := "v1"

	g := NewWithT(t)

	type testStruct struct {
		name      string
		refType   string
		cloneType git.TransportType
	}

	tests := []testStruct{
		{
			name:      "ssh-feature-branch",
			refType:   "branch",
			cloneType: git.SSH,
		},
		{
			name:      "ssh-v1",
			refType:   "tag",
			cloneType: git.SSH,
		},
	}

	// Not all cloud providers have repositories that support authentication with an accessToken
	// we don't run http tests for these.
	if cfg.gitPat != "" {
		httpTests := []testStruct{
			{
				name:      "https-feature-branch",
				refType:   "branch",
				cloneType: git.HTTP,
			},
			{
				name:      "https-v1",
				refType:   "tag",
				cloneType: git.HTTP,
			},
		}

		tests = append(tests, httpTests...)
	}

	t.Log("Creating application sources")
	url := getTransportURL(cfg.applicationRepository)
	client, err := getRepository(ctx, url, defaultBranch, cfg.defaultAuthOpts)
	g.Expect(err).ToNot(HaveOccurred())

	files := make(map[string]io.Reader)
	for _, tt := range tests {
		manifest := `
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: foobar
    `
		name := fmt.Sprintf("cloning-test/%s/configmap.yaml", tt.name)
		files[name] = strings.NewReader(manifest)
	}

	err = commitAndPushAll(ctx, client, files, branchName)
	g.Expect(err).ToNot(HaveOccurred())
	err = createTagAndPush(ctx, client.Path(), branchName, tagName, cfg.defaultAuthOpts)
	g.Expect(err).ToNot(HaveOccurred())

	t.Log("Verifying application-gitops namespaces")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ref := &sourcev1.GitRepositoryRef{
				Branch: branchName,
			}
			if tt.refType == "tag" {
				ref = &sourcev1.GitRepositoryRef{
					Tag: tagName,
				}
			}

			url := cfg.applicationRepository.http
			if tt.cloneType == git.SSH {
				url = cfg.applicationRepository.ssh
			}

			err := setupNamespace(ctx, tt.name, nsConfig{
				repoURL:    url,
				protocol:   tt.cloneType,
				objectName: tt.name,
				path:       fmt.Sprintf("./cloning-test/%s", tt.name),
				modifyGitSpec: func(spec *sourcev1.GitRepositorySpec) {
					spec.Reference = ref
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			//t.Cleanup(func() {
			//	err := deleteNamespace(ctx, tt.name)
			//	if err != nil {
			//		log.Printf("failed to delete resources in '%s' namespace", tt.name)
			//	}
			//})

			// Wait for configmap to be deployed
			g.Eventually(func() bool {
				err := verifyGitAndKustomization(ctx, testEnv.Client, tt.name, tt.name)
				if err != nil {
					fmt.Println(err)
					return false
				}
				return true
			}, 120*time.Second, 5*time.Second).Should(BeTrue())

			g.Eventually(func() bool {
				nn := types.NamespacedName{Name: "foobar", Namespace: tt.name}
				cm := &corev1.ConfigMap{}
				err = testEnv.Client.Get(ctx, nn, cm)
				if err != nil {
					return false
				}

				return true
			}, 120*time.Second, 5*time.Second).Should(BeTrue())

		})
	}
}
