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

# Exit immediately for non zero status
set -e
# Check unset variables
set -u
# Print commands
set -x

check-cni-ocp() { # Check that operator is running on OCP
    local ROOT
    ROOT="$(git rev-parse --show-toplevel)"
    local NS="${NS:-istio-operator}"
    
    local OPERATOR_NAME="${OPERATOR_NAME:-istio-operator}"
    local OPERATOR_NAMESPACE="${NS:-istio-operator}"
 
    echo "--------------------------------"
    echo "Check that cni processes are running as expected"
    echo "Operator Namespace: ${OPERATOR_NAMESPACE}"

    oc project "${OPERATOR_NAMESPACE}"
    timeout --foreground -v -s SIGHUP -k 2m 2m bash --verbose -c \
      "until oc get pods -n ${NS} --field-selector status.phase=Running | grep istio-cni; do sleep 5; done"
}

check-cni-ocp
