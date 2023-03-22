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
	"flag"
	"fmt"
	"log"
	"os"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	helmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	automationv1beta1 "github.com/fluxcd/image-automation-controller/api/v1beta1"
	reflectorv1beta1 "github.com/fluxcd/image-reflector-controller/api/v1beta1"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	notiv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/pkg/git"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/fluxcd/test-infra/tftestenv"
)

const (
	// azureTerraformPath is the path to the folder containing the
	// terraform files for azure infra
	azureTerraformPath = "./terraform/azure"

	// kubeconfigPath is the path of the file containing the kubeconfig
	kubeconfigPath = "./build/kubeconfig"
)

var (
	// cfg is a struct containing different variables needed for the test.
	cfg *testConfig

	// infraOpts are the options for running the terraform environment
	infraOpts tftestenv.Options

	// testRepos is a map of registry common name and URL of the test
	// repositories. This is used as the test cases to run the tests against.
	// The registry common name need not be the actual registry address but an
	// identifier to identify the test case without logging any sensitive
	// account IDs in the subtest names.
	// For example, map[string]string{"ecr", "xxxxx.dkr.ecr.xxxx.amazonaws.com/foo:v1"}
	// would result in subtest name TestImageRepositoryScanAWS/ecr.
	testRepos map[string]string

	// versions to tag and push for the podinfo image
	oldVersion  = "6.0.0"
	newVersion  = "6.0.1"
	podinfoTags = []string{oldVersion, newVersion}

	// testEnv is the test environment. It contains test infrastructure and
	// kubernetes client of the created cluster.
	testEnv *tftestenv.Environment
)

// testConfig hold different variable that will be needed by the different test functions.
type testConfig struct {
	// authentication info for git repositories
	gitPat              string
	gitUsername         string
	gitPrivateKey       string
	gitPublicKey        string
	defaultGitTransport git.TransportType
	defaultAuthOpts     *git.AuthOptions
	// Generate known host? Use flux cli?
	knownHosts            string
	fleetInfraRepository  repoConfig
	applicationRepository repoConfig

	dockerCred      dockerCred
	notificationURL string

	// cloud provider dependent argument to pass to the sops cli
	sopsArgs string
	// secret data for sops
	sopsSecretData map[string]string
	// envCredsData are data field for a secres containing environment variables that the Flux deployments
	// will need
	envCredsData map[string]string
	// kustomizationYaml is the  content of the kustomization.yaml for customizing the Flux manifests
	kustomizationYaml string
}

// repoConfig contains the http/ssh urls for the created git repositories
// on the various cloud providers.
type repoConfig struct {
	http string
	ssh  string
}

// dockerCred contains credentials for the container repository
type dockerCred struct {
	url      string
	username string
	password string
}

// getTestConfig gets the test configuration that contains different variables for running the tests
type getTestConfig func(ctx context.Context, output map[string]*tfjson.StateOutput) (*testConfig, error)

// registryLoginFunc is used to perform registry login against a provider based
// on the terraform state output values. It returns a map of registry common
// name and test repositories to test against, read from the terraform state
// output.
type registryLoginFunc func(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error)

type pushTestImages func(ctx context.Context, localImgs map[string]string, output map[string]*tfjson.StateOutput) (map[string]string, error)

// ProviderConfig contains the test configurations for the different cloud providers
type ProviderConfig struct {
	terraformPath    string
	createKubeconfig tftestenv.CreateKubeconfig
	getTestConfig    getTestConfig
	// registryLogin is used to perform registry login.
	registryLogin  registryLoginFunc
	pushTestImages pushTestImages
}

func init() {
	utilruntime.Must(sourcev1.AddToScheme(scheme.Scheme))
	utilruntime.Must(kustomizev1.AddToScheme(scheme.Scheme))
	utilruntime.Must(helmv2beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(reflectorv1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(automationv1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(notiv1beta2.AddToScheme(scheme.Scheme))
}

func TestMain(m *testing.M) {
	infraOpts.Bindflags(flag.CommandLine)
	flag.Parse()

	err := infraOpts.Validate()
	if err != nil {
		log.Fatal(err)
	}

	// TODO(somtochiama): remove when tests have been updated to support GCP and AWS
	if infraOpts.Provider != "azure" {
		log.Fatal("only azure e2e tests are currently supported.")
	}

	exitVal, err := setup(m)
	if err != nil {
		log.Printf("Received an error while running setup: %v", err)
		os.Exit(1)
	}
	os.Exit(exitVal)
}

func setup(m *testing.M) (exitVal int, err error) {
	ctx := context.TODO()

	// get provider specific configuration
	providerCfg, err := getProviderConfig(infraOpts.Provider)

	localImgs := map[string]string{
		"podinfo:6.0.0": "ghcr.io/stefanprodan/podinfo:6.0.0",
		"podinfo:6.0.1": "ghcr.io/stefanprodan/podinfo:6.0.1",
	}

	// Setup Terraform binary and init state
	log.Printf("Setting up %s e2e test infrastructure", infraOpts.Provider)
	envOpts := []tftestenv.EnvironmentOption{
		tftestenv.WithExisting(infraOpts.Existing),
		tftestenv.WithRetain(infraOpts.Retain),
		tftestenv.WithVerbose(infraOpts.Verbose),
		tftestenv.WithCreateKubeconfig(providerCfg.createKubeconfig),
	}

	// Create terraform infrastructure
	testEnv, err = tftestenv.New(context.Background(), scheme.Scheme, providerCfg.terraformPath, kubeconfigPath, envOpts...)
	if err != nil {
		return 0, err
	}

	// get terrraform infrastructure
	outputs, err := testEnv.StateOutput(context.Background())
	if err != nil {
		return 0, err
	}

	// get provider specific test configuration
	cfg, err = providerCfg.getTestConfig(context.Background(), outputs)
	if err != nil {
		return 0, err
	}

	_, err = providerCfg.registryLogin(ctx, outputs)
	if err != nil {
		return 0, err
	}

	testRepos, err = providerCfg.pushTestImages(ctx, localImgs, outputs)
	if err != nil {
		return 0, err
	}

	err = installFlux(ctx, testEnv.Client, installArgs{
		kubeconfigPath: kubeconfigPath,
		secretData:     cfg.envCredsData,
	})
	if err != nil {
		return 1, fmt.Errorf("error installing Flux: %v", err)
	}

	// Run tests
	log.Println("Running e2e tests")
	result := m.Run()

	if err := uninstallFlux(ctx); err != nil {
		log.Printf("Failed to uninstall: %v", err)
	}
	
	if err := testEnv.Stop(ctx); err != nil {
		log.Printf("Failed to stop environment: %v", err)
	}

	return result, nil
}

func getProviderConfig(provider string) (*ProviderConfig, error) {
	switch provider {
	case "azure":
		return &ProviderConfig{
			terraformPath:    azureTerraformPath,
			createKubeconfig: createKubeConfigAKS,
			getTestConfig:    getTestConfigAKS,
			registryLogin:    registryLoginACR,
			pushTestImages:   pushTestImagesACR,
		}, nil
	default:
		return nil, fmt.Errorf("provider '%s' is not supported", provider)
	}
}
