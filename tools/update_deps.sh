#!/bin/bash

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

# Update go dependencies
export GO111MODULE=on
go get -u "istio.io/istio@${UPDATE_BRANCH}"
go get -u "istio.io/client-go@${UPDATE_BRANCH}"
go mod tidy

# Update operator-sdk
OPERATOR_SDK_LATEST_VERSION=$(getLatestVersion operator-framework/operator-sdk)
sed -i "s|OPERATOR_SDK_VERSION ?= .*|OPERATOR_SDK_VERSION ?= ${OPERATOR_SDK_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update kustomize
KUSTOMIZE_LATEST_VERSION=$(getLatestVersion kubernetes-sigs/kustomize | cut -d/ -f2)
sed -i "s|KUSTOMIZE_VERSION ?= .*|KUSTOMIZE_VERSION ?= ${KUSTOMIZE_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update controller-tools
CONTROLLER_TOOLS_LATEST_VERSION=$(getLatestVersion kubernetes-sigs/controller-tools)
sed -i "s|CONTROLLER_TOOLS_VERSION ?= .*|CONTROLLER_TOOLS_VERSION ?= ${CONTROLLER_TOOLS_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update opm
OPM_LATEST_VERSION=$(getLatestVersion operator-framework/operator-registry)
sed -i "s|OPM_VERSION ?= .*|OPM_VERSION ?= ${OPM_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update kube-rbac-proxy
RBAC_PROXY_LATEST_VERSION=$(getLatestVersion brancz/kube-rbac-proxy | cut -d/ -f1)
sed -i "s|gcr.io/kubebuilder/kube-rbac-proxy:.*|gcr.io/kubebuilder/kube-rbac-proxy:${RBAC_PROXY_LATEST_VERSION}|" "${ROOTDIR}/config/default/manager_auth_proxy_patch.yaml"

# Regenerate files
make gen
