## Copyright 2019 Red Hat, Inc.
##
## Licensed under the Apache License, Version 2.0 (the "License");
## you may not use this file except in compliance with the License.
## You may obtain a copy of the License at
##
##     http://www.apache.org/licenses/LICENSE-2.0
##
## Unless required by applicable law or agreed to in writing, software
## distributed under the License is distributed on an "AS IS" BASIS,
## WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
## See the License for the specific language governing permissions and
## limitations under the License.

-include Makefile.overrides

# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 3.0.0
MINOR_VERSION := $(shell v='$(VERSION)'; echo "$${v%.*}")

OPERATOR_NAME ?= sailoperator


# Istio images names
ISTIO_CNI_IMAGE_NAME ?= install-cni
ISTIO_PILOT_IMAGE_NAME ?= pilot
ISTIO_PROXY_IMAGE_NAME ?= proxyv2

# GitHub creds
GITHUB_USER ?= maistra-bot
GITHUB_TOKEN ?= 

SOURCE_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

# Git repository state
ifndef GIT_TAG
GIT_TAG := $(shell git describe 2> /dev/null || echo "unknown")
endif
ifndef GIT_REVISION
GIT_REVISION := $(shell git rev-parse --verify HEAD 2> /dev/null || echo "unknown")
endif
ifndef GIT_STATUS
GIT_STATUS := $(shell git diff-index --quiet HEAD -- 2> /dev/null; if [ "$$?" = "0" ]; then echo Clean; else echo Modified; fi)
endif

# Linker flags for the go builds
GO_MODULE = maistra.io/istio-operator
LD_EXTRAFLAGS  = -X ${GO_MODULE}/pkg/version.buildVersion=${VERSION}
LD_EXTRAFLAGS += -X ${GO_MODULE}/pkg/version.buildGitRevision=${GIT_REVISION}
LD_EXTRAFLAGS += -X ${GO_MODULE}/pkg/version.buildTag=${GIT_TAG}
LD_EXTRAFLAGS += -X ${GO_MODULE}/pkg/version.buildStatus=${GIT_STATUS}
LD_FLAGS = -extldflags -static ${LD_EXTRAFLAGS} -s -w

# Image hub to use
HUB ?= quay.io/maistra-dev
# Image tag to use
TAG ?= ${MINOR_VERSION}-latest
# Image base to use
IMAGE_BASE ?= istio-operator
# Image URL to use all building/pushing image targets
IMAGE ?= ${HUB}/${IMAGE_BASE}:${TAG}
# Namespace to deploy the controller in
NAMESPACE ?= istio-operator
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.26.0

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
CHANNELS ?= "3.0"
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS = --channels=\"$(CHANNELS)\"
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
# quay.io/maistra-dev/istio-operator-bundle:$VERSION and quay.io/maistra-dev/istio-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= ${HUB}/${IMAGE_BASE}

BUNDLE_MANIFEST_DATE := $(shell cat bundle/manifests/${OPERATOR_NAME}.clusterserviceversion.yaml 2>/dev/null | grep createdAt | awk '{print $$2}')

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

TODAY ?= $(shell date -I)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /bin/bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

export

##@ Testing

.PHONY: test
test: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

.PHONY: test.scorecard ## Runs the operator scorecard test. Needs a valid k8s cluster as pointed by the KUBECONFIG variable
test.scorecard: operator-sdk
	$(OPERATOR_SDK) scorecard bundle

.PHONY: test.integration.ocp
test.integration.ocp:
	${SOURCE_DIR}/tests/integration/integ-suite-ocp.sh

.PHONY: test.integration.kind
test.integration.kind:
	${SOURCE_DIR}/tests/integration/integ-suite-kind.sh

##@ Build

.PHONY: build
build: build-amd64 ## Build manager binary.

.PHONY: run
run: gen ## Run a controller from your host.
	POD_NAMESPACE=${NAMESPACE} go run ./cmd/main.go --config-file=./hack/config.properties --resource-directory=./resources

# docker build -t ${IMAGE} --build-arg GIT_TAG=${GIT_TAG} --build-arg GIT_REVISION=${GIT_REVISION} --build-arg GIT_STATUS=${GIT_STATUS} .
.PHONY: docker-build
docker-build: build ## Build docker image with the manager.
	docker build -t ${IMAGE} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMAGE}

.PHONY: docker-push-nightly ## Build and push nightly docker image with the manager.
docker-push-nightly: TAG=$(MINOR_VERSION)-nightly-$(TODAY)
docker-push-nightly: docker-build
	docker push ${IMAGE}
	docker tag ${IMAGE} $(HUB)/$(IMAGE_BASE):$(MINOR_VERSION)-latest
	docker push $(HUB)/$(IMAGE_BASE):$(MINOR_VERSION)-latest

# NIGHTLY defines if the nightly image should be pushed or not
NIGHTLY ?= false

# BUILDX_OUTPUT defines the buildx output
# --load builds locally the container image
# --push builds and pushes the container image to a registry
BUILDX_OUTPUT ?= --push

# BUILDX_TAGS are the --tag flag passed to the docker buildx build command.
BUILDX_TAGS = --tag ${IMAGE}
ifeq ($(NIGHTLY),true)
BUILDX_TAGS += --tag $(HUB)/$(IMAGE_BASE):$(MINOR_VERSION)-nightly-$(TODAY)
endif

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMAGE=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMAGE=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
PLATFORM_ARCHITECTURES = $(shell echo ${PLATFORMS} | sed -e 's/,/\ /g' -e 's/linux\///g')

define BUILDX
.PHONY: build-$(1)
build-$(1): ## Build manager binary for specific architecture.
	GOARCH=$(1) LDFLAGS="$(LD_FLAGS)" common/scripts/gobuild.sh $(REPO_ROOT)/out/$(TARGET_OS)_$(1)/manager main.go

.PHONY: build-all
build-all: build-$(1)
endef

$(foreach arch,$(PLATFORM_ARCHITECTURES),$(eval $(call BUILDX,$(arch))))

.PHONY: docker-buildx
docker-buildx: test build-all ## Build and push (by default) docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- docker buildx create --name project-v3-builder
	docker buildx use project-v3-builder
	- docker buildx build $(BUILDX_OUTPUT) --platform=$(PLATFORMS) $(BUILDX_TAGS) -f Dockerfile.cross .
	- docker buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: gen-manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl create ns ${NAMESPACE} || echo "namespace ${NAMESPACE} already exists"
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	$(info NAMESPACE: $(NAMESPACE))
	$(MAKE) -s deploy-yaml | kubectl apply -f -

.PHONY: deploy-yaml
deploy-yaml: kustomize ## Outputs YAML manifests needed to deploy the controller
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMAGE}
	cd config/default && $(KUSTOMIZE) edit set namespace ${NAMESPACE}
	$(KUSTOMIZE) build config/default

.PHONY: deploy-openshift # TODO: remove this target and use deploy-olm instead (when we fix the internal registry TLS issues when using operator-sdk run bundle)
deploy-openshift: kustomize ## Deploy controller to OpenShift via YAML manifests
	$(info NAMESPACE: $(NAMESPACE))
	$(MAKE) -s deploy-yaml-openshift | kubectl apply -f -

.PHONY: deploy-yaml-openshift
deploy-yaml-openshift: kustomize ## Outputs YAML manifests needed to deploy the controller in OpenShift
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMAGE}
	cd config/default && $(KUSTOMIZE) edit set namespace ${NAMESPACE}
	$(KUSTOMIZE) build config/openshift

.PHONY: deploy-olm
deploy-olm: bundle bundle-build bundle-push ## Builds and pushes the operator OLM bundle and then deploys the operator using OLM
	kubectl create ns ${NAMESPACE} || echo "namespace ${NAMESPACE} already exists"
	$(OPERATOR_SDK) run bundle $(BUNDLE_IMG) -n ${NAMESPACE}

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	kubectl delete istios.operator.istio.io --all --all-namespaces --wait=true
	$(MAKE) deploy-yaml | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: undeploy-olm
undeploy-olm: operator-sdk ## Undeploys the operator from the cluster (used only if operator was installed via OLM)
	kubectl delete istios.operator.istio.io --all --all-namespaces --wait=true
	$(OPERATOR_SDK) cleanup sailoperator --delete-all -n ${NAMESPACE}

.PHONY: deploy-example
deploy-example: deploy-example-openshift

.PHONY: deploy-example-openshift
deploy-example-openshift: ## Deploy an example Istio resource on OpenShift
	kubectl create ns istio-system || echo "namespace istio-system already exists"
	kubectl apply -n istio-system -f config/samples/istio-sample-openshift.yaml

.PHONY: deploy-example-kubernetes
deploy-example-kubernetes: ## Deploy an example Istio resource on Kubernetes
	kubectl create ns istio-system || echo "namespace istio-system already exists"
	kubectl apply -n istio-system -f config/samples/istio-sample-kubernetes.yaml

##@ Generated Code & Resources

.PHONY: gen-manifests
gen-manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: gen-code
gen-code: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: gen-charts
gen-charts: ## Pull charts from maistra/istio repository
	@# use yq to generate a list of download-charts.sh commands for each version in versions.yaml; these commands are
	@# passed to sh and executed; in a nutshell, the yq command generates commands like:
	@# ./hack/download-charts.sh <version> <git repo> <commit> [chart1] [chart2] ...
	@yq eval '.versions | to_entries | .[] | "./hack/download-charts.sh " + .key + " " + .value.repo + " " + .value.commit + " " + ((.value.charts // []) | join(" "))' < versions.yaml | sh

	@# remove old version directories
	@hack/remove-old-versions.sh

	@# find the profiles used in the downloaded charts and update list of available profiles
	@hack/update-profiles-list.sh

	@# update the urn:alm:descriptor:com.tectonic.ui:select entries in istio_types.go to match the supported versions of the Helm charts
	@hack/update-version-list.sh

	@# calls copy-crds.sh with the version specified in the .crdSourceVersion field in versions.yaml
	@hack/copy-crds.sh "resources/$$(yq eval '.crdSourceVersion' versions.yaml)/charts"

.PHONY: gen ## Generate everything
gen: controller-gen gen-charts gen-manifests gen-code patch-istio-crd bundle

.PHONY: gen-check
gen-check: gen restore-manifest-dates check-clean-repo ## Verifies that changes in generated resources have been checked in

.PHONY: restore-manifest-dates
restore-manifest-dates:
ifneq "${BUNDLE_MANIFEST_DATE}" ""
	@sed -i -e "s/\(createdAt:\).*/\1 \"${BUNDLE_MANIFEST_DATE}\"/" bundle/manifests/${OPERATOR_NAME}.clusterserviceversion.yaml
endif

.PHONY: update-istio
update-istio: ## Updates the Istio commit hash in the 'latest' entry in versions.yaml to the latest commit in the branch
	@hack/update-istio.sh

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
OPM ?= $(LOCALBIN)/opm

## Tool Versions
  OPERATOR_SDK_VERSION ?= v1.33.0
  KUSTOMIZE_VERSION ?= v5.3.0
  CONTROLLER_TOOLS_VERSION ?= v0.13.0
  OPM_VERSION ?= v1.33.0

.PHONY: kustomize $(KUSTOMIZE)
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION) > /dev/stderr; then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing." > /dev/stderr; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	@test -s $(LOCALBIN)/kustomize || GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION) > /dev/stderr
.PHONY: operator-sdk $(OPERATOR_SDK)
operator-sdk: $(OPERATOR_SDK)
operator-sdk: OS=$(shell go env GOOS)
operator-sdk: ARCH=$(shell go env GOARCH)
$(OPERATOR_SDK): $(LOCALBIN)
	@if test -x $(LOCALBIN)/operator-sdk && ! $(LOCALBIN)/operator-sdk version | grep -q $(OPERATOR_SDK_VERSION); then \
		echo "$(LOCALBIN)/operator-sdk version is not expected $(OPERATOR_SDK_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/operator-sdk; \
	fi
	@test -s $(LOCALBIN)/operator-sdk || \
	curl -sSLfo $(LOCALBIN)/operator-sdk https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$(OS)_$(ARCH) && \
	chmod +x $(LOCALBIN)/operator-sdk;

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	@test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: bundle
bundle: gen kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMAGE)
	sed -i "s|^\(    containerImage:\).*$$|\1 ${IMAGE}|g" config/manifests/bases/${OPERATOR_NAME}.clusterserviceversion.yaml
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)

	# check if the only change in the CSV is the createdAt timestamp; if so, revert the change
	@csvPath="bundle/manifests/${OPERATOR_NAME}.clusterserviceversion.yaml"; \
		if ! (git diff "$$csvPath" | grep '^[+-][^+-][^+-]' | grep -v "createdAt:" >/dev/null); then \
			echo "reverting timestamp change in $$csvPath"; \
			git checkout "$$csvPath"; \
		fi
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMAGE=$(BUNDLE_IMG)

.PHONY: bundle-publish
bundle-publish: ## Create a PR for publishing in OperatorHub
	export GIT_USER=$(GITHUB_USER); \
	export GITHUB_TOKEN=$(GITHUB_TOKEN); \
	export OPERATOR_VERSION=$(OPERATOR_VERSION); \
	export OPERATOR_NAME=$(OPERATOR_NAME); \
	./hack/operatorhub/publish-bundle.sh

.PHONY: bundle-nightly ## Generate nightly bundle
bundle-nightly: VERSION:=$(VERSION)-nightly-$(TODAY)
bundle-nightly: CHANNELS:=$(MINOR_VERSION)-nightly
bundle-nightly: TAG=$(MINOR_VERSION)-nightly-$(TODAY)
bundle-nightly: bundle

.PHONY: bundle-publish-nightly
bundle-publish-nightly: OPERATOR_VERSION=$(VERSION)-nightly-$(TODAY)
bundle-publish-nightly: TAG=$(MINOR_VERSION)-nightly-$(TODAY)
bundle-publish-nightly: bundle-nightly bundle-publish

.PHONY: patch-istio-crd
patch-istio-crd: ## Update Istio CRD's openAPIV3Schema values
	@hack/patch-istio-crd.sh

.PHONY: opm $(OPM)
opm: $(OPM)
opm: OS=$(shell go env GOOS)
opm: ARCH=$(shell go env GOARCH)
$(OPM): $(LOCALBIN)
	@if test -x $(LOCALBIN)/opm && ! $(LOCALBIN)/opm version | grep -q $(OPM_VERSION); then \
		echo "$(LOCALBIN)/opm version is not expected $(OPM_VERSION). Removing it before installing."; \
		rm -f $(LOCALBIN)/opm; \
	fi
	test -s $(LOCALBIN)/opm || \
	curl -sSLfo $(LOCALBIN)/opm https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/$(OS)-$(ARCH)-opm && \
	chmod +x $(LOCALBIN)/opm;

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

.PHONY: lint-bundle
lint-bundle: operator-sdk ## runs linters against OLM metadata bundle
	$(OPERATOR_SDK) bundle validate bundle --select-optional suite=operatorframework

.PHONY: lint-watches
lint-watches: ## checks if the operator watches all resource kinds present in Helm charts
	@hack/lint-watches.sh

.PHONY: lint
lint: lint-scripts lint-copyright-banner lint-go lint-yaml lint-helm lint-bundle lint-watches ## runs all linters

.PHONY: format
format: format-go tidy-go ## Auto formats all code. This should be run before sending a PR.

.SILENT: kustomize $(KUSTOMIZE) $(LOCALBIN) deploy-yaml

include common/Makefile.common.mk
