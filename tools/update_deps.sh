#!/bin/bash

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

set -exo pipefail

UPDATE_BRANCH=${UPDATE_BRANCH:-"master"}

SCRIPTPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR=$(dirname "${SCRIPTPATH}")
cd "${ROOTDIR}"

# getLatestVersion gets the latest released version of a github project
# $1 = org/repo
function getLatestVersion() {
  curl -sL "https://api.github.com/repos/${1}/releases/latest" | yq '.tag_name'
}

# Update common files
make update-common

# Update go dependencies (skipped until cachito supports 1.22, see https://issues.redhat.com/browse/OSSM-5988)
export GO111MODULE=on
# go get -u "istio.io/istio@${UPDATE_BRANCH}"
# go get -u "istio.io/client-go@${UPDATE_BRANCH}"
go mod tidy

# Update operator-sdk
OPERATOR_SDK_LATEST_VERSION=$(getLatestVersion operator-framework/operator-sdk)
sed -i "s|OPERATOR_SDK_VERSION ?= .*|OPERATOR_SDK_VERSION ?= ${OPERATOR_SDK_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"
find "${ROOTDIR}/chart/templates/olm/scorecard.yaml" -type f -exec sed -i "s|quay.io/operator-framework/scorecard-test:.*|quay.io/operator-framework/scorecard-test:${OPERATOR_SDK_LATEST_VERSION}|" {} +

# Update helm
HELM_LATEST_VERSION=$(getLatestVersion helm/helm | cut -d/ -f2)
sed -i "s|HELM_VERSION ?= .*|HELM_VERSION ?= ${HELM_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update controller-tools
CONTROLLER_TOOLS_LATEST_VERSION=$(getLatestVersion kubernetes-sigs/controller-tools)
sed -i "s|CONTROLLER_TOOLS_VERSION ?= .*|CONTROLLER_TOOLS_VERSION ?= ${CONTROLLER_TOOLS_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update opm
OPM_LATEST_VERSION=$(getLatestVersion operator-framework/operator-registry)
sed -i "s|OPM_VERSION ?= .*|OPM_VERSION ?= ${OPM_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update kube-rbac-proxy
RBAC_PROXY_LATEST_VERSION=$(getLatestVersion brancz/kube-rbac-proxy | cut -d/ -f1)
# Only update it if the newer image is available in the registry
if docker manifest inspect "gcr.io/kubebuilder/kube-rbac-proxy:${RBAC_PROXY_LATEST_VERSION}" >/dev/null 2>/dev/null; then
  sed -i "s|gcr.io/kubebuilder/kube-rbac-proxy:.*|gcr.io/kubebuilder/kube-rbac-proxy:${RBAC_PROXY_LATEST_VERSION}|" "${ROOTDIR}/chart/templates/deployment.yaml"
fi

# Regenerate files
make update-istio gen
