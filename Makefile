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

MAISTRA_VERSION        ?= 2.1.0
MAISTRA_BRANCH         ?= maistra-2.1
REPLACES_PRODUCT_CSV   ?= 2.0.2
REPLACES_COMMUNITY_CSV ?= 2.0.2
VERSION                ?= development
IMAGE                  ?= docker.io/maistra/istio-ubi8-operator:${MAISTRA_VERSION}
CONTAINER_CLI          ?= docker
COMMUNITY              ?= true
TEST_TIMEOUT           ?= 5m
TEST_FLAGS             ?=

SOURCE_DIR          := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
RESOURCES_DIR        = ${SOURCE_DIR}/resources
OUT_DIR              = ${SOURCE_DIR}/tmp/_output
TEMPLATES_OUT_DIR    = ${OUT_DIR}/resources/default-templates
HELM_OUT_DIR         = ${OUT_DIR}/resources/helm
OLM_MANIFEST_OUT_DIR = ${OUT_DIR}/resources/manifests

OFFLINE_BUILD       ?= false
GIT_UPSTREAM_REMOTE ?= $(shell git remote -v |grep --color=never '[/:][Mm]aistra/istio-operator\.git.*(fetch)' |grep --color=never -o '^[^[:space:]]*')

MAISTRA_MANIFEST_DATE := $(shell cat manifests-maistra/${MAISTRA_VERSION}/maistraoperator.v${MAISTRA_VERSION}.clusterserviceversion.yaml 2>/dev/null | grep createdAt | awk '{print $$2}')
OSSM_MANIFEST_DATE := $(shell cat manifests-servicemesh/${MAISTRA_VERSION}/servicemeshoperator.v${MAISTRA_VERSION}.clusterserviceversion.yaml 2>/dev/null | grep createdAt | awk '{print $$2}')

ifeq "${GIT_UPSTREAM_REMOTE}" ""
GIT_UPSTREAM_REMOTE = "ci-upstream"
$(warning Could not find git remote for maistra/istio-operator, adding as '${GIT_UPSTREAM_REMOTE}')
$(shell git remote add ${GIT_UPSTREAM_REMOTE} https://github.com/maistra/istio-operator.git)
endif

ifeq "${COMMUNITY}" "true"
BUILD_TYPE = maistra
else
BUILD_TYPE = servicemesh
endif

$(info   Building $(BUILD_TYPE) operator)

export SOURCE_DIR OUT_DIR MAISTRA_BRANCH MAISTRA_VERSION VERSION COMMUNITY BUILD_TYPE

################################################################################
# clean ./tmp/_output
################################################################################
.PHONY: clean
clean:
	rm -rf "${OUT_DIR}"

################################################################################
# compile go binary
################################################################################
.PHONY: compile
compile:
	${SOURCE_DIR}/build/build.sh

################################################################################
# runs all the tests
################################################################################
.PHONY: test
test:
	go test -timeout ${TEST_TIMEOUT} -mod=vendor ${TEST_FLAGS} ./...

################################################################################
# maistra v1.0
################################################################################
.PHONY: update-remote-maistra-1.0
update-remote-maistra-1.0:
ifeq "${OFFLINE_BUILD}" "false"
	git fetch ${GIT_UPSTREAM_REMOTE} maistra-1.0:maistra-1.0
endif

.PHONY: update-1.0-charts
update-1.0-charts: update-remote-maistra-1.0
	git checkout ${GIT_UPSTREAM_REMOTE}/maistra-1.0 -- ${SOURCE_DIR}/resources/helm/v1.0
	git reset HEAD ${SOURCE_DIR}/resources/helm/v1.0
	HELM_DIR=${RESOURCES_DIR}/helm/v1.0 ${SOURCE_DIR}/build/patch-container-image.sh
	find ${RESOURCES_DIR}/helm/v1.0/istio-init/files/ -maxdepth 1 -name "*.crd.yaml" -delete
	# MAISTRA-1776
	sed -i -e '/kind: handler/,/kind:/ { /name: kubernetesenv/,/kind:/ s/params:/params: \{\}/ }' ${RESOURCES_DIR}/helm/v1.0/istio/charts/mixer/templates/config.yaml
	sed -i -e '/if (\$$spec.sds) and (eq \$$spec.sds.enabled true)/ a\{\{- if $$spec.sds \}\}\n\{\{- if eq $$spec.sds.enabled true \}\}' ${RESOURCES_DIR}/helm/v1.0/istio/charts/gateways/templates/role.yaml ${RESOURCES_DIR}/helm/v1.0/istio/charts/gateways/templates/rolebindings.yaml
	sed -i -e '/if (\$$spec.sds) and (eq \$$spec.sds.enabled true)/ d' ${RESOURCES_DIR}/helm/v1.0/istio/charts/gateways/templates/role.yaml ${RESOURCES_DIR}/helm/v1.0/istio/charts/gateways/templates/rolebindings.yaml
	echo "{{- end }}" >> ${RESOURCES_DIR}/helm/v1.0/istio/charts/gateways/templates/role.yaml
	echo "{{- end }}" >> ${RESOURCES_DIR}/helm/v1.0/istio/charts/gateways/templates/rolebindings.yaml
	CRD_DIR=${RESOURCES_DIR}/helm/v1.0/istio-init/files ${SOURCE_DIR}/build/split-istio-crds.sh

.PHONY: update-1.0-templates
update-1.0-templates:
	curl -L https://github.com/maistra/istio-operator/archive/maistra-1.0.tar.gz | tar -xzvC ${SOURCE_DIR}/resources/smcp-templates/v1.0 --strip-components 3 */deploy/smcp-templates
# XXX: for now, the templates for maistra-1.0 are stored in ./deploy/smcp-templates, so the following won't work
#update-1.0-templates: update-remote-maistra-1.0
#	git checkout ${GIT_UPSTREAM_REMOTE}/maistra-1.0 -- ${SOURCE_DIR}/resources/smcp-templates/v1.0

.PHONY: collect-1.0-charts
collect-1.0-charts:
	mkdir -p ${HELM_OUT_DIR}
	cp -rf ${RESOURCES_DIR}/helm/v1.0 ${HELM_OUT_DIR}

.PHONY: collect-1.0-templates
collect-1.0-templates:
	mkdir -p ${TEMPLATES_OUT_DIR}/v1.0
	cp ${RESOURCES_DIR}/smcp-templates/v1.0/${BUILD_TYPE} ${TEMPLATES_OUT_DIR}/v1.0/default
	cp ${RESOURCES_DIR}/smcp-templates/v1.0/base ${TEMPLATES_OUT_DIR}/v1.0


################################################################################
# maistra v1.1
################################################################################
.PHONY: update-remote-maistra-1.1
update-remote-maistra-1.1:
ifeq "${OFFLINE_BUILD}" "false"
	git fetch ${GIT_UPSTREAM_REMOTE} maistra-1.1:maistra-1.1
endif

.PHONY: update-1.1-charts
update-1.1-charts: update-remote-maistra-1.1
	git checkout ${GIT_UPSTREAM_REMOTE}/maistra-1.1 -- ${SOURCE_DIR}/resources/helm/v1.1
	git reset HEAD ${SOURCE_DIR}/resources/helm/v1.1
	find ${RESOURCES_DIR}/helm/v1.1/istio-init/files/ -maxdepth 1 -name "*.crd.yaml" -delete
	# MAISTRA-1776
	sed -i -e '/kind: handler/,/kind:/ { /name: kubernetesenv/,/kind:/ s/params:/params: \{\}/ }' ${RESOURCES_DIR}/helm/v1.1/istio/charts/mixer/templates/config.yaml
	CRD_DIR=${RESOURCES_DIR}/helm/v1.1/istio-init/files ${SOURCE_DIR}/build/split-istio-crds.sh

.PHONY: update-1.1-templates
update-1.1-templates: update-remote-maistra-1.1
	git checkout ${GIT_UPSTREAM_REMOTE}/maistra-1.1 -- ${SOURCE_DIR}/resources/smcp-templates/v1.1
	git reset HEAD ${SOURCE_DIR}/resources/smcp-templates/v1.1

.PHONY: collect-1.1-charts
collect-1.1-charts:
	mkdir -p ${HELM_OUT_DIR}
	cp -rf ${RESOURCES_DIR}/helm/v1.1 ${HELM_OUT_DIR}

.PHONY: collect-1.1-templates
collect-1.1-templates:
	mkdir -p ${TEMPLATES_OUT_DIR}/v1.1
	cp ${RESOURCES_DIR}/smcp-templates/v1.1/${BUILD_TYPE} ${TEMPLATES_OUT_DIR}/v1.1/default
	cp ${RESOURCES_DIR}/smcp-templates/v1.1/base ${TEMPLATES_OUT_DIR}/v1.1

################################################################################
# maistra v2.0
################################################################################
.PHONY: update-remote-maistra-2.0
update-remote-maistra-2.0:
ifeq "${OFFLINE_BUILD}" "false"
	git fetch ${GIT_UPSTREAM_REMOTE} maistra-2.0:maistra-2.0
endif

.PHONY: update-2.0-charts
update-2.0-charts: update-remote-maistra-2.0
	git checkout ${GIT_UPSTREAM_REMOTE}/maistra-2.0 -- ${SOURCE_DIR}/resources/helm/v2.0
	git reset HEAD ${SOURCE_DIR}/resources/helm/v2.0

.PHONY: update-2.0-templates
update-2.0-templates: update-remote-maistra-2.0
	git checkout ${GIT_UPSTREAM_REMOTE}/maistra-2.0 -- ${SOURCE_DIR}/resources/smcp-templates/v2.0
	git reset HEAD ${SOURCE_DIR}/resources/smcp-templates/v2.0

.PHONY: collect-2.0-charts
collect-2.0-charts:
	mkdir -p ${HELM_OUT_DIR}
	cp -rf ${RESOURCES_DIR}/helm/v2.0 ${HELM_OUT_DIR}

.PHONY: collect-2.0-templates
collect-2.0-templates:
	mkdir -p ${TEMPLATES_OUT_DIR}/v2.0
	cp ${RESOURCES_DIR}/smcp-templates/v2.0/${BUILD_TYPE} ${TEMPLATES_OUT_DIR}/v2.0/default
	cp ${RESOURCES_DIR}/smcp-templates/v2.0/base ${TEMPLATES_OUT_DIR}/v2.0

################################################################################
# maistra v2.1
################################################################################
.PHONY: update-2.1-charts
update-2.1-charts:
	HELM_DIR=${RESOURCES_DIR}/helm/v2.1 ISTIO_VERSION=1.8.0 ${SOURCE_DIR}/build/download-charts.sh
	CRD_DIR=${RESOURCES_DIR}/helm/v2.1/istio-init/files ${SOURCE_DIR}/build/split-istio-crds.sh

.PHONY: collect-2.1-charts
collect-2.1-charts:
	mkdir -p ${HELM_OUT_DIR}
	cp -rf ${RESOURCES_DIR}/helm/v2.1 ${HELM_OUT_DIR}

.PHONY: collect-2.1-templates
collect-2.1-templates:
	mkdir -p ${TEMPLATES_OUT_DIR}/v2.1
	cp ${RESOURCES_DIR}/smcp-templates/v2.1/${BUILD_TYPE} ${TEMPLATES_OUT_DIR}/v2.1/default
	find ${RESOURCES_DIR}/smcp-templates/v2.1/ -maxdepth 1 -type f ! -name "maistra" ! -name "servicemesh" |xargs cp -t ${TEMPLATES_OUT_DIR}/v2.1


################################################################################
# OLM manifest generation
################################################################################
.PHONY: generate-community-manifests
generate-community-manifests:
	COMMUNITY=true REPLACES_CSV=${REPLACES_COMMUNITY_CSV} ${SOURCE_DIR}/build/generate-manifests.sh

.PHONY: generate-product-manifests
generate-product-manifests:
	COMMUNITY=false REPLACES_CSV=${REPLACES_PRODUCT_CSV} ${SOURCE_DIR}/build/generate-manifests.sh

################################################################################
# resource generation
################################################################################
.PHONY: gen
gen:  generate-crds update-charts update-templates update-generated-code generate-manifests generate-docs

.PHONY: gen-check
gen-check: gen restore-manifest-dates check-clean-repo

.PHONY: check-clean-repo
check-clean-repo:
	@if [[ -n $$(git status --porcelain) ]]; then git status; git diff; echo "ERROR: Some files need to be updated, please run 'make gen' and include any changed files in your PR"; exit 1;	fi

.PHONY: generate-manifests
generate-manifests: generate-community-manifests generate-product-manifests

.PHONY: generate-crds
generate-crds:
	${SOURCE_DIR}/build/generate-crds.sh

.PHONY: generate-docs
generate-docs:
	rm -rf ${SOURCE_DIR}/docs/crd
	go run -mod=vendor github.com/maistra/istio-operator/tools/doc/ paths=github.com/maistra/istio-operator/pkg/apis/maistra/... output:dir=${SOURCE_DIR}/docs/crd doc:format=adoc,depth=2

.PHONY: restore-manifest-dates
restore-manifest-dates:
ifneq "${MAISTRA_MANIFEST_DATE}" ""
	sed -i -e "s/\(createdAt:\).*/\1 ${MAISTRA_MANIFEST_DATE}/" manifests-maistra/${MAISTRA_VERSION}/maistraoperator.v${MAISTRA_VERSION}.clusterserviceversion.yaml
endif
ifneq "${OSSM_MANIFEST_DATE}" ""
	sed -i -e "s/\(createdAt:\).*/\1 ${OSSM_MANIFEST_DATE}/" manifests-servicemesh/${MAISTRA_VERSION}/servicemeshoperator.v${MAISTRA_VERSION}.clusterserviceversion.yaml
endif

.PHONY: update-charts
update-charts: update-1.0-charts update-1.1-charts update-2.0-charts update-2.1-charts

.PHONY: update-templates
update-templates: update-1.0-templates update-1.1-templates update-2.0-templates

################################################################################
# resource collection
################################################################################
.PHONY: collect-charts
collect-charts: collect-1.0-charts collect-1.1-charts collect-2.0-charts collect-2.1-charts

.PHONY: collect-templates
collect-templates: collect-1.0-templates collect-1.1-templates collect-2.0-templates collect-2.1-templates

.PHONY: collect-olm-manifests
collect-olm-manifests:
	rm -rf  ${OLM_MANIFEST_OUT_DIR}
	mkdir -p ${OLM_MANIFEST_OUT_DIR}
	cp -ra ${SOURCE_DIR}/manifests-${BUILD_TYPE}/* ${OLM_MANIFEST_OUT_DIR}

.PHONY: collect-resources
collect-resources: collect-templates collect-charts collect-olm-manifests

################################################################################
# update-generated-code target regenerates k8s api related code
################################################################################
.PHONY: update-generated-code
update-generated-code:
	${SOURCE_DIR}/build/codegen/update-generated.sh

################################################################################
# build target compiles and updates resources
################################################################################
.PHONY: build
build: update-generated-code update-charts update-templates compile

################################################################################
# create image
################################################################################
.PHONY: image
image: build collect-resources
	${CONTAINER_CLI} build --no-cache -t "${IMAGE}" -f ${SOURCE_DIR}/build/Dockerfile --build-arg build_type=${BUILD_TYPE} .

.DEFAULT_GOAL := build
