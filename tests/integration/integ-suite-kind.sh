#!/bin/bash

# Copyright 2023 Red Hat, Inc.

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
ROOT=$(dirname "$(dirname "${SCRIPTPATH}")")

# shellcheck source=common/scripts/kind_provisioner.sh
source "${ROOT}/common/scripts/kind_provisioner.sh"

# We run a local-registry in a docker container that KinD nodes pull from
export KIND_REGISTRY_NAME="kind-registry"
export KIND_REGISTRY_PORT="5000"
export KIND_REGISTRY="localhost:${KIND_REGISTRY_PORT}"
export DEFAULT_CLUSTER_YAML="${SCRIPTPATH}/config/default.yaml"
export IP_FAMILY="${IP_FAMILY:-ipv4}"
export ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"

# Set variable for cluster kind name
export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-operator-integration-tests}"

# Use the local registry instead of the default HUB
export HUB="${KIND_REGISTRY}"
# Workaround make inside make: ovewrite this variable so it is not recomputed in Makefile.core.mk
export IMAGE="${HUB}/${IMAGE_BASE:-istio-operator}:${TAG:-latest}"

# Copied from Istio: https://github.com/istio/istio/blob/861abfbc050c5be41154054853fe70336a851ce9/prow/lib.sh#L149
function setup_kind_registry() {
  # create a registry container if it not running already
  running="$(docker inspect -f '{{.State.Running}}' "${KIND_REGISTRY_NAME}" 2>/dev/null || true)"
  if [[ "${running}" != 'true' ]]; then
      docker run \
        -d --restart=always -p "${KIND_REGISTRY_PORT}:5000" --name "${KIND_REGISTRY_NAME}" \
        gcr.io/istio-testing/registry:2

    # Allow kind nodes to reach the registry
    docker network connect "kind" "${KIND_REGISTRY_NAME}"
  fi

  # https://docs.tilt.dev/choosing_clusters.html#discovering-the-registry
  # TODO get context/config from existing variables
  kind export kubeconfig --name="${KIND_CLUSTER_NAME}"
  for node in $(kind get nodes --name="${KIND_CLUSTER_NAME}"); do
    kubectl annotate node "${node}" "kind.x-k8s.io/registry=localhost:${KIND_REGISTRY_PORT}" --overwrite;
  done
}

setup_kind_cluster "${KIND_CLUSTER_NAME}" "" "" "true" "true"
setup_kind_registry

# Run the integration tests
echo "Running integration tests"
./tests/integration/common-operator-integ-suite.sh --kind
