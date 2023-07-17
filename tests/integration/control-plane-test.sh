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

set -o pipefail

WD=$(dirname "$0")
WD=$(cd "$WD"; pwd)

ISTIO_HELM_INSTALL="https://raw.githubusercontent.com/maistra/istio-operator/maistra-3.0/config/samples/maistra.io_v1_istiohelminstall.yaml"
BRANCH="${BRANCH:-maistra-3.0}"

CP_NS="${CP_NS:-default}"

OPERATOR_NAME="${OPERATOR_NAME:-istio-operator}"
OPERATOR_NAMESPACE="${NS:-istio-operator}"

create-control-plane() {
    oc get ns "${CP_NS}" >/dev/null 2>&1 || oc create namespace "${CP_NS}"

    echo "Creating Istio Helm Install (Control Plane)"
    cd "$(git rev-parse --show-toplevel)"
    oc apply -f ${ISTIO_HELM_INSTALL} -n "${CP_NS}"

    oc project "${CP_NS}"
    timeout --foreground -v -s SIGHUP -k 2m 2m bash --verbose -c \
      "until oc get pods -n ${CP_NS} | grep istiod; do sleep 5; done"
    oc wait deployment istiod -n "${CP_NS}" --for condition=Available=True --timeout=2m
     
    timeout --foreground -v -s SIGHUP -k 2m 2m bash --verbose -c \
      "until oc get pods -n ${CP_NS} | grep istio-egressgateway; do sleep 5; done"
    oc wait deployment istio-egressgateway -n "${CP_NS}" --for condition=Available=True --timeout=2m

    timeout --foreground -v -s SIGHUP -k 2m 2m bash --verbose -c \
      "until oc get pods -n ${CP_NS} | grep istio-ingressgateway; do sleep 5; done"
    oc wait deployment istio-ingressgateway -n "${CP_NS}" --for condition=Available=True --timeout=2m
}

check-operator-status() {
    # check that operator is still Ready after Istio Helm Install create
    oc wait --for condition=Available -n "${OPERATOR_NAMESPACE}" deploy/"${OPERATOR_NAME}" --timeout=2m
}

create-control-plane
check-operator-status

