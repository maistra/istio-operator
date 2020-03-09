#!/usr/bin/env bash

set -e

: ${MAISTRA_VERSION:=1.1.0}
: ${MAISTRA_REPO:=https://github.com/Maistra/istio}
: ${MAISTRA_BRANCH:=maistra-1.1}

: ${SOURCE_DIR:=$(pwd)}
: ${OUT_DIR:=${SOURCE_DIR}/tmp/_output}

: ${ISTIO_VERSION:=1.4}
#ISTIO_BRANCH=release-1.1

RELEASES_DIR=${OUT_DIR}/helm/istio-releases

PLATFORM=linux

ISTIO_NAME=istio-${ISTIO_VERSION}
ISTIO_FILE="${MAISTRA_BRANCH}.zip"
ISTIO_URL="${MAISTRA_REPO}/archive/${MAISTRA_BRANCH}.zip"
EXTRACT_DIR="${MAISTRA_REPO##*/}-${MAISTRA_BRANCH}"
EXTRACT_CMD="unzip ${ISTIO_FILE} ${EXTRACT_DIR}/install/kubernetes/helm/*"
RELEASE_DIR="${RELEASES_DIR}/${ISTIO_NAME}"

ISTIO_NAME=${ISTIO_NAME//./-}

: ${HELM_DIR:=${RELEASE_DIR}}

if [[ "${ISTIO_VERSION}" =~ ^1\.0\..* ]]; then
  PATCH_1_0="true"
fi

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
      curl -LfO "${ISTIO_URL}"
    )
  fi

  (
      echo "extracting Istio Helm charts to ${RELEASES_DIR}"
      cd "${RELEASES_DIR}"
      rm -rf ${EXTRACT_DIR}
      ${EXTRACT_CMD}
      cp -rf ${EXTRACT_DIR}/install/kubernetes/helm/* ${HELM_DIR}/
      #(
      #  cd "${HELM_DIR}/istio"
      #  helm dep update
      #)
  )
}

function sed() {
  echo "ERROR: detected direct sed invocation"
  echo "Please use sed_wrap. It is a wrapper around sed that fails when no changes have been detected."
  echo "Failed call was: sed $@"
  return 1
}

function sed_wrap() {
  for filename; do true; done # this retrieves the last argument
  echo "patching $filename"
  state=$(cat $filename)
  command sed "$@"
  difference=$(diff <(echo "${state}") <(cat ${filename}) || true )
  if [[ -z "${difference}" ]]; then
    echo "ERROR: nothing changed, sed seems to not have matched. Exiting"
    echo "Failed call: sed $*"
    return 10
  fi
}

retrieveIstioRelease

source $(dirname ${BASH_SOURCE})/patch-charts.sh

(
  cd "${RELEASES_DIR}"
  echo "producing diff file for charts: $(pwd)/chart-diffs.diff"
  diff -uNr ${EXTRACT_DIR}/install/kubernetes/helm/ ${HELM_DIR}/ > chart-diffs.diff || [ $? -eq 1 ]
)
