#!/usr/bin/env bash

set -euo pipefail

SLEEP_TIME=10

COMMIT=$(yq eval '"git ls-remote --heads " + .latest.repo + ".git " + .latest.branch + " | cut -f 1"' versions.yaml | sh)
CURRENT=$(yq .latest.commit versions.yaml)

if [ "${COMMIT}" == "${CURRENT}" ]; then
  echo "versions.yaml is already up-to-date with latest commit ${COMMIT}."
  exit 0
fi

echo Updating version 'latest' to commit "${COMMIT}"
echo "Verifying the artifacts are available on GCS, this might take a while - you can abort the wait with CTRL+C"

URL="https://storage.googleapis.com/istio-build/dev/${COMMIT}"
until curl --output /dev/null --silent --head --fail "${URL}"; do
    echo -n '.'
    sleep ${SLEEP_TIME}
done
echo

FULL_VERSION=$(curl -sSfL "${URL}")
echo Full version: "${FULL_VERSION}"

yq -i '
    .latest.commit="'"${COMMIT}"'" |
    .latest.charts=[
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/base-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/cni-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/gateway-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/istiod-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/ztunnel-'"${FULL_VERSION}"'.tgz"
    ]' versions.yaml
