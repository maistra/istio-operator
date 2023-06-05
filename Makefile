# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 3.0.0
MINOR_VERSION := $(shell v='$(VERSION)'; echo "$${v%.*}")

# Istio repository to pull charts from
ISTIO_REPOSITORY ?= maistra/istio
# the branch to use when updating the commit hash below
ISTIO_BRANCH_30 ?= maistra-3.0
# the current commit of ${ISTIO_REPOSITORY} for v3.0. This will be used to copy charts
ISTIO_COMMIT_30 ?= 16ff353b180a1f461bd380be6ef2d1809b8c1fba

# Istio images names
ISTIO_CNI_IMAGE_NAME ?= install-cni
ISTIO_PILOT_IMAGE_NAME ?= pilot
ISTIO_PROXY_IMAGE_NAME ?= proxyv2

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
# maistra.io/maistraoperator-bundle:$VERSION and maistra.io/maistraoperator-catalog:$VERSION.
IMAGE_TAG_BASE ?= quay.io/maistra-dev/istio-operator-rhel9

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Image registry url to use
REGISTRY_URL ?= quay.io
# Image registry org to use
REGISTRY_ORG ?= maistra-dev
# Image hub to use
HUB ?= ${REGISTRY_URL}/${REGISTRY_ORG}
# Image tag to use
TAG ?= ${MINOR_VERSION}
# Image URL to use all building/pushing image targets
IMAGE ?= ${HUB}/istio-operator:${TAG}
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

##@ Testing

.PHONY: test
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: gen ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: gen ## Run a controller from your host.
	go run ./main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${IMAGE} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMAGE}

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMAGE=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMAGE=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: test ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- docker buildx create --name project-v3-builder
	docker buildx use project-v3-builder
	- docker buildx build --push --platform=$(PLATFORMS) --tag ${IMAGE} -f Dockerfile.cross .
	- docker buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: gen kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: gen kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: gen kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMAGE}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Generated Code & Resources

.PHONY: gen-manifests
gen-manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: gen-code
gen-code: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: gen-charts
gen-charts: ## Pull charts from maistra/istio repository
	hack/download-charts.sh v${MINOR_VERSION} https://github.com/${ISTIO_REPOSITORY} ${ISTIO_COMMIT_30}

.PHONY: gen ## Generate everything
gen: controller-gen gen-manifests gen-code gen-charts

.PHONY: gen-check
gen-check: gen check-clean-repo ## Verifies that changes in generated resources have been checked in

.PHONY: check-clean-repo
check-clean-repo:
	@if [[ -n $$(git status --porcelain) ]]; then git status; git diff; echo "ERROR: Some files need to be updated, please run 'make gen' and include any changed files in your PR"; exit 1;	fi

.PHONY: update-istio
update-istio: ## Updates the Istio commit hash to latest on ${ISTIO_BRANCH_30}
	@LATEST_COMMIT_30=`git ls-remote https://github.com/${ISTIO_REPOSITORY}.git | grep ${ISTIO_BRANCH_30} | cut -f 1` ;\
	echo Updating to ${ISTIO_REPOSITORY}@$${LATEST_COMMIT_30}; sed -i -e "s/^\(ISTIO_COMMIT_30 ?= \).*$$/\1$${LATEST_COMMIT_30}/g" Makefile

.PHONY: patch-istio-images
patch-istio-images: ## Patch the Istio images in the ClusterServiceVersion with the right tags
	sed -i -e "s/images3_0.cni: .*/images3_0.cni: $(REGISTRY_URL)\/$(REGISTRY_ORG)\/$(ISTIO_CNI_IMAGE_NAME):$(TAG)/" \
		-e "s/images3_0.istiod: .*/images3_0.istiod: $(REGISTRY_URL)\/$(REGISTRY_ORG)\/$(ISTIO_PILOT_IMAGE_NAME):$(TAG)/" \
		-e "s/images3_0.proxy: .*/images3_0.proxy: $(REGISTRY_URL)\/$(REGISTRY_ORG)\/$(ISTIO_PROXY_IMAGE_NAME):$(TAG)/" \
		$(shell ls bundle/manifests/*.clusterserviceversion.yaml)

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
OPERATOR_SDK_VERSION ?= v1.28.1
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.11.1

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK)
operator-sdk: OS=$(shell go env GOOS)
operator-sdk: ARCH=$(shell go env GOARCH)
$(OPERATOR_SDK): $(LOCALBIN)
	@if test -x $(LOCALBIN)/operator-sdk && ! $(LOCALBIN)/operator-sdk version | grep -q $(OPERATOR_SDK_VERSION); then \
		echo "$(LOCALBIN)/operator-sdk version is not expected $(OPERATOR_SDK_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/operator-sdk; \
	fi
	test -s $(LOCALBIN)/operator-sdk || \
	wget -q -c https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$(OS)_$(ARCH) \
		-O $(LOCALBIN)/operator-sdk && \
	chmod +x $(LOCALBIN)/operator-sdk;

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: bundle
bundle: gen kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMAGE)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMAGE=$(BUNDLE_IMG)

.PHONY: bundle-publish
bundle-publish: patch-istio-images ## Create a PR for publishing in OperatorHub
	./hack/operatorhub/publish-bundle.sh

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

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMAGE=$(CATALOG_IMG)


##@ Linting

FINDFILES=find . \( -path ./.git -o -path ./.github -o -path ./tmp \) -prune -o -type f
XARGS = xargs -0 -r

.PHONY: lint-scripts
lint-scripts: ## runs linters against all shell scripts
	@${FINDFILES} -name '*.sh' -print0 | ${XARGS} shellcheck

.PHONY: lint-go
lint-go: ## runs linters against Go code
	golangci-lint run -c .golangci.yml -v

.PHONY: lint-yaml
lint-yaml: ## runs linters against YAML files
	@${FINDFILES} \( -name '*.yml' -o -name '*.yaml' \) -not -exec grep -q -e "{{" {} \; -print0 | ${XARGS} yamllint -f parsable

.PHONY: lint-helm
lint-helm: ## runs linters against helm charts
	@echo "Helm version: `helm version`"
	@${FINDFILES} -name 'Chart.yaml' -path './resources/helm/v3.?/*' \
	-print0 | ${XARGS} -L 1 dirname | xargs -r helm lint --strict

.PHONY: lint
lint: lint-scripts lint-go lint-yaml lint-helm ## runs all linters
