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

# Exit immediately for non zero status
set -e
# Check unset variables
set -u
# Print commands
set -x

set -o pipefail

WD=$(dirname "$0")
WD=$(cd "$WD"; pwd)
ROOT="$(git rev-parse --show-toplevel)"
CR="${ROOT}/deploy/examples/maistra_v2_servicemeshcontrolplane_cr_minimal.yaml"
BRANCH="${BRANCH:-maistra-2.4}"
HTTPBIN="https://raw.githubusercontent.com/maistra/istio/${BRANCH}/samples/httpbin/httpbin.yaml"
SLEEP="https://raw.githubusercontent.com/maistra/istio/${BRANCH}/samples/sleep/sleep.yaml"
MEMBER_NS="default"

create-control-plane() { # creates a SMCP with a mini CR
    local SMCP_NS="${SMCP_NS:-istio-system}"
    kubectl get ns "${MEMBER_NS}" >/dev/null 2>&1 || kubectl create namespace "${MEMBER_NS}"
    kubectl get ns "${SMCP_NS}" >/dev/null 2>&1 || kubectl create namespace "${SMCP_NS}"

    # sets the ${MEMBER_NS} namespace as a member of a SMMR
    sed "s:#- bookinfo:- ${MEMBER_NS}:g" "${CR}" | kubectl apply -n "${SMCP_NS}" -f -

    kubectl wait --for condition=Ready -n "${SMCP_NS}" smcp/minimal --timeout 300s
    kubectl wait --for condition=Ready -n "${SMCP_NS}" smmr/default --timeout 180s
    kubectl get -n "${SMCP_NS}" smcp -o wide
}

create-httpbin() { # creates a httpbin app and injects sidecar proxy
    kubectl apply -n "${MEMBER_NS}" -f "${HTTPBIN}"
    kubectl apply -n "${MEMBER_NS}" -f "${SLEEP}"
    kubectl wait --for condition=Ready -n "${MEMBER_NS}" pod -l app=httpbin --timeout 300s
    kubectl wait --for condition=Ready -n "${MEMBER_NS}" pod -l app=sleep --timeout 180s
}

check-httpbin() {
    # checks if a httpbin pod has two containers
    echo "Check httpbin pod status:"
    kubectl get -n "${MEMBER_NS}" pod
    kubectl get -n "${MEMBER_NS}" pod -l app=httpbin \
        -o=jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.containerStatuses[*].ready}{"\n"}{end}' \
        | grep 'true true'

    # TBD: KinD network issue
    echo "Check if a request is routed through the proxy:"
    kubectl exec -n "${MEMBER_NS}" deploy/sleep -c sleep \
        -- curl --fail --retry 5 --retry-delay 3 -sS http://httpbin:8000/headers \
        | grep "X-Envoy-Attempt-Count"
}

create-control-plane
create-httpbin
check-httpbin