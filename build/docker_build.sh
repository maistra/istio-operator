#!/usr/bin/env bash

set -e -u

: "${IMAGE:?"Need to set IMAGE, e.g. gcr.io/<repo>/<your>-operator"}"

COMMUNITY=${COMMUNITY:-true}
[ "${COMMUNITY}" = "true" ] && BUILD_TYPE="maistra" || BUILD_TYPE="servicemesh"

SOURCE_DIR=$(pwd)
BUILD_DIR="$(dirname "${BASH_SOURCE[0]}")"

RESOURCES_DIR="$(pwd)/tmp/_output/resources"
rm -rf "${RESOURCES_DIR}" && mkdir -p "${RESOURCES_DIR}"

# Allow the developer to use other tool, e.g. podman
CONTAINER_CLI=${CONTAINER_CLI:-docker}
if ! which "${CONTAINER_CLI}" > /dev/null; then
	echo "${CONTAINER_CLI} needs to be installed"
	exit 1
fi

echo "building istio-operator exe"
"${BUILD_DIR}/build.sh"

echo "collecting helm charts"
"${BUILD_DIR}/download-charts.sh"

TEMPLATES_DIR="${RESOURCES_DIR}/default-templates"
mkdir "${TEMPLATES_DIR}"
cp "${SOURCE_DIR}/deploy/smcp-templates/${BUILD_TYPE}" "${TEMPLATES_DIR}/default"
cp "${SOURCE_DIR}/deploy/smcp-templates/base" "${TEMPLATES_DIR}"

HELM_DIR="${RESOURCES_DIR}/helm/"
mkdir "${HELM_DIR}"
cp -r "${SOURCE_DIR}/tmp/_output/helm/istio-releases/istio-1.1.0" "${HELM_DIR}/1.1.0"

echo "building container ${IMAGE}..."
${CONTAINER_CLI} build --no-cache -t "${IMAGE}" -f tmp/build/Dockerfile --build-arg build_type=${BUILD_TYPE} .
