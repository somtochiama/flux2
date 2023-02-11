GO_TEST_ARGS ?=
PROVIDER_ARG ?=
TEST_TIMEOUT ?= 60m

test:
	 docker pull ghcr.io/stefanprodan/podinfo:6.0.0
	 docker pull ghcr.io/stefanprodan/podinfo:6.0.1
	go test -timeout $(TEST_TIMEOUT) -v ./ $(GO_TEST_ARGS) $(PROVIDER_ARG) --tags=integration

test-azure:
	$(MAKE) test PROVIDER_ARG="-provider azure"
