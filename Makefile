OS=$(shell go env GOOS)
ARCH=$(shell go env GOARCH)

GIT_VERSION := $(shell date +v%Y%m%d)-$(shell git rev-parse --short HEAD)
ifneq ($(shell git status --porcelain),)
	GIT_VERSION := $(GIT_VERSION)-dirty
endif
TAG ?= $(GIT_VERSION)
# Image URL to use all building/pushing image targets
IMG ?= localhost.dev/controller:$(TAG)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-23s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=kubedredger-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

#KIND_CLUSTER ?= k8s-example-kubedredger-test-e2e
KIND_CLUSTER ?= k8s-example-kubedredger-kind

.PHONY: setup-test-e2e
setup-test-e2e: ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER) go test -tags=e2e ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	$(GOLANGCI_LINT) config verify

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build kubedredger binary.
	go build -o bin/kubedredger cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the kubedredger image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the kubedredger.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the kubedredger.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the kubedredger image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the kubedredger for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name k8s-example-kubedredger-builder
	$(CONTAINER_TOOL) buildx use k8s-example-kubedredger-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm k8s-example-kubedredger-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= $(LOCALBIN)/kubectl
KIND ?= $(LOCALBIN)/kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUBECTL_VERSION ?= 1.34.0
KIND_VERSION ?= 0.30.0
KUSTOMIZE_VERSION ?= v5.6.0
CONTROLLER_TOOLS_VERSION ?= v0.18.0
#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')
GOLANGCI_LINT_VERSION ?= 2.3.0

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): dep-install-golangci-lint
#$(GOLANGCI_LINT): $(LOCALBIN)
#	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,v$(GOLANGCI_LINT_VERSION))

##@ Workshop-specific helpers

# extra deps
.PHONY: dep-install-kubectl
dep-install-kubectl: $(KUBECTL) ## Download kubectl locally if necessary, or reuse the system binary.
$(KUBECTL): $(LOCALBIN)
	@command -v kubectl >/dev/null 2>&1 && { \
		ln -sf $(shell command -v kubectl ) $(LOCALBIN) ;\
		echo "reusing system kubectl" ;\
	} || { \
		curl -sSL "https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$(OS)/$(ARCH)/kubectl" -o $(KUBECTL)-v$(KUBECTL_VERSION) ;\
		chmod 0755 $(KUBECTL)-v$(KUBECTL_VERSION) ;\
		ln -sf $(KUBECTL)-v$(KUBECTL_VERSION) $(KUBECTL) ;\
		echo "downloaded $(shell basename $(KUBECTL)) $(KUBECTL_VERSION) in $(LOCALBIN)" ;\
	}

.PHONY: dep-install-golangci-lint
dep-install-golangci-lint: $(LOCALBIN)  ## Download golangci-lint locally if necessary, or reuse the system binary
	@[ ! -f $(LOCALBIN)/golangci-lint ] && { \
		command -v golangci-lint >/dev/null 2>&1 && {\
			ln -sf $(shell command -v golangci-lint ) $(LOCALBIN) ;\
			echo "reusing system golangci-lint" ;\
		} || { \
			curl -sSL "https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH).tar.gz" -o $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH).tar.gz ;\
			tar -x -C $(LOCALBIN) -f $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH).tar.gz ;\
			ln -sf $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION)-$(OS)-$(ARCH)/golangci-lint $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION) ;\
			ln -sf $(GOLANGCI_LINT)-$(GOLANGCI_LINT_VERSION) $(GOLANGCI_LINT) ;\
		}; \
	} || true

.PHONY: dep-install-kind
dep-install-kind: $(KIND) ## Download kind locally if necessary, or reuse the system binary
$(KIND): $(LOCALBIN)
	@command -v kind >/dev/null 2>&1 && {\
		ln -sf $(shell command -v kind ) $(LOCALBIN) ;\
		echo "reusing system kubectl" ;\
	} || { \
		curl -sSL "https://kind.sigs.k8s.io/dl/v$(KIND_VERSION)/kind-$(OS)-$(ARCH)" -o $(KIND)-v$(KIND_VERSION) ;\
		chmod 0755 $(KIND)-v$(KIND_VERSION) ;\
		ln -sf $(KIND)-v$(KIND_VERSION) $(KIND) ;\
		echo "downloaded $(shell basename $(KIND)) $(KIND_VERSION) in $(LOCALBIN)" ;\
	}

.PHONY: system-lint
system-lint: golangci-lint ## Run golangci-lint linter using system binaries
	golangci-lint run

.PHONY: system-lint-fix
system-lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes using system binaries
	golangci-lint run --fix

.PHONY: lint-config
system-lint-config: golangci-lint ## Verify golangci-lint linter configuration using system binaries
	golangci-lint config verify

.PHONY: kind-setup
kind-setup: ## Set up a Kind cluster if it does not exist
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: kind-teardown
kind-teardown: ## Tear down the Kind cluster created with kind-setup
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: kind-load-image
kind-load-image: ## Load the image in the local cluster
	@$(KIND) load docker-image ${IMG} --name $(KIND_CLUSTER)

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] && [ "$$(readlink -- "$(1)" 2>/dev/null)" = "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
echo "downloaded $$(basename $(1)) $(3) in $(LOCALBIN)" ;\
} ;\
ln -sf $$(realpath $(1)-$(3)) $(1)
endef
