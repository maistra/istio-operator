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

WD=$(dirname "$0")
WD=$(cd "$WD" || exit; pwd)

# verify if a kind cluster is running with the name istio-operator
if ! kind get clusters | grep -q "istio-operator"; then
    echo "No kind cluster found"
    # Create cluster
    kind create cluster --name istio-operator --config "${WD}/config/default.yaml"
fi

# Wait until kind cluster is running
max_retries=30
retry_interval=10

for ((i = 1; i <= max_retries; i++)); do
    if kind get clusters | grep -q "istio-operator"; then
        echo "Kind cluster is running"
        break
    fi

    echo "Waiting for kind cluster to be running (Attempt $i/$max_retries)"
    sleep $retry_interval

    if [ "$i" -eq "$max_retries" ]; then
        echo "Cluster is not ready after $max_retries attempts. Exiting."
        exit 1
    fi
done

# Run the integration tests
echo "Running integration tests"
./tests/integration/common-operator-integ-suite.sh --kind

# Delete kind cluster
kind delete cluster --name istio-operator