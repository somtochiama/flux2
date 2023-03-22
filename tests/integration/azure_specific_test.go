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
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	. "github.com/onsi/gomega"
	giturls "github.com/whilp/git-urls"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	notiv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/pkg/runtime/events"
)

func TestNotification(t *testing.T) {
	g := NewWithT(t)
	// Currently, only azuredevops is supported
	if infraOpts.Provider != "azure" {
		fmt.Printf("Skipping event notification for %s as it is not supported.\n", infraOpts.Provider)
		return
	}

	ctx := context.TODO()
	name := "test-notification"

	// Start listening to eventhub with latest offset
	// TODO(somtochiama): Make here provider agnostic
	hub, err := eventhub.NewHubFromConnectionString(cfg.notificationURL)
	g.Expect(err).ToNot(HaveOccurred())
	c := make(chan string, 10)
	handler := func(ctx context.Context, event *eventhub.Event) error {
		c <- string(event.Data)
		return nil
	}
	runtimeInfo, err := hub.GetRuntimeInformation(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(runtimeInfo.PartitionIDs)).To(Equal(1))
	listenerHandler, err := hub.Receive(ctx, runtimeInfo.PartitionIDs[0], handler)
	g.Expect(err).ToNot(HaveOccurred())

	// Setup Flux resources
	manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: foobar`

	repoUrl := getTransportURL(cfg.applicationRepository)
	client, err := getRepository(ctx, repoUrl, defaultBranch, cfg.defaultAuthOpts)
	g.Expect(err).ToNot(HaveOccurred())
	files := make(map[string]io.Reader)
	files["configmap.yaml"] = strings.NewReader(manifest)
	err = commitAndPushAll(ctx, client, files, name)
	g.Expect(err).ToNot(HaveOccurred())
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, &secret, func() error {
		secret.StringData = map[string]string{
			"address": cfg.notificationURL,
		}
		return nil
	})
	defer testEnv.Client.Delete(ctx, &secret)

	provider := notiv1beta2.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, &provider, func() error {
		provider.Spec = notiv1beta2.ProviderSpec{
			Type:    "azureeventhub",
			Address: repoUrl,
			SecretRef: &meta.LocalObjectReference{
				Name: name,
			},
		}
		return nil
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer testEnv.Client.Delete(ctx, &provider)

	alert := notiv1beta2.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, &alert, func() error {
		alert.Spec = notiv1beta2.AlertSpec{
			ProviderRef: meta.LocalObjectReference{
				Name: provider.Name,
			},
			EventSources: []notiv1beta2.CrossNamespaceObjectReference{
				{
					Kind:      "Kustomization",
					Name:      name,
					Namespace: name,
				},
			},
		}
		return nil
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer testEnv.Client.Delete(ctx, &alert)

	modifyKsSpec := func(spec *kustomizev1.KustomizationSpec) {
		spec.HealthChecks = []meta.NamespacedObjectKindReference{
			{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "foobar",
				Namespace:  name,
			},
		}
	}
	err = setupNamespace(ctx, name, nsConfig{
		repoURL:      repoUrl,
		path:         "./",
		modifyKsSpec: modifyKsSpec,
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

	// Wait to read even from event hub
	g.Eventually(func() bool {
		select {
		case eventJson := <-c:
			event := &events.Event{}
			err := json.Unmarshal([]byte(eventJson), event)
			if err != nil {
				t.Logf("the received event type does not match Flux format, error: %v", err)
				return false
			}

			if event.InvolvedObject.Kind == kustomizev1.KustomizationKind &&
				strings.Contains(event.Message, "Health check passed") {
				return true
			}

			t.Logf("event received from '%s/%s': %s",
				event.InvolvedObject.Kind, event.InvolvedObject.Name, event.Message)
			return false
		default:
			return false
		}
	}, 60*time.Second, 1*time.Second).Should(BeTrue())
	err = listenerHandler.Close(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	err = hub.Close(ctx)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestAzureDevOpsCommitStatus(t *testing.T) {
	g := NewWithT(t)

	// Currently, only azuredevops is supported
	if infraOpts.Provider != "azure" {
		fmt.Printf("Skipping commit status test for %s as it is not supported.\n", infraOpts.Provider)
		return
	}

	ctx := context.TODO()
	name := "commit-status"
	manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: foobar`

	repoUrl := getTransportURL(cfg.applicationRepository)
	c, err := getRepository(ctx, repoUrl, defaultBranch, cfg.defaultAuthOpts)
	g.Expect(err).ToNot(HaveOccurred())
	files := make(map[string]io.Reader)
	files["configmap.yaml"] = strings.NewReader(manifest)
	err = commitAndPushAll(ctx, c, files, name)
	g.Expect(err).ToNot(HaveOccurred())

	modifyKsSpec := func(spec *kustomizev1.KustomizationSpec) {
		spec.HealthChecks = []meta.NamespacedObjectKindReference{
			{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "foobar",
				Namespace:  name,
			},
		}
	}
	err = setupNamespace(ctx, name, nsConfig{
		repoURL:      repoUrl,
		path:         "./",
		modifyKsSpec: modifyKsSpec,
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer deleteNamespace(ctx, name)
	g.Eventually(func() bool {
		err := verifyGitAndKustomization(ctx, testEnv.Client, name, name)
		if err != nil {
			return false
		}
		return true
	}, 10*time.Second, 1*time.Second)

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azuredevops-token",
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, &secret, func() error {
		secret.StringData = map[string]string{
			"token": cfg.gitPat,
		}
		return nil
	})
	defer testEnv.Delete(ctx, &secret)

	provider := notiv1beta2.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azuredevops",
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, &provider, func() error {
		provider.Spec = notiv1beta2.ProviderSpec{
			Type:    "azuredevops",
			Address: repoUrl,
			SecretRef: &meta.LocalObjectReference{
				Name: "azuredevops-token",
			},
		}
		return nil
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer testEnv.Delete(ctx, &provider)

	alert := notiv1beta2.Alert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azuredevops",
			Namespace: name,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, testEnv.Client, &alert, func() error {
		alert.Spec = notiv1beta2.AlertSpec{
			ProviderRef: meta.LocalObjectReference{
				Name: provider.Name,
			},
			EventSources: []notiv1beta2.CrossNamespaceObjectReference{
				{
					Kind:      "Kustomization",
					Name:      name,
					Namespace: name,
				},
			},
		}
		return nil
	})
	g.Expect(err).ToNot(HaveOccurred())
	defer testEnv.Delete(ctx, &alert)

	url, err := ParseGitAddress(repoUrl)
	g.Expect(err).ToNot(HaveOccurred())

	rev, err := c.Head()
	g.Expect(err).ToNot(HaveOccurred())

	connection := azuredevops.NewPatConnection(url.OrgURL, cfg.gitPat)
	client, err := git.NewClient(ctx, connection)
	g.Expect(err).ToNot(HaveOccurred())
	getArgs := git.GetStatusesArgs{
		Project:      &url.Project,
		RepositoryId: &url.Repo,
		CommitId:     &rev,
	}
	g.Eventually(func() bool {
		statuses, err := client.GetStatuses(ctx, getArgs)
		if err != nil {
			return false
		}
		if len(*statuses) != 1 {
			return false
		}
		return true
	}, 500*time.Second, 5*time.Second)
}

type AzureDevOpsURL struct {
	OrgURL  string
	Project string
	Repo    string
}

// TODO(somtochiama): move this into fluxcd/pkg and reuse in NC
func ParseGitAddress(s string) (AzureDevOpsURL, error) {
	var args AzureDevOpsURL
	u, err := giturls.Parse(s)
	if err != nil {
		return args, nil
	}

	scheme := u.Scheme
	if u.Scheme == "ssh" {
		scheme = "https"
	}

	id := strings.TrimLeft(u.Path, "/")
	id = strings.TrimSuffix(id, ".git")

	comp := strings.Split(id, "/")
	if len(comp) != 4 {
		return args, fmt.Errorf("invalid repository id %q", id)
	}

	args = AzureDevOpsURL{
		OrgURL:  fmt.Sprintf("%s://%s/%s", scheme, u.Host, comp[0]),
		Project: comp[1],
		Repo:    comp[3],
	}

	return args, nil

}
