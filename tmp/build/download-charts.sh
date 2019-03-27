#!/usr/bin/env bash

set -e

: ${MAISTRA_VERSION:=0.10.0}

SOURCE_DIR=$(pwd)
DIR=$(pwd)/tmp/_output/helm

ISTIO_VERSION=1.1.0
#ISTIO_BRANCH=release-1.1

RELEASES_DIR=${DIR}/istio-releases

PLATFORM=linux
if [ -n "${ISTIO_VERSION}" ] ; then
  ISTIO_NAME=istio-${ISTIO_VERSION}
  ISTIO_FILE="${ISTIO_NAME}-${PLATFORM}.tar.gz"
  ISTIO_URL="https://github.com/istio/istio/releases/download/${ISTIO_VERSION}/${ISTIO_FILE}"
  EXTRACT_CMD="tar --strip-components=4 -C ./${ISTIO_NAME} -xvzf ${ISTIO_FILE} ${ISTIO_NAME}/install/kubernetes/helm"
  RELEASE_DIR="${RELEASES_DIR}/${ISTIO_NAME}"
else
  ISTIO_NAME=istio-${ISTIO_BRANCH}
  ISTIO_FILE="${ISTIO_BRANCH}.zip"
  ISTIO_URL="https://github.com/istio/istio/archive/${ISTIO_FILE}"
  EXTRACT_CMD="unzip ${ISTIO_FILE} ${ISTIO_NAME}/install/kubernetes/helm"
  RELEASE_DIR="${RELEASES_DIR}/${ISTIO_NAME}"
fi

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
      ${EXTRACT_CMD}
      #(
      #  cd "${HELM_DIR}/istio"
      #  helm dep update
      #)
  )
}

retrieveIstioRelease

source $(dirname ${BASH_SOURCE})/patch-charts.sh