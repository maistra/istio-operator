# Copyright Istio Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Save the current, builtin variables so we can filter them out in the `print-variables` target
OLD_VARS := $(.VARIABLES)

# Most variables defined in this Makefile can be overriden in Makefile.vendor.mk
# Use `make print-variables` to inspect the values of the variables
-include Makefile.vendor.mk

VERSION ?= 3.0.0
MINOR_VERSION := $(shell v='$(VERSION)'; echo "$${v%.*}")

OPERATOR_NAME ?= sailoperator
VERSIONS_YAML_FILE ?= versions.yaml

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
GO_MODULE = github.com/istio-ecosystem/sail-operator
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
IMAGE_BASE ?= sail-operator
# Image URL to use all building/pushing image targets
IMAGE ?= ${HUB}/${IMAGE_BASE}:${TAG}
# Namespace to deploy the controller in
NAMESPACE ?= sail-operator
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION ?= 1.29.0

# Set DOCKER_BUILD_FLAGS to specify flags to pass to 'docker build', default to empty. Example: --platform=linux/arm64
DOCKER_BUILD_FLAGS ?= "--platform=$(TARGET_OS)/$(TARGET_ARCH)"

VERBOSE_FLAG := $(if $(VERBOSE),-v)

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
# quay.io/sail-dev/sail-operator-bundle:$VERSION and quay.io/sail-dev/sail-operator-catalog:$VERSION.
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

# Default values and flags used when rendering chart templates locally
HELM_VALUES_FILE ?= chart/values.yaml
HELM_TEMPL_DEF_FLAGS ?= --include-crds --values $(HELM_VALUES_FILE)

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
test: test.unit test.integration ## Run both unit tests and integration test.

.PHONY: test.unit
test.unit:  ## Run unit tests.
	go test $(VERBOSE_FLAG) ./...

.PHONY: test.integration
test.integration: envtest ## Run integration tests located in the tests/integration directory.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	go test --tags=integration $(VERBOSE_FLAG) ./tests/integration/...

.PHONY: test.scorecard
test.scorecard: operator-sdk ## Run the operator scorecard test.
	OPERATOR_SDK=$(OPERATOR_SDK) ${SOURCE_DIR}/tests/scorecard-test.sh

.PHONY: test.e2e.ocp
test.e2e.ocp: ## Run the end-to-end tests against an existing OCP cluster.
	${SOURCE_DIR}/tests/e2e/integ-suite-ocp.sh

.PHONY: test.e2e.kind
test.e2e.kind: ## Deploy a KinD cluster and run the end-to-end tests against it.
	${SOURCE_DIR}/tests/e2e/integ-suite-kind.sh

.PHONY: test.e2e.describe
test.e2e.describe: ## Runs ginkgo outline -format indent over the e2e test to show in BDD style the steps and test structure
	${SOURCE_DIR}/tests/e2e/common-operator-integ-suite.sh --describe

##@ Build

.PHONY: build
build: build-$(TARGET_ARCH) ## Build manager binary.

.PHONY: run
run: gen ## Run a controller from your host.
	POD_NAMESPACE=${NAMESPACE} go run ./cmd/main.go --config-file=./hack/config.properties --resource-directory=./resources

# docker build -t ${IMAGE} --build-arg GIT_TAG=${GIT_TAG} --build-arg GIT_REVISION=${GIT_REVISION} --build-arg GIT_STATUS=${GIT_STATUS} .
.PHONY: docker-build
docker-build: build ## Build docker image.
	docker build ${DOCKER_BUILD_FLAGS} -t ${IMAGE} .

PHONY: push
push: docker-push ## Build and push docker image.

.PHONY: docker-push
docker-push: docker-build ## Build and Push docker image.
	docker push ${IMAGE}

.PHONY: docker-push-nightly
docker-push-nightly: TAG=$(MINOR_VERSION)-nightly-$(TODAY) ## Build and push nightly docker image.
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

# BUILDX_ADDITIONAL_TAGS are the additional --tag flags passed to the docker buildx build command.
BUILDX_ADDITIONAL_TAGS =
ifeq ($(NIGHTLY),true)
BUILDX_ADDITIONAL_TAGS += --tag $(HUB)/$(IMAGE_BASE):$(MINOR_VERSION)-nightly-$(TODAY)
endif

# BUILDX_BUILD_ARGS are the additional --build-arg flags passed to the docker buildx build command.
BUILDX_BUILD_ARGS = --build-arg TARGETOS=$(TARGET_OS)

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMAGE=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMAGE=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
PLATFORM_ARCHITECTURES = $(shell echo ${PLATFORMS} | sed -e 's/,/\ /g' -e 's/linux\///g')

ifndef BUILDX
define BUILDX
.PHONY: build-$(1)
build-$(1): ## Build manager binary for specific architecture.
	GOARCH=$(1) LDFLAGS="$(LD_FLAGS)" common/scripts/gobuild.sh $(REPO_ROOT)/out/$(TARGET_OS)_$(1)/manager cmd/main.go

.PHONY: build-all
build-all: build-$(1)
endef

$(foreach arch,$(PLATFORM_ARCHITECTURES),$(eval $(call BUILDX,$(arch))))
undefine BUILDX
endif

.PHONY: docker-buildx
docker-buildx: test build-all ## Build and push docker image with cross-platform support.
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- docker buildx create --name project-v4-builder
	docker buildx use project-v4-builder
	- docker buildx build $(BUILDX_OUTPUT) --platform=$(PLATFORMS) --tag ${IMAGE} $(BUILDX_ADDITIONAL_TAGS) $(BUILDX_BUILD_ARGS) -f Dockerfile.cross .
	- docker buildx rm project-v4-builder
	rm Dockerfile.cross

##@ Deployment

.PHONY: install
install: gen-manifests ## Install CRDs into an existing cluster.
	kubectl create ns ${NAMESPACE} || echo "namespace ${NAMESPACE} already exists"
	kubectl apply --server-side=true -f chart/crds

.PHONY: uninstall
uninstall: ## Uninstall CRDs from an existing cluster.
	kubectl delete --ignore-not-found -f chart/crds

.PHONY: deploy
deploy: helm ## Deploy controller to an existing cluster.
	$(info NAMESPACE: $(NAMESPACE))
	kubectl create ns ${NAMESPACE} || echo "namespace ${NAMESPACE} already exists"
	$(HELM) template chart chart $(HELM_TEMPL_DEF_FLAGS) --set image='$(IMAGE)' --namespace $(NAMESPACE) | kubectl apply --server-side=true -f -

.PHONY: deploy-yaml
deploy-yaml: helm ## Output YAML manifests used by `deploy`.
	$(HELM) template chart chart $(HELM_TEMPL_DEF_FLAGS) --set image='$(IMAGE)' --namespace $(NAMESPACE)

.PHONY: deploy-openshift # TODO: remove this target and use deploy-olm instead (when we fix the internal registry TLS issues when using operator-sdk run bundle)
deploy-openshift: helm ## Deploy controller to an existing OCP cluster.
	$(info NAMESPACE: $(NAMESPACE))
	$(HELM) template chart chart $(HELM_TEMPL_DEF_FLAGS) --set image='$(IMAGE)' --namespace $(NAMESPACE) --set platform="openshift" | kubectl apply --server-side=true -f -

.PHONY: deploy-yaml-openshift
deploy-yaml-openshift: helm ## Output YAML manifests used by `deploy-openshift`.
	$(HELM) template chart chart $(HELM_TEMPL_DEF_FLAGS) --set image='$(IMAGE)' --namespace $(NAMESPACE) --set platform="openshift"

.PHONY: deploy-olm
deploy-olm: bundle bundle-build bundle-push ## Build and push the operator OLM bundle and deploy the operator using OLM.
	kubectl create ns ${NAMESPACE} || echo "namespace ${NAMESPACE} already exists"
	$(OPERATOR_SDK) run bundle $(BUNDLE_IMG) -n ${NAMESPACE}

.PHONY: undeploy
undeploy: ## Undeploy controller from an existing cluster.
	kubectl delete istios.operator.istio.io --all --all-namespaces --wait=true
	$(MAKE) -e HELM_TEMPL_DEF_FLAGS="$(HELM_TEMPL_DEF_FLAGS)" deploy-yaml | kubectl delete --ignore-not-found -f -
	kubectl delete ns ${NAMESPACE} --ignore-not-found
	$(HELM) template chart chart $(HELM_TEMPL_DEF_FLAGS) --set image='$(IMAGE)' --namespace $(NAMESPACE) | kubectl delete --ignore-not-found -f -

.PHONY: undeploy-olm
undeploy-olm: operator-sdk ## Undeploy the operator from an existing cluster (used only if operator was installed via OLM).
	kubectl delete istios.operator.istio.io --all --all-namespaces --wait=true
	$(OPERATOR_SDK) cleanup $(OPERATOR_NAME) --delete-all -n ${NAMESPACE}

.PHONY: deploy-example
deploy-example: deploy-example-openshift ## Deploy an example Istio resource to an existing OCP cluster. Same as `deploy-example-openshift`.

.PHONY: deploy-example-openshift
deploy-example-openshift: ## Deploy an example Istio resource to an existing OCP cluster.
	kubectl create ns istio-system || echo "namespace istio-system already exists"
	kubectl apply -n istio-system -f chart/samples/istio-sample-openshift.yaml

.PHONY: deploy-example-kubernetes
deploy-example-kubernetes: ## Deploy an example Istio resource on an existing cluster.
	kubectl create ns istio-system || echo "namespace istio-system already exists"
	kubectl apply -n istio-system -f chart/samples/istio-sample-kubernetes.yaml

##@ Generated Code & Resources

.PHONY: gen-manifests
gen-manifests: controller-gen ## Generate WebhookConfiguration and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd:allowDangerousTypes=true webhook paths="./..." output:crd:artifacts:config=chart/crds

# TODO: move this to versions.yaml or get the files via go.mod instead of downloading them
ISTIO_REPO_BASE=https://raw.githubusercontent.com/istio/istio/0e7ecbd31f9524b063ced1f49f1a6f6e063d2bf5
API_REPO_BASE=https://raw.githubusercontent.com/istio/api/ccd5cd40965ccba232d1f7c3b0e4ecacd0f6ceda
.PHONY: gen-api
gen-api: ## Generate API types from upstream files.
	# TODO: should we get these files from the local filesystem by inspecting go.mod?
	echo Generating API types from upstream files
	curl -sSLfo /tmp/values_types.pb.go $(ISTIO_REPO_BASE)/operator/pkg/apis/istio/v1alpha1/values_types.pb.go
	curl -sSLfo /tmp/config.pb.go $(API_REPO_BASE)/mesh/v1alpha1/config.pb.go
	curl -sSLfo /tmp/network.pb.go $(API_REPO_BASE)/mesh/v1alpha1/network.pb.go
	curl -sSLfo /tmp/proxy.pb.go $(API_REPO_BASE)/mesh/v1alpha1/proxy.pb.go
	curl -sSLfo /tmp/proxy_config.pb.go $(API_REPO_BASE)/networking/v1beta1/proxy_config.pb.go
	curl -sSLfo /tmp/selector.pb.go $(API_REPO_BASE)/type/v1beta1/selector.pb.go
	curl -sSLfo /tmp/destination_rule.pb.go $(API_REPO_BASE)/networking/v1alpha3/destination_rule.pb.go
	curl -sSLfo /tmp/virtual_service.pb.go $(API_REPO_BASE)/networking/v1alpha3/virtual_service.pb.go
	go run hack/api_transformer/main.go hack/api_transformer/transform.yaml

.PHONY: gen-code
gen-code: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="common/scripts/copyright-banner-go.txt" paths="./..."

.PHONY: gen-charts
gen-charts: ## Pull charts from istio repository.
	@# use yq to generate a list of download-charts.sh commands for each version in versions.yaml; these commands are
	@# passed to sh and executed; in a nutshell, the yq command generates commands like:
	@# ./hack/download-charts.sh <version> <git repo> <commit> [chart1] [chart2] ...
	@yq eval '.versions[] | "./hack/download-charts.sh " + .name + " " + .repo + " " + .commit + " " + ((.charts // []) | join(" "))' < $(VERSIONS_YAML_FILE) | sh

	@# remove old version directories
	@hack/remove-old-versions.sh

	@# find the profiles used in the downloaded charts and update list of available profiles
	@hack/update-profiles-list.sh

	@# update the urn:alm:descriptor:com.tectonic.ui:select entries in istio_types.go to match the supported versions of the Helm charts
	@hack/update-version-list.sh

	@# calls copy-crds.sh with the version specified in the .crdSourceVersion field in versions.yaml
	@hack/copy-crds.sh "resources/$$(yq eval '.crdSourceVersion' $(VERSIONS_YAML_FILE))/charts"

.PHONY: gen
gen: controller-gen gen-api gen-charts gen-manifests gen-code bundle ## Generate everything.

.PHONY: gen-check
gen-check: gen restore-manifest-dates check-clean-repo ## Verify that changes in generated resources have been checked in.

.PHONY: restore-manifest-dates
restore-manifest-dates:
ifneq "${BUNDLE_MANIFEST_DATE}" ""
	@sed -i -e "s/\(createdAt:\).*/\1 \"${BUNDLE_MANIFEST_DATE}\"/" bundle/manifests/${OPERATOR_NAME}.clusterserviceversion.yaml
endif

.PHONY: update-istio
update-istio: ## Update the Istio commit hash in the 'latest' entry in versions.yaml to the latest commit in the branch.
	@hack/update-istio.sh

.PHONY: print-variables
print-variables: ## Print all Makefile variables; Useful to inspect overrides of variables.
	$(foreach v,                                        \
  $(filter-out $(OLD_VARS) OLD_VARS,$(.VARIABLES)), \
  $(info $(v) = $($(v))))
	@echo

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
HELM ?= $(LOCALBIN)/helm
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
OPM ?= $(LOCALBIN)/opm

## Tool Versions
OPERATOR_SDK_VERSION ?= v1.34.1
HELM_VERSION ?= v3.14.3
CONTROLLER_TOOLS_VERSION ?= v0.14.0
OPM_VERSION ?= v1.37.0

.PHONY: helm $(HELM)
helm: $(HELM) ## Download helm to bin directory. If wrong version is installed, it will be overwritten.
$(HELM): $(LOCALBIN)
	@if test -x $(LOCALBIN)/helm && ! $(LOCALBIN)/helm version | grep -q $(shell v='$(HELM_VERSION)'; echo "$${v%.*}") > /dev/stderr; then \
		echo "$(LOCALBIN)/helm version is not expected $(HELM_VERSION). Removing it before installing." > /dev/stderr; \
		rm -rf $(LOCALBIN)/helm; \
	fi
	@test -s $(LOCALBIN)/helm || GOBIN=$(LOCALBIN) GO111MODULE=on go install helm.sh/helm/v3/cmd/helm@$(HELM_VERSION) > /dev/stderr
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
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen to bin directory. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	@test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup to bin directory.
$(ENVTEST): $(LOCALBIN)
	@test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: bundle
bundle: gen helm operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(HELM) template chart chart $(HELM_TEMPL_DEF_FLAGS) --set image='$(IMAGE)' --set platform=openshift --set bundleGeneration=true | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)

	# update CSV's spec.customresourcedefinitions.owned field. ideally we could do this straight in ./bundle, but
	# sadly this is only possible if the file lives in a `bases` directory
	mkdir -p _tmp/bases
	cp bundle/manifests/$(OPERATOR_NAME).clusterserviceversion.yaml _tmp/bases
	$(OPERATOR_SDK) generate kustomize manifests --input-dir=_tmp --output-dir=_tmp
	mv _tmp/bases/$(OPERATOR_NAME).clusterserviceversion.yaml bundle/manifests/$(OPERATOR_NAME).clusterserviceversion.yaml
	rm -rf _tmp

	# check if the only change in the CSV is the createdAt timestamp; if so, revert the change
	@csvPath="bundle/manifests/${OPERATOR_NAME}.clusterserviceversion.yaml"; \
		if (git ls-files --error-unmatch "$$csvPath" &>/dev/null); then \
			if ! (git diff "$$csvPath" | grep '^[+-][^+-][^+-]' | grep -v "createdAt:" >/dev/null); then \
				echo "reverting timestamp change in $$csvPath"; \
				git checkout "$$csvPath"; \
			fi \
		fi
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMAGE=$(BUNDLE_IMG)

.PHONY: bundle-publish
bundle-publish: ## Create a PR for publishing in OperatorHub.
	export GIT_USER=$(GITHUB_USER); \
	export GITHUB_TOKEN=$(GITHUB_TOKEN); \
	export OPERATOR_VERSION=$(OPERATOR_VERSION); \
	export OPERATOR_NAME=$(OPERATOR_NAME); \
	./hack/operatorhub/publish-bundle.sh

.PHONY: bundle-nightly
bundle-nightly: VERSION:=$(VERSION)-nightly-$(TODAY)  ## Generate nightly bundle.
bundle-nightly: CHANNELS:=$(MINOR_VERSION)-nightly
bundle-nightly: TAG=$(MINOR_VERSION)-nightly-$(TODAY)
bundle-nightly: bundle

.PHONY: bundle-publish-nightly
bundle-publish-nightly: OPERATOR_VERSION=$(VERSION)-nightly-$(TODAY)  ## Publish nightly bundle.
bundle-publish-nightly: TAG=$(MINOR_VERSION)-nightly-$(TODAY)
bundle-publish-nightly: bundle-nightly bundle-publish

.PHONY: patch-istio-crd
patch-istio-crd: ## Update Istio CRD's openAPIV3Schema values.
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
lint-bundle: operator-sdk ## Run linters against OLM metadata bundle.
	$(OPERATOR_SDK) bundle validate bundle --select-optional suite=operatorframework

.PHONY: lint-watches
lint-watches: ## Checks if the operator watches all resource kinds present in Helm charts.
	@hack/lint-watches.sh

.PHONY: lint
lint: lint-scripts lint-copyright-banner lint-go lint-yaml lint-helm lint-bundle lint-watches ## Run all linters.

.PHONY: format
format: format-go tidy-go ## Auto-format all code. This should be run before sending a PR.

.SILENT: helm $(HELM) $(LOCALBIN) deploy-yaml gen-api

COMMON_IMPORTS ?= lint-all lint-scripts lint-copyright-banner lint-go lint-yaml lint-helm format-go tidy-go check-clean-repo update-common
.PHONY: $(COMMON_IMPORTS)
$(COMMON_IMPORTS):
	@$(MAKE) --no-print-directory -f common/Makefile.common.mk $@
