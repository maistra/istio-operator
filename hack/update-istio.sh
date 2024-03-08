#!/bin/bash

# Copyright Istio Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

SLEEP_TIME=10
VERSIONS_YAML_FILE=${VERSIONS_YAML_FILE:-"versions.yaml"}

COMMIT=$(yq '.versions[] | select(.name == "latest") | "git ls-remote --heads " + .repo + ".git " + .branch + " | cut -f 1"' "${VERSIONS_YAML_FILE}" | sh)
CURRENT=$(yq '.versions[] | select(.name == "latest") | .commit' "${VERSIONS_YAML_FILE}")

if [ "${COMMIT}" == "${CURRENT}" ]; then
  echo "${VERSIONS_YAML_FILE} is already up-to-date with latest commit ${COMMIT}."
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

yq -i '(.versions[] | select(.name == "latest") | .commit) = "'"${COMMIT}"'"' "${VERSIONS_YAML_FILE}"
yq -i '
    (.versions[] | select(.name == "latest") | .charts) = [
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/base-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/cni-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/gateway-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/istiod-'"${FULL_VERSION}"'.tgz",
        "https://storage.googleapis.com/istio-build/dev/'"${FULL_VERSION}"'/helm/ztunnel-'"${FULL_VERSION}"'.tgz"
    ]' "${VERSIONS_YAML_FILE}"
