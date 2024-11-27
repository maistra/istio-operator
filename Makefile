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

# Determine this makefile's path.
# Be sure to place this BEFORE `include` directives, if any.
THIS_FILE := $(lastword $(MAKEFILE_LIST))

-include Makefile.overrides

MAISTRA_VERSION        ?= 2.4.13
MAISTRA_BRANCH         ?= maistra-2.4
ISTIO_VERSION          ?= 1.16.7
REPLACES_PRODUCT_CSV   ?= 2.4.12
REPLACES_COMMUNITY_CSV ?= 2.4.12
VERSION                ?= development
CONTAINER_CLI          ?= docker
COMMUNITY              ?= true
TEST_TIMEOUT           ?= 10m
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

FINDFILES=find . \( -path ./.git -o -path ./.github -o -path ./tmp \) -prune -o -type f
XARGS = xargs -0 -r


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
	go test -timeout ${TEST_TIMEOUT} ${TEST_FLAGS} ./...

################################################################################
# Helm charts generation and templates processing
################################################################################

SUPPORTED_VERSIONS := 2.1 2.2 2.3 2.4

$(addprefix update-remote-maistra-,$(SUPPORTED_VERSIONS)): update-remote-maistra-%:
	$(eval version:=$*)
	@ if [[ ${OFFLINE_BUILD} == "false" && ${MAISTRA_VERSION} != ${version}.* ]]; \
	then \
		git remote set-branches --add ${GIT_UPSTREAM_REMOTE} maistra-${version}; \
		git fetch ${GIT_UPSTREAM_REMOTE} maistra-${version}:maistra-${version}; \
	fi

$(addprefix update-charts-,$(SUPPORTED_VERSIONS)): update-charts-%:
	$(eval version:=$*)
	@# If we are calling make against previous version,
	@# sync from previous branches and explicitly call dependent target with extracted version.
	@# Otherwise only download charts.
	@ if [[ ${MAISTRA_VERSION} != ${version}.* ]]; \
	then \
		$(MAKE) -f $(THIS_FILE) update-remote-maistra-${version}; \
		git checkout ${GIT_UPSTREAM_REMOTE}/maistra-${version} -- ${SOURCE_DIR}/resources/helm/v${version}; \
		git reset HEAD ${SOURCE_DIR}/resources/helm/v${version}; \
	else \
		HELM_DIR=${RESOURCES_DIR}/helm/v${version} ISTIO_VERSION=${ISTIO_VERSION} ${SOURCE_DIR}/build/download-charts.sh; \
	fi

$(addprefix update-templates-,$(SUPPORTED_VERSIONS)): update-templates-%: update-remote-maistra-%
	$(eval version:=$*)
	@ if [[ ${MAISTRA_VERSION} != ${version}.* ]]; \
	then \
		git checkout ${GIT_UPSTREAM_REMOTE}/maistra-${version} -- ${SOURCE_DIR}/resources/smcp-templates/v${version}; \
		git reset HEAD ${SOURCE_DIR}/resources/smcp-templates/v${version}; \
	fi

$(addprefix collect-charts-,$(SUPPORTED_VERSIONS)): collect-charts-%:
	$(eval version:=$*)
	mkdir -p ${HELM_OUT_DIR}
	cp -rf ${RESOURCES_DIR}/helm/v${version} ${HELM_OUT_DIR}

$(addprefix collect-templates-,$(SUPPORTED_VERSIONS)): collect-templates-%:
	$(eval version:=$*)
	mkdir -p ${TEMPLATES_OUT_DIR}/v${version}
	cp ${RESOURCES_DIR}/smcp-templates/v${version}/${BUILD_TYPE} ${TEMPLATES_OUT_DIR}/v${version}/default
	find ${RESOURCES_DIR}/smcp-templates/v${version}/ -maxdepth 1 -type f ! -name "maistra" ! -name "servicemesh" -exec cp {} ${TEMPLATES_OUT_DIR}/v${version}/ \;

.PHONY: update-charts
update-charts: $(addprefix update-charts-,$(SUPPORTED_VERSIONS))

.PHONY: update-templates
update-templates: $(addprefix update-templates-,$(SUPPORTED_VERSIONS))

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
gen:  generate-crds update-charts update-templates update-generated-code generate-manifests

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

.PHONY: restore-manifest-dates
restore-manifest-dates:
ifneq "${MAISTRA_MANIFEST_DATE}" ""
	sed -i -e "s/\(createdAt:\).*/\1 ${MAISTRA_MANIFEST_DATE}/" manifests-maistra/${MAISTRA_VERSION}/maistraoperator.v${MAISTRA_VERSION}.clusterserviceversion.yaml
endif
ifneq "${OSSM_MANIFEST_DATE}" ""
	sed -i -e "s/\(createdAt:\).*/\1 ${OSSM_MANIFEST_DATE}/" manifests-servicemesh/${MAISTRA_VERSION}/servicemeshoperator.v${MAISTRA_VERSION}.clusterserviceversion.yaml
endif


################################################################################
# resource collection
################################################################################
.PHONY: collect-charts
collect-charts: collect-charts-2.1 collect-charts-2.2 collect-charts-2.3 collect-charts-2.4

.PHONY: collect-templates
collect-templates: collect-templates-2.1 collect-templates-2.2 collect-templates-2.3 collect-templates-2.4

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
# linting
################################################################################
.PHONY: lint-scripts
lint-scripts:
	@${FINDFILES} -name '*.sh' -print0 | ${XARGS} shellcheck

.PHONY: lint-go
lint-go:
	@${FINDFILES} -name '*.go' \( ! \( -name '*.gen.go' -o -name '*.pb.go' -o -name 'zz_generated.*.go' \) \) -print0 | ${XARGS} build/lint_go.sh

.PHONY: lint-yaml
lint-yaml:
	@${FINDFILES} \( -name '*.yml' -o -name '*.yaml' \) -not \( -wholename './build/manifest-templates/clusterserviceversion.yaml' \) -not -exec grep -q -e "{{" {} \; -print0 | ${XARGS} yamllint -c build/.yamllint.yml -f parsable

.PHONY: lint-helm
lint-helm:
	@echo "Helm version: `helm version`"
	@${FINDFILES} -name 'Chart.yaml' -path './resources/helm/v2.?/*' \
	-not \(	-path './resources/helm/v2.0/*' -o -path './resources/helm/v2.1/*' -o -path './resources/helm/v2.2/*' \) \
	-print0 | ${XARGS} -L 1 dirname | xargs -r helm lint --strict

.PHONY: lint
lint: lint-scripts lint-go lint-yaml lint-helm

################################################################################
# create & push image
################################################################################
.PHONY: check-image-var image push
check-image-var:
	@if [ -z "${IMAGE}" ]; then echo "Please set the IMAGE variable" && exit 1; fi

image: check-image-var build collect-resources
	${CONTAINER_CLI} build --no-cache -t "${IMAGE}" -f ${SOURCE_DIR}/build/Dockerfile --build-arg build_type=${BUILD_TYPE} .

push: image
	${CONTAINER_CLI} push "${IMAGE}"

.DEFAULT_GOAL := build

################################################################################
# run an integration test in KinD
################################################################################
.PHONY: test.integration.kind
test.integration.kind:
	${SOURCE_DIR}/tests/integration/operator-integ-suite-kind.sh

