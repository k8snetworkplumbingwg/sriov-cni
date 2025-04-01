#
# Credit:
#   This makefile was adapted from: https://github.com/vincentbernat/hellogopher/blob/feature/glide/Makefile
#
# Package related
BINARY_NAME=sriov
PACKAGE=sriov-cni
BINDIR=$(CURDIR)/bin
BUILDDIR=$(CURDIR)/build
PKGS = $(or $(PKG),$(shell go list ./... | grep -v ".*/mocks"))
IMAGE_BUILDER ?= docker

# Test settings
TIMEOUT = 30
COVERAGE_DIR = $(CURDIR)/test/coverage
COVERAGE_MODE = atomic
COVERAGE_PROFILE = $(COVERAGE_DIR)/cover-unit.out

# Docker
IMAGEDIR=$(CURDIR)/images
DOCKERFILE?=$(CURDIR)/Dockerfile
TAG?=ghcr.io/k8snetworkplumbingwg/sriov-cni
# Accept proxy settings for docker 
DOCKERARGS=
ifdef HTTP_PROXY
	DOCKERARGS += --build-arg http_proxy=$(HTTP_PROXY)
endif
ifdef HTTPS_PROXY
	DOCKERARGS += --build-arg https_proxy=$(HTTPS_PROXY)
endif

# Go settings
GO = go
GO_BUILD_OPTS ?=CGO_ENABLED=0
GO_LDFLAGS ?=
GO_FLAGS ?=
GO_TAGS ?=-tags no_openssl
export GOPATH?=$(shell go env GOPATH)

# debug
V ?= 0
Q = $(if $(filter 1,$V),,@)

.PHONY: all
all: fmt lint build

$(BINDIR) $(BUILDDIR) $(COVERAGE_DIR): ; $(info Creating directory $@...)
	@mkdir -p $@

.PHONY: build
build: | $(BUILDDIR) ; $(info Building $(BINARY_NAME)...) @ ## Build SR-IOV CNI plugin
	$Q cd $(CURDIR)/cmd/$(BINARY_NAME) && $(GO_BUILD_OPTS) go build -ldflags '$(GO_LDFLAGS)' $(GO_FLAGS) -o $(BUILDDIR)/$(BINARY_NAME) $(GO_TAGS) -v
	$(info Done!)

# Tools
GOLANGCI_LINT = $(BINDIR)/golangci-lint
GOLANGCI_LINT_VERSION = v1.64.7
$(GOLANGCI_LINT): | $(BINDIR) ; $(info  Installing golangci-lint...)
	$Q GOBIN=$(BINDIR) $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# Tools
MOCKERY = $(BINDIR)/mockery
MOCKERY_VERSION = v2.50.2
$(MOCKERY): | $(BINDIR) ; $(info  Installing mockery...)
	$Q GOBIN=$(BINDIR) $(GO) install github.com/vektra/mockery/v2@$(MOCKERY_VERSION)

# Tests
TEST_TARGETS := test-default test-verbose test-race
.PHONY: $(TEST_TARGETS) test
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race         ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
test: ; $(info  running $(NAME:%=% )tests...) @ ## Run tests
	$Q $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(PKGS)

.PHONY: test-coverage
test-coverage: | $(COVERAGE_DIR) ; $(info  Running coverage tests...) @ ## Run coverage tests
	$Q $(GO) test -timeout $(TIMEOUT)s -cover -covermode=$(COVERAGE_MODE) -coverprofile=$(COVERAGE_PROFILE) $(PKGS)

.PHONY: lint
lint: $(GOLANGCI_LINT) ; $(info  Running golangci-lint linter...) @ ## Run golangci-lint linter
	$Q $(GOLANGCI_LINT) run

.PHONY: mock-generate
mock-generate: $(MOCKERY) ; $(info  Running mockery...) @ ## Run golangci-lint linter
	$Q $(MOCKERY)  --recursive=true --name=NetlinkManager --output=./pkg/utils/mocks/ --filename=netlink_manager_mock.go --exported --dir pkg/utils
	$Q $(MOCKERY)  --recursive=true --name=pciUtils --output=./pkg/sriov/mocks/ --filename=pci_utils_mock.go --exported --dir pkg/sriov


.PHONY: fmt
fmt: ; $(info  Running go fmt...) @ ## Run go fmt on all source files
	@ $(GO) fmt ./...

.PHONY: vet
vet: ; $(info  Running go vet...) @ ## Run go vet on all source files
	@ $(GO) vet ./...

# Docker image
# To pass proxy for Docker invoke it as 'make image HTTP_POXY=http://192.168.0.1:8080'
.PHONY: image
image: ; $(info Building Docker image...) @ ## Build SR-IOV CNI docker image
	@$(IMAGE_BUILDER) build -t $(TAG) -f $(DOCKERFILE)  $(CURDIR) $(DOCKERARGS)

test-image: image
	$Q $(IMAGEDIR)/image_test.sh $(IMAGE_BUILDER) $(TAG)

BASH_UNIT=$(BINDIR)/bash_unit
$(BASH_UNIT): $(BINDIR)
	curl -L https://github.com/pgrange/bash_unit/raw/refs/tags/v2.3.2/bash_unit > bin/bash_unit
	chmod a+x bin/bash_unit

test-integration: $(BASH_UNIT)
	mkdir -p $(COVERAGE_DIR)/integration
	GOCOVERDIR=$(COVERAGE_DIR)/integration $(BASH_UNIT) test/integration/test_*.sh
	go tool covdata textfmt -pkg github.com/k8snetworkplumbingwg/sriov-cni/... -i $(COVERAGE_DIR)/integration -o test/coverage/cover-integration.out

GOCOVMERGE = $(BINDIR)/gocovmerge
gocovmerge: ## Download gocovmerge locally if necessary.
	GOBIN=$(BINDIR) $(GO) install github.com/shabbyrobe/gocovmerge/cmd/gocovmerge@v0.0.0-20230507112040-c3350d9342df

merge-test-coverage: gocovmerge
	$(GOCOVMERGE) $(COVERAGE_DIR)/cover-*.out > $(COVERAGE_DIR)/cover.out

# Misc
.PHONY: deps-update
deps-update: ; $(info  Updating dependencies...) @ ## Update dependencies
	@ $(GO) mod tidy

.PHONY: clean
clean: ; $(info  Cleaning...) @ ## Cleanup everything
	@ $(GO) clean --modcache --cache --testcache
	@ rm -rf $(BUILDDIR)
	@ rm -rf $(BINDIR)
	@ rm -rf test/

.PHONY: help
help: ; @ ## Display this help message
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
