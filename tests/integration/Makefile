GO_TEST_ARGS ?=
PROVIDER_ARG ?=
TEST_TIMEOUT ?= 60m

test:
	# These two versions of podinfo are pushed to the cloud registry and used in tests for ImageUpdateAutomation
	mkdir -p build
	cp ../../bin/flux build/flux
	docker pull ghcr.io/stefanprodan/podinfo:6.0.0
	docker pull ghcr.io/stefanprodan/podinfo:6.0.1
	go test -timeout $(TEST_TIMEOUT) -v ./ $(GO_TEST_ARGS) $(PROVIDER_ARG) --tags=integration

test-azure:
	$(MAKE) test PROVIDER_ARG="-provider azure"
