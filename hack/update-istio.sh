#!/usr/bin/env bash

set -euo pipefail

COMMIT=$(yq eval '"git ls-remote --heads " + .latest.repo + ".git " + .latest.branch + " | cut -f 1"' versions.yaml | sh)
echo Updating version 'latest' to commit "${COMMIT}"

FULL_VERSION=$(curl -s "https://storage.googleapis.com/istio-build/dev/$COMMIT")
echo Full version: "${FULL_VERSION}"

yq -i '
    .latest.commit="'"$COMMIT"'" |
    .latest.charts=[
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/base-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/cni-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/gateway-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/istiod-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/ztunnel-'"${FULL_VERSION}"'.tgz"
    ]' versions.yaml
