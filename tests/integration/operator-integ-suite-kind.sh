#!/bin/bash


# Copyright 2022 Red Hat, Inc.

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

WD=$(dirname "$0")
WD=$(cd "$WD"; pwd)
export CLUSTER_NAME
CLUSTER_NAME="maistra-operator-$(date +%s)"

# Exit immediately for non zero status
set -e
# Check unset variables
set -u
# Print commands
set -x

# cleanup_kind_cluster takes a single parameter CLUSTER_NAME
# and deletes the KinD cluster with that name
function cleanup_kind_cluster() {
  echo "Test exited with exit code $?."
  CLUSTER_NAME="${1}"
  if [[ -z "${SKIP_CLEANUP:-}" ]]; then
    echo "Cleaning up kind cluster"
    kind delete cluster --name "${CLUSTER_NAME}" -v9 || true
  fi
}

# explicitly disable shellcheck since we actually want $CLUSTER_NAME to expand now
# shellcheck disable=SC2064
trap "cleanup_kind_cluster ${CLUSTER_NAME}" EXIT

# provision a kind cluster
echo "--------------------------------"
echo "Provision a kind cluster"
echo "--------------------------------"
"${WD}"/istio-integ-suite-kind.sh

echo "--------------------------------"
echo "Build an operator image"
echo "--------------------------------"
"${WD}"/build-operator.sh

# deploy operator in kind
echo "--------------------------------"
echo "Deploy istio operator in kind"
echo "--------------------------------"
"${WD}"/deploy-operator.sh
# wait for validation webhook
echo "Wait 30s for validation webhook..."
sleep 30

# create a SMCP and test httpbin
echo "--------------------------------"
echo "Create a SMCP and test httpbin"
echo "--------------------------------"
"${WD}"/smcp-httpbin-test.sh

# delete the kind cluster
cleanup_kind_cluster "${CLUSTER_NAME}"