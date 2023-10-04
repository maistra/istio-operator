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

# To be able to run this script it's needed to pass the flag --ocp or --kind
if [ $# -eq 0 ]; then
  echo "No arguments provided"
  exit 1
fi

while [ $# -gt 0 ]; do
  case "$1" in
    --ocp)
      shift
      OCP=true
      ;;
    --kind)
      shift
      OCP=false
      ;;
    *)
      echo "Invalid flag: $1"
      exit 1
      ;;
  esac
done

if [ "${OCP}" == "true" ]; then
  echo "Running on OCP"
else
  echo "Running on kind"
fi

WD=$(dirname "$0")
WD=$(cd "$WD" || exit; pwd)

set -eux -o pipefail

NAMESPACE="${NAMESPACE:-istio-operator}"
OPERATOR_NAME="${OPERATOR_NAME:-istio-operator}"
CONTROL_PLANE_NS="${CONTROL_PLANE_NS:-istio-system}"
COMMAND="kubectl"

if [ "${OCP}" == "true" ]; then
  COMMAND="oc"
fi

BRANCH="${BRANCH:-maistra-3.0}"

if [ "${OCP}" == "true" ]; then
  ISTIO_MANIFEST="${WD}/../../config/samples/istio-sample-openshift.yaml"
else
  ISTIO_MANIFEST="${WD}/../../config/samples/istio-sample-kubernetes.yaml"
fi

TIMEOUT="3m"

check_ready() {
    local NS=$1
    local POD_NAME=$2
    local DEPLOYMENT_NAME=$3

    echo "Check POD: NAME SPACE: \"${NS}\"   POD NAME: \"${POD_NAME}\""
    timeout --foreground -v -s SIGHUP -k ${TIMEOUT} ${TIMEOUT} bash --verbose -c \
    "until $COMMAND  get pod --field-selector=status.phase=Running -n ${NS} | grep ${POD_NAME}; do sleep 5; done"

    echo "Check Deployment Available: NAME SPACE: \"${NS}\"   DEPLOYMENT NAME: \"${DEPLOYMENT_NAME}\""
    $COMMAND  wait deployment "${DEPLOYMENT_NAME}" -n "${NS}" --for condition=Available=True --timeout=${TIMEOUT}
}


# Main

echo "Check that istio operator is running"
check_ready "${NAMESPACE}" "${OPERATOR_NAME}" "${OPERATOR_NAME}"


echo "Deploy Istio"
$COMMAND  get ns "${CONTROL_PLANE_NS}" >/dev/null 2>&1 || oc create namespace "${CONTROL_PLANE_NS}"
$COMMAND  apply -f "${ISTIO_MANIFEST}" -n "${CONTROL_PLANE_NS}"


echo "Check that Istio is running"
check_ready "${CONTROL_PLANE_NS}" "istiod" "istiod"

echo "Make sure only istiod got deployed and nothing else"
res=$($COMMAND  -n "${CONTROL_PLANE_NS}" get deploy -o json | jq -j '.items | length')
if [ "${res}" != "1" ]; then
  echo "Expected just istiod deployment, got:"
  $COMMAND  -n "${CONTROL_PLANE_NS}" get deploy
  exit 1
fi

echo "Check that CNI deamonset ready are running"
timeout --foreground -v -s SIGHUP -k ${TIMEOUT} ${TIMEOUT} bash --verbose -c \
    "until $COMMAND  rollout status ds/istio-cni-node -n ${NAMESPACE}; do sleep 5; done"
