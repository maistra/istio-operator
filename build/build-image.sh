#!/usr/bin/env bash

set -e

# Allow the developer to use other tool, e.g. podman
CONTAINER_CLI=${CONTAINER_CLI:-docker}

if ! which "${CONTAINER_CLI}" > /dev/null; then
	echo "${CONTAINER_CLI} needs to be installed"
	exit 1
fi

: ${IMAGE:?"Need to set IMAGE, e.g. quay.io/<your-user>/istio-operator:dev"}

IMAGE_DIR="${OUT_DIR}/image"

rm -rf "${IMAGE_DIR}"
mkdir -p "${IMAGE_DIR}/bin"

cp "${MAIN_DIR}/build/Dockerfile" "${IMAGE_DIR}"
cp "${OUT_DIR}/bin/istio-operator" "${IMAGE_DIR}/bin"
cp -r "${MAIN_DIR}/manifests" "${IMAGE_DIR}"
cp -r "${OUT_DIR}/helm/${MAISTRA_BRANCH}-patched" "${IMAGE_DIR}/helm"

echo "Building container image ${IMAGE}..."
"${CONTAINER_CLI}" build --no-cache -t "${IMAGE}" "${IMAGE_DIR}"
