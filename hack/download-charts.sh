#!/usr/bin/env bash

set -e -u

: "${MAISTRA_RELEASE_STREAM:=$1}"
: "${ISTIO_REPO:=$2}"
: "${ISTIO_COMMIT:=$3}"

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPO_ROOT=$(dirname "${SCRIPT_DIR}")
HELM_DIR="${REPO_ROOT}/resources/charts/${MAISTRA_RELEASE_STREAM}"

ISTIO_FILE="${ISTIO_COMMIT}.zip"
ISTIO_URL="${ISTIO_REPO}/archive/${ISTIO_COMMIT}.zip"
WORK_DIR=$(mktemp -d)
EXTRACT_DIR="${ISTIO_REPO##*/}-${ISTIO_COMMIT}"
EXTRACT_CMD="unzip -q ${ISTIO_FILE} ${EXTRACT_DIR}/manifests/charts/* ${EXTRACT_DIR}/manifests/addons/dashboards/*"

function downloadIstioCharts() {
  rm -rf "${HELM_DIR}"
  mkdir -p "${HELM_DIR}"

  echo "downloading charts from ${ISTIO_URL}"
  cd "${WORK_DIR}"
  curl -LfO "${ISTIO_URL}"

  echo "extracting charts to ${WORK_DIR}/${EXTRACT_DIR}"
  ${EXTRACT_CMD}
  echo "copying charts to ${HELM_DIR}"
  cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/* "${HELM_DIR}/"
}

function patchIstioCharts() {
  # NOTE: everything in the patchIstioCharts should be here only temporarily,
  # until we push the required changes upstream
  sed -i '0,/rules:/s//rules:\
- apiGroups: ["security.openshift.io"] \
  resources: ["securitycontextconstraints"] \
  resourceNames: ["privileged"] \
  verbs: ["use"]/' "${HELM_DIR}/istio-cni/templates/clusterrole.yaml"
}

downloadIstioCharts
patchIstioCharts