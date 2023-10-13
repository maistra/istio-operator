#!/usr/bin/env bash

set -eu -o pipefail

# include sed_wrap
# shellcheck source=build/sed-wrapper.sh
source "$(dirname "${BASH_SOURCE[0]}")/sed-wrapper.sh"

: "${MAISTRA_VERSION:=2.4.3}"
: "${MAISTRA_REPO:=https://github.com/maistra/istio}"
: "${MAISTRA_BRANCH:=maistra-2.4}"

: "${SOURCE_DIR:=$(pwd)}"
: "${OUT_DIR:=${SOURCE_DIR}/tmp/_output}"

: "${ISTIO_VERSION:=1.16.5}"

RELEASES_DIR=${OUT_DIR}/helm/istio-releases

# shellcheck disable=SC2034
PLATFORM=linux

ISTIO_NAME=istio-${ISTIO_VERSION}
MAISTRA_BRANCH_WITHOUT_SLASH="${MAISTRA_BRANCH/\//-}"
ISTIO_FILE="${MAISTRA_BRANCH_WITHOUT_SLASH}.zip"
ISTIO_URL="${MAISTRA_REPO}/archive/${MAISTRA_BRANCH}.zip"
EXTRACT_DIR="${MAISTRA_REPO##*/}-${MAISTRA_BRANCH_WITHOUT_SLASH}"
EXTRACT_CMD="unzip ${ISTIO_FILE} ${EXTRACT_DIR}/manifests/charts/* ${EXTRACT_DIR}/manifests/addons/dashboards/*"
RELEASE_DIR="${RELEASES_DIR}/${ISTIO_NAME}"

ISTIO_NAME=${ISTIO_NAME//./-}

: "${HELM_DIR:=${RELEASE_DIR}}"

function retrieveIstioRelease() {
  if [ -d "${HELM_DIR}" ] ; then
    rm -rf "${HELM_DIR}"
  fi
  mkdir -p "${HELM_DIR}"

  if [ ! -f "${RELEASES_DIR}/${ISTIO_FILE}" ] ; then
    (
      if [ ! -f "${RELEASES_DIR}" ] ; then
        mkdir -p "${RELEASES_DIR}"
      fi
      echo "downloading Istio Release: ${ISTIO_URL}"
      cd "${RELEASES_DIR}"
      curl -Lf -o "${ISTIO_FILE}" "${ISTIO_URL}"
    )
  fi

  (
      echo "extracting Istio Helm charts to ${RELEASES_DIR}"
      cd "${RELEASES_DIR}"
      rm -rf ${EXTRACT_DIR}
      ${EXTRACT_CMD}
      cp -rf ${EXTRACT_DIR}/manifests/charts/* "${HELM_DIR}/"
      # grafana dashboards
      mkdir -p "${HELM_DIR}/istio-telemetry/grafana/dashboards"
      cp -rf ${EXTRACT_DIR}/manifests/addons/dashboards/* "${HELM_DIR}/istio-telemetry/grafana/dashboards/"
      #(
      #  cd "${HELM_DIR}/istio"
      #  helm dep update
      #)
  )
}

retrieveIstioRelease

# shellcheck source=build/patch-charts.sh
source "$(dirname "${BASH_SOURCE[0]}")/patch-charts.sh"

(
  cd "${RELEASES_DIR}"
  echo "producing diff file for charts: $(pwd)/chart-diffs.diff"
  diff -uNr "${EXTRACT_DIR}/manifests/charts/" "${HELM_DIR}/" > chart-diffs.diff || [ $? -eq 1 ]
#  cp -r ${EXTRACT_DIR}/manifests/charts/ ${HELM_DIR}-original/
  echo "Location of patched charts: ${HELM_DIR}/"
)
