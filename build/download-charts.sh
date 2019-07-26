#!/usr/bin/env bash

set -e

RELEASES_DIR=${OUT_DIR}/helm
ISTIO_FILE="${MAISTRA_BRANCH}.tar.gz"
ISTIO_URL="https://github.com/Maistra/istio/archive/${MAISTRA_BRANCH}.tar.gz"
HELM_DIR="${RELEASES_DIR}/${MAISTRA_BRANCH}-patched"
ORIGINAL_DIR="${RELEASES_DIR}/${MAISTRA_BRANCH}-original"

function retrieveIstioRelease() {
  rm -rf "${HELM_DIR}" "${ORIGINAL_DIR}"
  mkdir -p "${ORIGINAL_DIR}"

  if [ ! -f "${RELEASES_DIR}/${ISTIO_FILE}" ] ; then
    (
      echo "Downloading Istio Release: ${ISTIO_URL}"
      cd "${RELEASES_DIR}"
      curl -sLO "${ISTIO_URL}"
    )
  else
    echo "Skipping download of charts, file ${ISTIO_FILE} already exists"
  fi

  (
      echo "extracting Istio Helm charts to ${RELEASES_DIR}"
      cd "${RELEASES_DIR}"
      tar xfz ${ISTIO_FILE} -C "${ORIGINAL_DIR}" --strip=4 istio-${MAISTRA_BRANCH}/install/kubernetes/helm/*
      cp -r "${ORIGINAL_DIR}" "${HELM_DIR}"
  )
}

function produceDiff() {
  (
    cd "${RELEASES_DIR}"
    echo "producing diff file for charts: $(pwd)/chart-diffs.diff"
    diff -uNr "${MAISTRA_BRANCH}-original" "${MAISTRA_BRANCH}-patched" > chart-diffs.diff || [ $? -eq 1 ] 
  )
}

function main() {
  retrieveIstioRelease
  source ${MAIN_DIR}/build/patch-charts.sh
  produceDiff
}

main