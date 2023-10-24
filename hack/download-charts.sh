#!/usr/bin/env bash

set -e -u -o pipefail

: "${ISTIO_VERSION:=$1}"
: "${ISTIO_REPO:=$2}"
: "${ISTIO_COMMIT:=$3}"
CHART_URLS=("${@:4}")

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPO_ROOT=$(dirname "${SCRIPT_DIR}")
MANIFEST_DIR="${REPO_ROOT}/resources/${ISTIO_VERSION}"
CHARTS_DIR="${MANIFEST_DIR}/charts"
PROFILES_DIR="${MANIFEST_DIR}/profiles"

ISTIO_URL="${ISTIO_REPO}/archive/${ISTIO_COMMIT}.tar.gz"
WORK_DIR=$(mktemp -d)

function downloadIstioManifests() {
  rm -rf "${CHARTS_DIR}"
  mkdir -p "${CHARTS_DIR}"

  rm -rf "${PROFILES_DIR}"
  mkdir -p "${PROFILES_DIR}"

  pushd "${WORK_DIR}"
  echo "downloading Git archive from ${ISTIO_URL}"
  curl -LfO "${ISTIO_URL}"

  ISTIO_FILE="${ISTIO_URL##*/}"
  EXTRACT_DIR="${ISTIO_REPO##*/}-${ISTIO_COMMIT}"

  if [ "${#CHART_URLS[@]}" -gt 0 ]; then
    for url in "${CHART_URLS[@]}"; do
      echo "downloading chart from $url"
      curl -LfO "$url"

      file="${url##*/}"

      echo "extracting charts from $file to ${CHARTS_DIR}"
      tar zxf "$file" -C "${CHARTS_DIR}"
    done

    echo "extracting profiles from ${ISTIO_FILE} to ${PROFILES_DIR}"
    tar zxf "${ISTIO_FILE}" "${EXTRACT_DIR}/manifests/profiles"
    echo "copying profiles to ${PROFILES_DIR}"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/profiles/* "${PROFILES_DIR}/"

  else
    echo "extracting charts and profiles from ${ISTIO_FILE} to ${WORK_DIR}/${EXTRACT_DIR}"
    tar zxf "${ISTIO_FILE}" "${EXTRACT_DIR}/manifests/charts" "${EXTRACT_DIR}/manifests/profiles"

    echo "copying charts to ${CHARTS_DIR}"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/base "${CHARTS_DIR}/base"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/gateway "${CHARTS_DIR}/gateway"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/istio-cni "${CHARTS_DIR}/cni"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/istio-control/istio-discovery "${CHARTS_DIR}/istiod"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/ztunnel "${CHARTS_DIR}/ztunnel"

    echo "copying profiles to ${PROFILES_DIR}"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/profiles/* "${PROFILES_DIR}/"
  fi

  popd
}

function patchIstioCharts() {
  # NOTE: everything in the patchIstioCharts should be here only temporarily,
  # until we push the required changes upstream
  sed -i '0,/rules:/s//rules:\
- apiGroups: ["security.openshift.io"] \
  resources: ["securitycontextconstraints"] \
  resourceNames: ["privileged"] \
  verbs: ["use"]/' "${CHARTS_DIR}/cni/templates/clusterrole.yaml"
}

function convertIstioProfiles() {
  for profile in "${PROFILES_DIR}"/*.yaml; do
    yq eval -i '.apiVersion="operator.istio.io/v1alpha1"
      | .kind="Istio"
      | del(.metadata)
      | del(.spec.components)
      | del(.spec.meshConfig)
      | del(.spec.hub)
      | del(.spec.tag)' "$profile"
  done
}

downloadIstioManifests
patchIstioCharts
convertIstioProfiles