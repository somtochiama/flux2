module github.com/fluxcd/flux2/tests/azure

go 1.18

require (
	github.com/Azure/azure-event-hubs-go/v3 v3.3.18
	github.com/fluxcd/go-git/v5 v5.0.0-20221201130551-583c913c13b7
	github.com/fluxcd/helm-controller/api v0.24.0
	github.com/fluxcd/image-automation-controller/api v0.25.0
	github.com/fluxcd/image-reflector-controller/api v0.21.0
	github.com/fluxcd/kustomize-controller/api v0.28.0
	github.com/fluxcd/notification-controller/api v0.26.0
	github.com/fluxcd/pkg/apis/meta v0.15.0
	github.com/fluxcd/pkg/git v0.7.0
	github.com/fluxcd/pkg/git/gogit v0.2.0
	github.com/fluxcd/pkg/runtime v0.18.0
	github.com/fluxcd/source-controller/api v0.29.0
	github.com/microsoft/azure-devops-go-api/azuredevops v1.0.0-b5
	github.com/onsi/gomega v1.24.1
	github.com/stretchr/testify v1.8.0
	github.com/whilp/git-urls v1.0.0
	go.uber.org/multierr v1.8.0
	k8s.io/api v0.25.0
	k8s.io/apimachinery v0.25.0
	k8s.io/client-go v0.25.0
	sigs.k8s.io/controller-runtime v0.12.3
)

require (
	github.com/Masterminds/semver/v3 v3.1.1 // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20221026131551-cf6655e29de4 // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/cloudflare/circl v1.3.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fluxcd/pkg/ssh v0.7.0 // indirect
	github.com/fluxcd/pkg/version v0.2.0 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.3.1 // indirect
	github.com/hashicorp/hc-install v0.3.2 // indirect
	github.com/hashicorp/terraform-exec v0.16.1 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.15.8 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/pjbgf/sha1cd v0.2.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/skeema/knownhosts v1.1.0 // indirect
	github.com/xanzy/ssh-agent v0.3.2 // indirect
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

// Fix CVE-2022-28948
replace gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.1

require (
	github.com/Azure/azure-amqp-common-go/v3 v3.2.3 // indirect
	github.com/Azure/azure-sdk-for-go v51.1.0+incompatible // indirect
	github.com/Azure/go-amqp v0.17.0 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.27 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.20 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.12.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/devigned/tab v0.1.1 // indirect
	github.com/docker/cli v20.10.17+incompatible // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.17+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/emicklei/go-restful/v3 v3.8.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/fluxcd/pkg/apis/acl v0.1.0 // indirect
	github.com/fluxcd/pkg/apis/kustomize v0.5.0
	github.com/fluxcd/test-infra/tftestenv v0.0.0-20220726140458-65e1a901cbb9
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/zapr v1.2.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.14 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.2.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/go-containerregistry v0.11.0
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.1 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/hashicorp/terraform-json v0.14.0
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.13.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect; indirectct
	github.com/vbatts/tar-split v0.11.2 // indirect
	github.com/zclconf/go-cty v1.11.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/zap v1.23.0 // indirect
	golang.org/x/crypto v0.2.0 // indirect
	golang.org/x/net v0.2.0 // indirect
	golang.org/x/oauth2 v0.0.0-20220718184931-c8730f7fcb92 // indirect
	golang.org/x/sys v0.2.0 // indirect
	golang.org/x/term v0.2.0 // indirect
	golang.org/x/text v0.4.0 // indirect
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.25.0 // indirect
	k8s.io/component-base v0.25.0 // indirect
	k8s.io/klog/v2 v2.70.1 // indirect
	k8s.io/kube-openapi v0.0.0-20220803162953-67bda5d908f1 // indirect
	k8s.io/utils v0.0.0-20220728103510-ee6ede2d64ed // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

// Fix for CVE-2020-29652: https://github.com/golang/crypto/commit/8b5274cf687fd9316b4108863654cc57385531e8
// Fix for CVE-2021-43565: https://github.com/golang/crypto/commit/5770296d904e90f15f38f77dfc2e43fdf5efc083
replace golang.org/x/crypto => golang.org/x/crypto v0.0.0-20211215153901-e495a2d5b3d3

replace github.com/fluxcd/test-infra/tftestenv => github.com/somtochiama/test-infra/tftestenv v0.0.0-20220902234215-0a583c0d3b76

// This lets us use `go-billy/util.Walk()`, as this function hasn't been released
// in a tagged version yet:
// https://github.com/go-git/go-billy/blob/e0768be422ff616fc042d1d62bfa65962f716ad8/util/walk.go#L59
replace github.com/go-git/go-billy/v5 => github.com/go-git/go-billy/v5 v5.3.2-0.20210603175951-e0768be422ff
