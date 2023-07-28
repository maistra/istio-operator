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

WD=$(dirname "$0")
WD=$(cd "$WD"; pwd)

# Exit immediately for non zero status
set -e
# Check unset variables
set -u
# Print commands
set -x

OPERATOR_NS="${OPERATOR_NS:-istio-operator}"
OPERATOR_NAME="${OPERATOR_NAME:-istio-operator}"
CONTROL_PLANE_NS="${CONTROL_PLANE_NS:-default}"

BRANCH="${BRANCH:-maistra-3.0}"
ISTIO_HELM_INSTALL="https://raw.githubusercontent.com/maistra/istio-operator/${BRANCH}/config/samples/maistra.io_v1_istiohelminstall.yaml"

TIMEOUT="3m"

check_ready() {
    local NS=$1
    local POD_NAME=$2
    local DEPLOYMENT_NAME=$3

    echo "Check POD: NAME SPACE: \"${NS}\"   POD NAME: \"${POD_NAME}\""
    timeout --foreground -v -s SIGHUP -k ${TIMEOUT} ${TIMEOUT} bash --verbose -c \
    "until oc get pod --field-selector=status.phase=Running -n ${NS} | grep ${POD_NAME}; do sleep 5; done"

    if [ ${#DEPLOYMENT_NAME} -gt 0 ]; then
      echo "Check Deployment Available: NAME SPACE: \"${NS}\"   DEPLOYMENT NAME: \"${DEPLOYMENT_NAME}\""
      oc wait deployment "${DEPLOYMENT_NAME}" -n "${NS}" --for condition=Available=True --timeout=${TIMEOUT}
    fi
}


# Main

echo "Check that istio operator is running"
check_ready "${OPERATOR_NS}" "${OPERATOR_NAME}" "${OPERATOR_NAME}"


echo "Create a IstioHelmInstall Control Plane"
oc get ns "${CONTROL_PLANE_NS}" >/dev/null 2>&1 || oc create namespace "${CONTROL_PLANE_NS}"
oc apply -f "${ISTIO_HELM_INSTALL}" -n "${CONTROL_PLANE_NS}"


echo "Check that IstioHelmInstall Control Plane is running"
check_ready "${CONTROL_PLANE_NS}" "istiod" "istiod"
check_ready "${CONTROL_PLANE_NS}" "istio-egressgateway" "istio-egressgateway"
check_ready "${CONTROL_PLANE_NS}" "istio-ingressgateway" "istio-ingressgateway"


echo "Check that CNI processes are running"
check_ready "${OPERATOR_NS}" "istio-cni" ""
