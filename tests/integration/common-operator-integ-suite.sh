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

# To be able to run this script, it's needed to pass the flag --ocp or --kind
set -eu -o pipefail

check_arguments() {
  if [ $# -eq 0 ]; then
    echo "No arguments provided"
    exit 1
  fi
}

parse_flags() {
  SKIP_BUILD=false
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
      --skip-build)
        shift
        SKIP_BUILD=true
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
}

initialize_variables() {
  WD=$(dirname "$0")
  WD=$(cd "${WD}" || exit; pwd)

  NAMESPACE="${NAMESPACE:-istio-operator}"
  DEPLOYMENT_NAME="${DEPLOYMENT_NAME:-istio-operator}"
  CONTROL_PLANE_NS="${CONTROL_PLANE_NS:-istio-system}"
  COMMAND="kubectl"

  if [ "${OCP}" == "true" ]; then
    COMMAND="oc"
  fi

  echo "Using command: ${COMMAND}"

  if [ "${OCP}" == "true" ]; then
    ISTIO_MANIFEST="${WD}/../../config/samples/istio-sample-openshift.yaml"
  else
    ISTIO_MANIFEST="${WD}/../../config/samples/istio-sample-kubernetes.yaml"
  fi

  ISTIO_NAME=$(yq eval '.metadata.name' "$ISTIO_MANIFEST")

  TIMEOUT="3m"
}

get_internal_registry() {
  # Validate that the internal registry is running, configure the variable to be used in the Makefile. 
  # If there is no internal registry, the test can't be executed targeting to the internal registry

  # Check if the registry pods are running
  ${COMMAND} get pods -n openshift-image-registry --no-headers | grep -v "Running" && echo "It looks like the OCP image registry is not deployed or Running. This tests scenario requires it. Aborting." && exit 1

  # Check if default route already exist
  if [ -z "$(${COMMAND} get route default-route -n openshift-image-registry -o name)" ]; then
    echo "Route default-route does not exist, patching DefaultRoute to true on Image Registry."
    ${COMMAND} patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
  
    timeout --foreground -v -s SIGHUP -k ${TIMEOUT} ${TIMEOUT} bash --verbose -c \
      "until ${COMMAND} get route default-route -n openshift-image-registry &> /dev/null; do sleep 5; done && echo 'The 'default-route' has been created.'"
  fi

  # Get the registry route
  URL=$(${COMMAND} get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
  # Hub will be equal to the route url/project-name(NameSpace) 
  export HUB="${URL}/${NAMESPACE}"
  echo "Internal registry URL: ${HUB}"

  # Create namespace where operator will be located
  # This is needed because the roles are created in the namespace where the operator is deployed
  ${COMMAND} create namespace "${NAMESPACE}" || true

  # Adding roles to avoid the need to be authenticated to push images to the internal registry
  # Using envsubst to replace the variable NAMESPACE in the yaml file
  envsubst < "${WD}/config/role-bindings.yaml" | ${COMMAND} apply -f -

  # Login to the internal registry when running on CRC
  # Take into count that you will need to add before the registry URL as Insecure registry in "/etc/docker/daemon.json"
  if [[ ${URL} == *".apps-crc.testing"* ]]; then
    echo "Executing Docker login to the internal registry"
    if ! oc whoami -t | docker login -u "$(${COMMAND} whoami)" --password-stdin "${URL}"; then
      echo "***** Error: Failed to log in to Docker registry."
      echo "***** Check the error and if is related to 'tls: failed to verify certificate' please add the registry URL as Insecure registry in '/etc/docker/daemon.json'"
      exit 1
    fi
  fi
}

build_and_push_image() {
  # Build and push docker multi-platform image
  # Notes: to be able to build and push to the local registry we need to set these variables to be used in the Makefile
  # IMAGE ?= ${HUB}/${IMAGE_BASE}:${TAG}, so we need to pass hub, image_base, and tag to be able to build and push the image
  echo "Building and pushing image"
  echo "Image base: ${IMAGE_BASE}"
  echo " Tag: ${TAG}"
  IMAGE=${HUB}/${IMAGE_BASE}:${TAG} make docker-buildx
}

deploy_operator() {
  echo "Deploying Operator"
  local TARGET="deploy"
  if [ "${OCP}" == "true" ]; then
    # This is a workaround
    # To avoid errors of certificates meanwhile we are pulling the operator image from the internal registry
    # We need to set image $HUB to a fixed known value
    # This value always will be equal to the svc url of the internal registry
    HUB="image-registry.openshift-image-registry.svc:5000/istio-operator"

    TARGET="deploy-openshift"
  fi
  IMAGE=${HUB}/${IMAGE_BASE}:${TAG} make -s --no-print-directory ${TARGET}
}

check_ready() {
  local NS=$1
  local POD_NAME=$2
  local DEPLOYMENT_NAME=$3

  echo "Check POD: NAMESPACE: \"${NS}\"   POD NAME: \"${POD_NAME}\""
  timeout --foreground -v -s SIGHUP -k ${TIMEOUT} ${TIMEOUT} bash --verbose -c \
    "until ${COMMAND} get pod --field-selector=status.phase=Running -n ${NS} | grep ${POD_NAME}; do sleep 5; done"

  echo "Check Deployment Available: NAMESPACE: \"${NS}\"   DEPLOYMENT NAME: \"${DEPLOYMENT_NAME}\""
  ${COMMAND} wait deployment "${DEPLOYMENT_NAME}" -n "${NS}" --for condition=Available=True --timeout=${TIMEOUT}
}

logFailure() {
  echo
  echo "FAIL: $*"
}

main_test() {
  # Add here all the validation tests for the operator
  echo "Check that istio operator is running"
  check_ready "${NAMESPACE}" "${DEPLOYMENT_NAME}" "${DEPLOYMENT_NAME}"
  
  # Deploy and test every istio version inside versions.yaml
  versions=$(yq eval '.versions | keys | .[]' versions.yaml)
  echo "Versions to test: ${versions//$'\n'/ }"
  for ver in ${versions}; do
    echo "--------------------------------------------------------------"
    echo "Deploy Istio version '${ver}'"
    ${COMMAND} get ns "${CONTROL_PLANE_NS}" >/dev/null 2>&1 || ${COMMAND} create namespace "${CONTROL_PLANE_NS}"
    sed -e "s/version:.*/version: ${ver}/g" "${ISTIO_MANIFEST}" | ${COMMAND} apply -f -

    echo "Wait for Istio to be Reconciled"
    ${COMMAND} wait "istio/${ISTIO_NAME}" --for condition=Reconciled=True --timeout=${TIMEOUT}

    echo "Wait for Istio to be Ready"
    ${COMMAND} wait "istio/${ISTIO_NAME}" --for condition=Ready=True --timeout=${TIMEOUT}

    echo "Give the operator 30s to settle down"
    sleep 30

    echo "Check that the operator has stopped reconciling the resource (waiting 30s)"
    # wait for 30s, then check the last 30s of the log
    sleep 30
    last30secondsOfLog=$(${COMMAND} logs "deploy/${DEPLOYMENT_NAME}" -n "${NAMESPACE}" --since 30s)
    if echo "$last30secondsOfLog" | grep "Reconciliation done" >/dev/null 2>&1; then
        logFailure "Expected istio-operator to stop reconciling the resource, but it didn't:"
        echo "$last30secondsOfLog"
        echo "Note: The above log was captured at $(date)"
        exit 1
    fi

    echo "Check that Istio is running"
    check_ready "${CONTROL_PLANE_NS}" "istiod" "istiod"

    echo "Make sure only istiod got deployed and nothing else"
    res=$(${COMMAND}  -n "${CONTROL_PLANE_NS}" get deploy -o json | jq -j '.items | length')
    if [ "${res}" != "1" ]; then
      logFailure "Expected just istiod deployment, got:"
      ${COMMAND}  -n "${CONTROL_PLANE_NS}" get deploy
      exit 1
    fi

    if [ "${OCP}" == "true" ]; then
      echo "Check that CNI deamonset is ready"
      timeout --foreground -v -s SIGHUP -k ${TIMEOUT} ${TIMEOUT} bash --verbose -c \
        "until ${COMMAND}  rollout status ds/istio-cni-node -n ${NAMESPACE}; do sleep 5; done"
    else
      echo "Check that CNI daemonset was not deployed"
      if ${COMMAND} get ds/istio-cni-node -n "${NAMESPACE}" > /dev/null 2>&1; then
        logFailure "Expected CNI daemonset to not exist, but it does:"
        ${COMMAND} get ds/istio-cni-node -n "${NAMESPACE}"
        exit 1
      fi
    fi

    echo "Undeploy Istio"
    ${COMMAND} delete -f "${ISTIO_MANIFEST}"

    echo "Check that istiod deployment has been deleted (waiting $TIMEOUT)"
    timeout --foreground -v -s SIGHUP -k ${TIMEOUT} ${TIMEOUT} bash --verbose -c \
      "until ! ${COMMAND} get deployment istiod -n ${CONTROL_PLANE_NS}; do sleep 5; done"

    echo "Delete namespace ${CONTROL_PLANE_NS}"
    ${COMMAND} delete ns "${CONTROL_PLANE_NS}"
  done
}

# PRE SETUP: Get arguments and initialize variables
check_arguments "$@"
parse_flags "$@"
initialize_variables

if [ "${SKIP_BUILD}" == "false" ]; then
  # SETUP
  if [ "${OCP}" == "true" ]; then
    # Internal Registry is only available in OCP clusters
    get_internal_registry
  fi

  # BUILD AND PUSH IMAGE
  build_and_push_image
fi

# Deploy the operator
deploy_operator
# RUNNING TEST VALIDATIONS
main_test
