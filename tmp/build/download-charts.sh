#!/usr/bin/env bash

set -e

: ${MAISTRA_VERSION:=0.12.0}
: ${MAISTRA_BRANCH:=0.12}

SOURCE_DIR=$(pwd)
DIR=$(pwd)/tmp/_output/helm

ISTIO_VERSION=1.1.0
#ISTIO_BRANCH=release-1.1

RELEASES_DIR=${DIR}/istio-releases

PLATFORM=linux

ISTIO_NAME=istio-${ISTIO_VERSION}
ISTIO_FILE="maistra-${MAISTRA_BRANCH}.zip"
ISTIO_URL="https://github.com/Maistra/istio/archive/maistra-${MAISTRA_BRANCH}.zip"
EXTRACT_CMD="unzip ${ISTIO_FILE} istio-maistra-${MAISTRA_BRANCH}/install/kubernetes/helm/*"
RELEASE_DIR="${RELEASES_DIR}/${ISTIO_NAME}"

ISTIO_NAME=${ISTIO_NAME//./-}

HELM_DIR=${RELEASE_DIR}

if [[ "${ISTIO_VERSION}" =~ ^1\.0\..* ]]; then
  PATCH_1_0="true"
fi

COMMUNITY=${COMMUNITY:-true}

function retrieveIstioRelease() {
  if [ -d "${RELEASE_DIR}" ] ; then
    rm -rf "${RELEASE_DIR}"
  fi
  mkdir -p "${RELEASE_DIR}"

  if [ ! -f "${RELEASES_DIR}/${ISTIO_FILE}" ] ; then
    (
      echo "downloading Istio Release: ${ISTIO_URL}"
      cd "${RELEASES_DIR}"
      curl -LO "${ISTIO_URL}"
    )
  fi

  (
      echo "extracting Istio Helm charts to ${RELEASES_DIR}"
      cd "${RELEASES_DIR}"
      rm -rf istio-maistra-${MAISTRA_BRANCH}
      ${EXTRACT_CMD}
      cp -rf istio-maistra-${MAISTRA_BRANCH}/install/kubernetes/helm/* ${RELEASE_DIR}/
      #(
      #  cd "${HELM_DIR}/istio"
      #  helm dep update
      #)
  )
}

retrieveIstioRelease

source $(dirname ${BASH_SOURCE})/patch-charts.sh

(
  cd "${RELEASES_DIR}"
  echo "producing diff file for charts: $(pwd)/chart-diffs.diff"
  diff -uNr istio-maistra-${MAISTRA_BRANCH}/install/kubernetes/helm/ ${RELEASE_DIR}/ > chart-diffs.diff || [ $? -eq 1 ]
)
