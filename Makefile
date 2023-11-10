# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.0.1
PROJECT_DIR = $(CURDIR)
SCRIPTS_DIR = ${PROJECT_DIR}/scripts
BIN_DIR = ${PROJECT_DIR}/bin
# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# jfrog.com/operator-bundle:$VERSION and jfrog.com/operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= carmitassumerole2.jfrogdev.org/access-aws-poc/aws-assume-role/aws-cred-rotator:118

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
Adding modularity and methods structuring
# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

#localhost:5000/aws-cred-rotator:39
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.26.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=charts/jfrog-registry-operator/crds

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
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./...

## build
.PHONY: build
build:
	@$(call echoDebug,"Building...")
	@GOCMD="${GOCMD}" BIN_DIR="${BIN_DIR}" PROJECT_DIR="${PROJECT_DIR}" "${SCRIPTS_DIR}/binary.sh" "${BINARYNAME}" "${CMD_SRC_DIR}"

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/amd64,linux/arm64
# PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: test operator ## Build and push docker image for the manager for cross-platform support
	- docker buildx create --name mybuilder --driver-opt 'image=moby/buildkit:v0.12.1-rootless' #todo turn --driver-opt into optional/remove, this fixes a colima issue
	docker buildx use mybuilder
	docker buildx create --name mybuilder --use || true
	docker buildx build --no-cache --platform linux/arm64,linux/amd64 --build-arg UBI_MICRO="9.2.13" --build-arg UBI_MINIMAL="9.2.717" -t ${IMAGE_TAG_BASE} -f "Dockerfile" --metadata-file=build-metadata-operator  --push .
	- rm -rf build-metadata-operator
	- docker buildx rm mybuilder

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.11.1

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

## operator
##@ Build multiarch operator binaries
.PHONY: operator
operator: build-operator-linux-amd64 build-operator-linux-arm64 
## clean-build
##@ clean the bin directory
.PHONY: clean-build
clean-build:
	@$(call echoDebug,"Removing the bin directory: ${BIN_DIR}")
	@rm -rf "${BIN_DIR}"

## build-docker-amd64-local --locally
.PHONY: build-docker-amd64-local
build-docker-amd64-local: build-operator-linux-amd64
	@build/build-docker-local.sh "operator" "entplus.jfrog.io/inst-jfrog-registry-operator/inst-jfrog-registry-operator:0.x-SNAPSHOT" "${PROJECT_DIR}" "linux/amd64"

## build-docker-arm64-local --locally
.PHONY: build-docker-arm64-local
build-docker-arm64-local: build-operator-linux-arm64
	@build/build-docker-local.sh "operator" "entplus.jfrog.io/inst-jfrog-registry-operator/inst-jfrog-registry-operator:0.x-SNAPSHOT" "${PROJECT_DIR}" "linux/arm64"

## build-operator-docker
##@ Build Operator multiarch docker image.
.PHONY: build-operator-docker
build-operator-docker:
	@.jfrog-pipelines/build/build-docker.sh "operator" "${OPERATOR_IMAGE_NAME}" "${PROJECT_DIR}"

## build-operator-linux-amd64
.PHONY: build-operator-linux-amd64
build-operator-linux-amd64:
	@$(call echoDebug,"")
	@GOOS="linux" GOARCH="amd64" $(MAKE) build BINARYNAME="operator" CMD_SRC_DIR="${PROJECT_DIR}"

## build-operator-linux-arm64
.PHONY: build-operator-linux-arm64
build-operator-linux-arm64:
	@$(call echoDebug,"")
	@GOOS="linux" GOARCH="arm64" $(MAKE) build BINARYNAME="operator" CMD_SRC_DIR="${PROJECT_DIR}"
