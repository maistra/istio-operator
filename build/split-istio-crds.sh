#!/usr/bin/env bash

set -e -u

: "${CRD_DIR:?"Need to set CRD_DIR to location of CRD yaml files, e.g. resources/helm/v2.0/istio-init/files"}"
: "${SOURCE_DIR:=$(pwd)}"

CRD_DIR=$(realpath --relative-to "${SOURCE_DIR}" "${CRD_DIR}")
(
  cd "${SOURCE_DIR}"
  find "${CRD_DIR}" -maxdepth 1 -name "*.yaml" -type f ! -name "*crd.yaml" -exec go run -mod=vendor ./tools/crd --zap-encoder=console {} +
  # delete the original files
  find "${CRD_DIR}" -maxdepth 1 ! -name "*crd.yaml" -a -type f -delete
)