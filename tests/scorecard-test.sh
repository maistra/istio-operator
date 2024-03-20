#!/bin/bash

# Copyright Istio Authors

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

set -eux -o pipefail

SCRIPTPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT="$(dirname "${SCRIPTPATH}")"

# shellcheck source=common/scripts/kind_provisioner.sh
source "${ROOT}/common/scripts/kind_provisioner.sh"

# Create a temporary kubeconfig
KUBECONFIG="$(mktemp)"
export KUBECONFIG

# Create the kind cluster
export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-kind}"
export DEFAULT_CLUSTER_YAML="${ROOT}/tests/e2e/config/default.yaml"
export ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
export IP_FAMILY="${IP_FAMILY:-ipv4}"
setup_kind_cluster "${KIND_CLUSTER_NAME}" "" "" "true" "true"

kind export kubeconfig --name="${KIND_CLUSTER_NAME}"

# Run the test
OPERATOR_SDK="${OPERATOR_SDK:-operator-sdk}"
${OPERATOR_SDK} scorecard --kubeconfig="${KUBECONFIG}" --namespace=default bundle
