/*
Copyright 2021 The Flux authors

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

	"github.com/fluxcd/test-infra/tftestenv"
	tfjson "github.com/hashicorp/terraform-json"
)

const (
	azureDevOpsKnownHosts = "ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H"
)

// createKubeConfigAKS constructs kubeconfig for an AKS cluster from the
// terraform state output at the given kubeconfig path.
func createKubeConfigAKS(ctx context.Context, state map[string]*tfjson.StateOutput, kcPath string) error {
	kubeconfigYaml, ok := state["kubeconfig"].Value.(string)
	if !ok || kubeconfigYaml == "" {
		return fmt.Errorf("failed to obtain kubeconfig from tf output")
	}
	return tftestenv.CreateKubeconfigAKS(ctx, kubeconfigYaml, kcPath)
}

func getTestConfigAKS(ctx context.Context, outputs map[string]*tfjson.StateOutput) (*testConfig, error) {
	fluxAzureSp := outputs["flux_azure_sp"].Value.(map[string]interface{})

	fleetInfraRepository := outputs["fleet_infra_repository"].Value.(map[string]interface{})
	applicationRepository := outputs["application_repository"].Value.(map[string]interface{})

	acr := outputs["acr"].Value.(map[string]interface{})
	eventHubSas := outputs["event_hub_sas"].Value.(string)

	sharedSopsId := outputs["sops_id"].Value.(string)

	kustomizeYaml := `
resources:
 - gotk-components.yaml
 - gotk-sync.yaml
patchesStrategicMerge:
 - |-
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: kustomize-controller
     namespace: flux-system
   spec:
    template:
      spec:
        containers:
        - name: manager
          env:
          - name: AZURE_AUTH_METHOD
            value: msi
`

	config := &testConfig{
		gitPat:        outputs["shared_pat"].Value.(string),
		gitPrivateKey: outputs["shared_id_rsa"].Value.(string),
		gitPublicKey:  outputs["shared_id_rsa_pub"].Value.(string),
		knownHosts:    azureDevOpsKnownHosts,
		fleetInfraRepository: repoConfig{
			http: fleetInfraRepository["http"].(string),
			ssh:  fleetInfraRepository["ssh"].(string),
		},
		applicationRepository: repoConfig{
			http: applicationRepository["http"].(string),
			ssh:  applicationRepository["ssh"].(string),
		},
		dockerCred: dockerCred{
			url:      acr["url"].(string),
			username: acr["username"].(string),
			password: acr["password"].(string),
		},
		notificationURL: eventHubSas,
		sopsArgs:        fmt.Sprintf("--azure-kv %s", sharedSopsId),
		sopsSecretData: map[string]string{
			"sops.azure-kv": fmt.Sprintf(`clientId: %s`, fluxAzureSp["client_id"].(string)),
		},
		kustomizationYaml: kustomizeYaml,
	}

	return config, nil
}

// registryLoginACR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
func registryLoginACR(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// NOTE: ACR registry accept dynamic repository creation by just pushing a
	// new image with a new repository name.
	testRepos := map[string]string{}

	acr := output["acr"].Value.(map[string]interface{})
	registryURL := acr["url"].(string)
	if err := tftestenv.RegistryLoginACR(ctx, registryURL); err != nil {
		return nil, err
	}
	testRepos["acr"] = registryURL

	return testRepos, nil
}
