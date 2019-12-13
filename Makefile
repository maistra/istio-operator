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

ifeq "${COMMUNITY}" "true"
BUILD_TYPE = maistra
else
BUILD_TYPE = servicemesh
endif

export SOURCE_DIR OUT_DIR MAISTRA_BRANCH MAISTRA_VERSION VERSION COMMUNITY BUILD_TYPE

.PHONY: clean
clean:
	rm -rf "${OUT_DIR}"

.PHONY: compile
compile:
	${SOURCE_DIR}/build/build.sh

.PHONY: collect-charts
collect-charts: collect-1.1-charts

.PHONY: generate-charts
generate-charts: 
	HELM_DIR=${RESOURCES_DIR}/helm/v1.1 ISTIO_VERSION=1.1.0 ${SOURCE_DIR}/build/download-charts.sh

.PHONY: generate-community-manifests
generate-community-manifests: 
	COMMUNITY=true ${SOURCE_DIR}/build/generate-manifests.sh

.PHONY: generate-product-manifests
generate-product-manifests: 
	COMMUNITY=false ${SOURCE_DIR}/build/generate-manifests.sh

.PHONY: generate-manifests
generate-manifests: generate-community-manifests generate-product-manifests

.PHONY: collect-1.1-charts
collect-1.1-charts:
	mkdir -p ${HELM_OUT_DIR}
	cp -rf ${RESOURCES_DIR}/helm/v1.1 ${HELM_OUT_DIR}

.PHONY: collect-templates
collect-templates: collect-1.1-templates

.PHONY: collect-1.1-templates
collect-1.1-templates:
	mkdir -p ${TEMPLATES_OUT_DIR}/v1.1
	cp ${RESOURCES_DIR}/smcp-templates/v1.1/${BUILD_TYPE} ${TEMPLATES_OUT_DIR}/v1.1/default
	cp ${RESOURCES_DIR}/smcp-templates/v1.1/base ${TEMPLATES_OUT_DIR}/v1.1

.PHONY: collect-resources
collect-resources: collect-templates collect-charts

################################################################################
# update-generated-code target regenerates k8s api related code
################################################################################
.PHONY: update-generated-code
update-generated-code:
	${SOURCE_DIR}/build/codegen/update-generated.sh

.PHONY: build
build: compile update-generated-code generate-charts

.PHONY: image
image: build collect-resources
	${CONTAINER_CLI} build --no-cache -t "${IMAGE}" -f ${SOURCE_DIR}/build/Dockerfile --build-arg build_type=${BUILD_TYPE} .

.DEFAULT_GOAL := build
