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

default: build

MAIN_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

GOBINARY ?= go
OUT_DIR ?= ${MAIN_DIR}/_output

MAISTRA_VERSION ?= 1.0.0
MAISTRA_BRANCH  ?= maistra-1.0
COMMUNITY       ?= true

export MAIN_DIR OUT_DIR MAISTRA_BRANCH MAISTRA_VERSION COMMUNITY

PACKAGE_NAME := github.com/maistra/istio-operator
LD_EXTRAFLAGS =

VERSION ?= development
LD_EXTRAFLAGS += -X ${PACKAGE_NAME}/pkg/version.buildVersion=${VERSION}

GITREVISION ?= $(shell git rev-parse --verify HEAD 2> /dev/null)
ifeq ($(GITREVISION),)
  GITREVISION = unknown
endif
LD_EXTRAFLAGS += -X ${PACKAGE_NAME}/pkg/version.buildGitRevision=${GITREVISION}

GITSTATUS ?= $(shell git diff-index --quiet HEAD --  2> /dev/null; echo $$?)
ifeq ($(GITSTATUS),0)
  GITSTATUS = Clean
else ifeq ($(GITSTATUS),1)
  GITSTATUS = Modified
else
  GITSTATUS = unknown
endif
LD_EXTRAFLAGS += -X ${PACKAGE_NAME}/pkg/version.buildStatus=${GITSTATUS}

GITTAG ?= $(shell git describe 2> /dev/null)
ifeq ($(GITTAG),)
  GITTAG = unknown
endif
LD_EXTRAFLAGS += -X ${PACKAGE_NAME}/pkg/version.buildTag=${GITTAG}

LDFLAGS = '-extldflags -static ${LD_EXTRAFLAGS}'

.PHONY: clean
clean:
	rm -rf "${OUT_DIR}"

.PHONY: build
build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 ${GOBINARY} build -o "${OUT_DIR}/bin/istio-operator" -ldflags ${LDFLAGS} ./cmd/...

patch-charts:
	${MAIN_DIR}/build/patch-charts.sh

dev/download-charts:
	${MAIN_DIR}/build/download-charts.sh

# Not calling it 'image' to make it clear that this image is for developer purposes only
dev/image: build dev/download-charts
	${MAIN_DIR}/build/build-image.sh
