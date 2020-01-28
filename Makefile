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

MAISTRA_VERSION ?= 1.1.0
MAISTRA_BRANCH  ?= maistra-1.1
VERSION         ?= development
IMAGE           ?= docker.io/maistra/istio-ubi8-operator:${MAISTRA_VERSION}
CONTAINER_CLI   ?= docker
COMMUNITY       ?= true

SOURCE_DIR        := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
RESOURCES_DIR     = ${SOURCE_DIR}/resources
OUT_DIR           = ${SOURCE_DIR}/tmp/_output
TEMPLATES_OUT_DIR = ${OUT_DIR}/resources/default-templates
HELM_OUT_DIR      = ${OUT_DIR}/resources/helm

OFFLINE_BUILD       ?= false
GIT_UPSTREAM_REMOTE ?= $(shell git remote -v |grep --color=never ':Maistra/istio-operator\.git.*(fetch)' |grep --color=never -o '^[^[:space:]]*')

ifeq "${GIT_UPSTREAM_REMOTE}" ""
$(warning Could not find git remote for Maistra/istio-operator)
endif

ifeq "${COMMUNITY}" "true"
BUILD_TYPE = maistra
else
BUILD_TYPE = servicemesh
endif

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
	go test -mod=vendor ./...

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

# XXX: for now, the templates for maistra-1.0 are stored in ./deploy/smcp-templates, so the following won't work
#.PHONY: update-1.0-templates
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
.PHONY: update-1.1-charts
update-1.1-charts:
	HELM_DIR=${RESOURCES_DIR}/helm/v1.1 ISTIO_VERSION=1.1.0 ${SOURCE_DIR}/build/download-charts.sh

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
# OLM manifest generation
################################################################################
.PHONY: generate-community-manifests
generate-community-manifests: 
	COMMUNITY=true ${SOURCE_DIR}/build/generate-manifests.sh

.PHONY: generate-product-manifests
generate-product-manifests: 
	COMMUNITY=false ${SOURCE_DIR}/build/generate-manifests.sh

################################################################################
# resource generation
################################################################################
.PHONY: generate-manifests
generate-manifests: generate-community-manifests generate-product-manifests

.PHONY: update-charts
update-charts: update-1.0-charts update-1.1-charts

################################################################################
# resource collection
################################################################################
.PHONY: collect-charts
collect-charts: collect-1.0-charts collect-1.1-charts

.PHONY: collect-templates
collect-templates: collect-1.0-templates collect-1.1-templates

.PHONY: collect-resources
collect-resources: collect-templates collect-charts

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
build: update-generated-code update-charts compile

################################################################################
# create image
################################################################################
.PHONY: image
image: build collect-resources
	${CONTAINER_CLI} build --no-cache -t "${IMAGE}" -f ${SOURCE_DIR}/build/Dockerfile --build-arg build_type=${BUILD_TYPE} .

.DEFAULT_GOAL := build
