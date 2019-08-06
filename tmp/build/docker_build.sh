#!/usr/bin/env bash

set -e

TEMPLATES_DIR="$(pwd)/tmp/_output/templates"
COMMUNITY=${COMMUNITY:-true}
SOURCE_DIR=$(pwd)

# Allow the developer to use other tool, e.g. podman
CONTAINER_CLI=${CONTAINER_CLI:-docker}

if ! which ${CONTAINER_CLI} > /dev/null; then
	echo "${CONTAINER_CLI} needs to be installed"
	exit 1
fi

: ${IMAGE:?"Need to set IMAGE, e.g. gcr.io/<repo>/<your>-operator"}

BUILD_DIR="$(dirname ${BASH_SOURCE[0]})"

echo "building istio-operator exe"
${BUILD_DIR}/build.sh

echo "collecting helm charts"
${BUILD_DIR}/download-charts.sh

mkdir -p ${TEMPLATES_DIR}
if [[ "${COMMUNITY,,}" == "true" ]]; then
	cp ${SOURCE_DIR}/deploy/smcp-templates/maistra ${TEMPLATES_DIR}/default
else
	cp ${SOURCE_DIR}/deploy/smcp-templates/servicemesh ${TEMPLATES_DIR}/default
fi
cp ${SOURCE_DIR}/deploy/smcp-templates/base ${TEMPLATES_DIR}

echo "building container ${IMAGE}..."
${CONTAINER_CLI} build --no-cache -t "${IMAGE}" -f tmp/build/Dockerfile .
