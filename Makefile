#
# Credit:
#   This makefile was adapted from: https://github.com/vincentbernat/hellogopher/blob/feature/glide/Makefile
#
# Package related
BINARY_NAME=sriov
PACKAGE=sriov-cni
ORG_PATH=github.com/k8snetworkplumbingwg
REPO_PATH=$(ORG_PATH)/$(PACKAGE)
GOPATH=$(CURDIR)/.gopath
GOBIN=$(CURDIR)/bin
BUILDDIR=$(CURDIR)/build
BASE=$(GOPATH)/src/$(REPO_PATH)
GOFILES = $(shell find . -name *.go | grep -vE "(\/vendor\/)|(_test.go)")
PKGS     = $(or $(PKG),$(shell cd $(BASE) && env GOPATH=$(GOPATH) $(GO) list ./... | grep -v "^$(PACKAGE)/vendor/"))
TESTPKGS = $(shell env GOPATH=$(GOPATH) $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))

export GOPATH
export GOBIN
export GO111MODULE=on
# Docker
IMAGEDIR=$(BASE)/images
DOCKERFILE=$(CURDIR)/Dockerfile
TAG=ghcr.io/k8snetworkplumbingwg/sriov-cni
# Accept proxy settings for docker 
DOCKERARGS=
ifdef HTTP_PROXY
	DOCKERARGS += --build-arg http_proxy=$(HTTP_PROXY)
endif
ifdef HTTPS_PROXY
	DOCKERARGS += --build-arg https_proxy=$(HTTPS_PROXY)
endif

# Go tools
GO      = go
GOFMT   = gofmt
TIMEOUT = 15
V ?= 0
Q = $(if $(filter 1,$V),,@)

.PHONY: all
all: fmt lint build

$(BASE): ; $(info  Setting GOPATH...)
	@mkdir -p $(dir $@)
	@ln -sf $(CURDIR) $@

$(GOBIN):
	@mkdir -p $@

$(BUILDDIR): | $(BASE) ; $(info Creating build directory...)
	@cd $(BASE) && mkdir -p $@

build: $(BUILDDIR)/$(BINARY_NAME) ; $(info Building $(BINARY_NAME)...) @ ## Build SR-IOV CNI plugin
	$(info Done!)

$(BUILDDIR)/$(BINARY_NAME): $(GOFILES) | $(BUILDDIR)
	@cd $(BASE)/cmd/$(BINARY_NAME) && CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -v


# Tools

GOLANGCILINT = $(GOBIN)/golangci-lint
$(GOLANGCILINT): | $(BASE) ; $(info  Installing golangci-lint...)
	$Q go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.52.2

GOCOVMERGE = $(GOBIN)/gocovmerge
$(GOCOVMERGE): | $(BASE) ; $(info  Building gocovmerge...)
	$Q go install github.com/wadey/gocovmerge@latest

GOCOV = $(GOBIN)/gocov
$(GOCOV): | $(BASE) ; $(info  Building gocov...)
	$Q go install github.com/axw/gocov/gocov@v1.1.0

GOCOVXML = $(GOBIN)/gocov-xml
$(GOCOVXML): | $(BASE) ; $(info  Building gocov-xml...)
	$Q go install github.com/AlekSi/gocov-xml@latest

GCOV2LCOV = $(GOBIN)/gcov2lcov
$(GCOV2LCOV): | $(BASE) ; $(info  building gcov2lcov...)
	$Q go install github.com/jandelgado/gcov2lcov@latest

GO2XUNIT = $(GOBIN)/go2xunit
$(GO2XUNIT): | $(BASE) ; $(info  Building go2xunit...)
	$Q go install github.com/tebeka/go2xunit@latest


# Tests

TEST_TARGETS := test-default test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) test-xml check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race         ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
check test tests: fmt lint | $(BASE) ; $(info  Running $(NAME:%=% )tests...) @ ## Run tests
	$Q cd $(BASE) && $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

test-xml: fmt lint | $(BASE) $(GO2XUNIT) ; $(info  Running $(NAME:%=% )tests...) @ ## Run tests with xUnit output
	$Q cd $(BASE) && 2>&1 $(GO) test -timeout 20s -v $(TESTPKGS) | tee test/tests.output
	$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

COVERAGE_DIR = $(CURDIR)/test/coverage
COVERAGE_MODE = atomic
COVERAGE_PROFILE = $(COVERAGE_DIR)/profile.out
COVERAGE_XML = $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML = $(COVERAGE_DIR)/index.html
.PHONY: test-coverage test-coverage-tools
test-coverage-tools: | $(GOCOVMERGE) $(GOCOV) $(GOCOVXML) $(GCOV2LCOV)
test-coverage: fmt test-coverage-tools | $(BASE) ; $(info  Running coverage tests...) @ ## Run coverage tests
	$Q mkdir -p $(COVERAGE_DIR)/pkgs
	$Q cd $(BASE) && for pkg in $(TESTPKGS); do \
		$(GO) test \
			-coverpkg=$(REPO_PATH)/... \
			-covermode=$(COVERAGE_MODE) \
			-coverprofile="$(COVERAGE_DIR)/pkgs/`echo $$pkg | tr "/" "-"`.cover" $$pkg ;\
	done
	$Q $(GOCOVMERGE) $(COVERAGE_DIR)/pkgs/*.cover > $(COVERAGE_PROFILE)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)
	$Q $(GCOV2LCOV) -infile $(COVERAGE_PROFILE) -outfile $(COVERAGE_DIR)/lcov.info

.PHONY: lint
lint: | $(BASE) $(GOLANGCILINT) ; $(info  Running golangci-lint...) @ ## Run golint on all source files
	$Q $(GOLANGCILINT) run ./...

.PHONY: fmt
fmt: ; $(info  Running gofmt...) @ ## Run gofmt on all source files
	@ret=0 && for d in $$($(GO) list -f '{{.Dir}}' ./... | grep -v /vendor/); do \
		$(GOFMT) -l -w $$d/*.go || ret=$$? ; \
	 done ; exit $$ret

# Docker image
# To pass proxy for Docker invoke it as 'make image HTTP_POXY=http://192.168.0.1:8080'
.PHONY: image
image: | $(BASE) ; $(info Building Docker image...) @ ## Build SR-IOV CNI docker image
	@docker build -t $(TAG) -f $(DOCKERFILE)  $(CURDIR) $(DOCKERARGS)

# Misc

.PHONY: deps-update
deps-update: ; $(info  Updating dependencies...) @ ## Update dependencies
	@go mod tidy && go mod vendor

.PHONY: clean
clean: | $(BASE) ; $(info  Cleaning...) @ ## Cleanup everything
	@cd $(BASE) && $(GO) clean --modcache --cache --testcache
	@rm -rf $(GOPATH)
	@rm -rf $(BUILDDIR)
	@rm -rf $(GOBIN)
	@rm -rf test/

.PHONY: help
help: ; @ ## Display this help message
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
