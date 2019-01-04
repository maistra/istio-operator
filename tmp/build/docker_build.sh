#!/usr/bin/env bash

set -e

if ! which docker > /dev/null; then
	echo "docker needs to be installed"
	exit 1
fi

: ${IMAGE:?"Need to set IMAGE, e.g. gcr.io/<repo>/<your>-operator"}

BUILD_DIR="$(dirname ${BASH_SOURCE[0]})"

echo "building istio-operator exe"
${BUILD_DIR}/build.sh

echo "collecting helm charts"
${BUILD_DIR}/download-charts.sh

echo "building container ${IMAGE}..."
docker build --no-cache -t "${IMAGE}" -f tmp/build/Dockerfile .
