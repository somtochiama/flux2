## E2E Tests

The goal is to verify that Flux integration with cloud providers are actually working now and in the future.
Currently, we only have tests for Azure.

## Azure
## Architecture

The [azure](./terraform/azure) Terraform creates the AKS cluster and related resources to run the tests. It creates:
- An Azure Container Registry
- An Azure Kubernetes Cluster
- Two Azure DevOps repositories
- Azure EventHub for sending notifications
- An Azure Key Vault

## Requirements

- Azure account with an active subscription to be able to create AKS and ACR, and permission to assign roles. Role assignment is required for allowing AKS workloads to access ACR.
- Azure CLI, need to be logged in using `az login`.
- Docker CLI for registry login.
- kubectl for applying certain install manifests.
- An Azure DevOps organization, personal access token and ssh keys for accessing repositories within the organization. The scope required for the personal access token is:
  - `Project and Team` - read, write and manage access
  - `Code` - Full
  - Please take a look at the [terraform provider](https://registry.terraform.io/providers/microsoft/azuredevops/latest/docs/guides/authenticating_using_the_personal_access_token#create-a-personal-access-token)
    for more explanation.

## Tests

Each test run is initiated by running `terraform apply` in the azure Terraform directory, it does this by using the [tftestenv package](https://github.com/fluxcd/test-infra/blob/main/tftestenv/testenv.go) within the `fluxcd/test-infra` repository.
It then reads the output of the Terraform to get information needed for the tests like the kubernetes client ID, the azure DevOps repository urls, the key vault ID etc. This means that a lot of the communication with the Azure API is offset to
Terraform instead of requiring it to be implemented in the test.

The following tests are currently implemented:

- [x] Flux can be successfully installed on AKS using the Flux CLI
- [x] source-controller can clone Azure DevOps repositories (https+ssh)
- [x] image-reflector-controller can list tags from Azure Container Registry image repositories
- [x] kustomize-controller can decrypt secrets using SOPS and Azure Key Vault
- [x] image-automation-controller can create branches and push to Azure DevOps repositories (https+ssh)
- [x] notification-controller can send commit status to Azure DevOps
- [x] notification-controller can forward events to Azure Event Hub
- [x] source-controller can pull charts from Azure Container Registry Helm repositories

## Running these tests locally

1. Copy `.env.sample` to `.env` and add the values for the different variables which includes - your Azure DevOps org, 
personal access tokens and ssh keys for accessing repositories on Azure DevOps org. Run  `source .env` to set them.
2. Ensure that you have the Flux CLI binary that is to be tested built and ready. You can build it by running
`make build` at the root of this repository. The binary is located at `./bin` directory at the root and by default
this is where the Makefile copies the binary for the tests from. If you have it in a different location, you can set it
with the `FLUX_BINARY` variable
3. Run `make test-azure`, setting the location of the flux binary with `FLUX_BINARY` variable

```console
$ GO_TEST_ARGS="-existing" make test-azure
/Library/Developer/CommandLineTools/usr/bin/make test PROVIDER_ARG="-provider azure"
# These two versions of podinfo are pushed to the cloud registry and used in tests for ImageUpdateAutomation
mkdir -p build
cp ../../bin/flux build/flux
docker pull ghcr.io/stefanprodan/podinfo:6.0.0
6.0.0: Pulling from stefanprodan/podinfo
Digest: sha256:e7eeab287181791d36c82c904206a845e30557c3a4a66a8143fa1a15655dae97
Status: Image is up to date for ghcr.io/stefanprodan/podinfo:6.0.0
ghcr.io/stefanprodan/podinfo:6.0.0
docker pull ghcr.io/stefanprodan/podinfo:6.0.1
6.0.1: Pulling from stefanprodan/podinfo
Digest: sha256:1169f220a670cf640e45e1a7ac42dc381a441e9d4b7396432cadb75beb5b5d68
Status: Image is up to date for ghcr.io/stefanprodan/podinfo:6.0.1
ghcr.io/stefanprodan/podinfo:6.0.1
go test -timeout 60m -v ./ -existing -provider azure --tags=integration
2023/03/24 02:32:25 Setting up azure e2e test infrastructure
2023/03/24 02:32:25 Terraform binary:  /usr/local/bin/terraform
2023/03/24 02:32:25 Init Terraform
2023/03/24 02:32:37 Applying Terraform
2023/03/24 02:36:49 pushing flux test image acrappsherring.azurecr.io/podinfo:6.0.0
2023/03/24 02:36:58 pushing flux test image acrappsherring.azurecr.io/podinfo:6.0.1
2023/03/24 02:38:06 Running e2e tests
=== RUN   TestNotification
--- PASS: TestNotification (17.69s)
=== RUN   TestAzureDevOpsCommitStatus
--- PASS: TestAzureDevOpsCommitStatus (5.80s)
=== RUN   TestFluxInstallation
--- PASS: TestFluxInstallation (0.00s)
=== RUN   TestRepositoryCloning
    flux_test.go:92: Creating application sources
We noticed you're using an older version of Git. For the best experience, upgrade to a newer version.
Analyzing objects... (1/1) (6 ms)
Storing packfile... done (177 ms)
Storing index... done (52 ms)
We noticed you're using an older version of Git. For the best experience, upgrade to a newer version.
    flux_test.go:114: Verifying application-gitops namespaces
=== RUN   TestRepositoryCloning/ssh-feature-branch
=== RUN   TestRepositoryCloning/ssh-v1
=== RUN   TestRepositoryCloning/https-feature-branch
=== RUN   TestRepositoryCloning/https-v1
--- PASS: TestRepositoryCloning (36.06s)
    --- PASS: TestRepositoryCloning/ssh-feature-branch (8.39s)
    --- PASS: TestRepositoryCloning/ssh-v1 (8.57s)
    --- PASS: TestRepositoryCloning/https-feature-branch (8.40s)
    --- PASS: TestRepositoryCloning/https-v1 (8.56s)
=== RUN   TestImageRepositoryAndAutomation
--- PASS: TestImageRepositoryAndAutomation (18.20s)
=== RUN   TestACRHelmRelease
2023/03/24 02:39:27 Pushing 'ghcr.io/stefanprodan/charts/podinfo:6.2.0' to 'acrappsherring.azurecr.io/charts/podinfo:v0.0.1'
2023/03/24 02:39:33 helm repository condition not ready
--- PASS: TestACRHelmRelease (15.31s)
=== RUN   TestKeyVaultSops
--- PASS: TestKeyVaultSops (15.98s)
PASS
2023/03/24 02:40:12 Destroying environment...
ok      github.com/fluxcd/flux2/tests/integration       947.341s
```


In the above, the test created a build directory build/ and the flux cli binary is copied build/flux. It would be used
to bootstrap Flux on the cluster. You can configure the location of the Flux CLI binary by setting the FLUX_BINARY variable. 
We also pull two version of `ghcr.io/stefanprodan/podinfo` image. These images are pushed to the Azure Container Registry
and used to test `ImageRepository` and `ImageUpdateAutomation`. The terraform resources get created and the tests are run.


**IMPORTANT:** In case the terraform infrastructure results in a bad state, maybe due to a crash during the apply, 
the whole infrastructure can be destroyed by running terraform destroy in terraform/<provider> directory.


## Debugging the tests

For debugging environment provisioning, enable verbose output with `-verbose` test flag.

```sh
make test-azure GO_TEST_ARGS="-verbose"
```

The test environment is destroyed at the end by default. Run the tests with -retain flag to retain the created test infrastructure.

```sh
make test-azure GO_TEST_ARGS="-retain"
```
The tests require the infrastructure state to be clean. For re-running the tests with a retained infrastructure, set -existing flag.

```sh
make test-azure GO_TEST_ARGS="-retain -existing"
```

To delete an existing infrastructure created with -retain flag:

```sh
make test-azure GO_TEST_ARGS="-existing"
```
