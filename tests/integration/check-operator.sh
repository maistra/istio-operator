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

check-operator-ocp() { # Check that operator is running on OCP
    local ROOT
    ROOT="$(git rev-parse --show-toplevel)"
    local NS="${NS:-istio-operator}"
    
 
    echo "--------------------------------"
    echo "Check that operator is running as expected"
    echo "Operator Namespace: ${OPERATOR_NAMESPACE}"
    echo "Operator Name: ${OPERATOR_NAME}"
    echo "ROOT: ${ROOT}"
    
    oc get ns "${OPERATOR_NAMESPACE}" >/dev/null 2>&1 || oc create namespace "${OPERATOR_NAMESPACE}"
   
    oc project "${OPERATOR_NAMESPACE}"
    timeout --foreground -v -s SIGHUP -k 2m 2m bash --verbose -c \
      "until oc get pods -n ${NS} | grep istio; do sleep 5; done"

    oc wait --for condition=available -n "${NS}" deploy/"${OPERATOR_NAME}" --timeout=120s
}

check-operator-ocp
