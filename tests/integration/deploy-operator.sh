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

install-operator-k8s() { # installs istio-operator on kubernetes
    local ROOT
    ROOT="$(git rev-parse --show-toplevel)"
    local TAG
    TAG="${TAG:-$(git rev-parse HEAD)}"
    local NS="${NS:-openshift-operators}"
    # var name IMAGE is in kind provisioner
    local OPERATOR_IMAGE="${OPERATOR_IMAGE:-localhost:5000/istio-operator-integ}:${TAG}"
    local ISTIO_CNI_IMAGE_NAME="${ISTIO_CNI_IMAGE_NAME:-quay.io/maistra-dev/istio-cni-ubi8-integ:latest}"
    local PILOT_IMAGE_NAME="${PILOT_IMAGE_NAME:-quay.io/maistra-dev/pilot-ubi8-integ:latest}"
    local PROXY_IMAGE_NAME="${PROXY_IMAGE_NAME:-quay.io/maistra-dev/proxyv2-ubi8-integ:latest}"
    # MAISTRA_VERSION value is in Makefile

    echo "--------------------------------"
    echo "Installing operator"
    echo "Using image $OPERATOR_IMAGE"
    echo "--------------------------------"

    kubectl get ns "$NS" >/dev/null 2>&1 || kubectl create namespace "$NS"

    kubectl apply -f "${ROOT}/manifests-maistra/${MAISTRA_VERSION}/servicemeshcontrolplanes.crd.yaml"
    kubectl apply -f "${ROOT}/manifests-maistra/${MAISTRA_VERSION}/servicemeshmemberrolls.crd.yaml"
    kubectl apply -f "${ROOT}/manifests-maistra/${MAISTRA_VERSION}/servicemeshmembers.crd.yaml"

    sed -e "s/namespace: istio-operator/namespace: $NS/g" "${ROOT}/deploy/src/rbac.yaml" | kubectl apply -n "$NS" -f -
    sed -e "s/namespace: istio-operator/namespace: $NS/g" "${ROOT}/deploy/src/serviceaccount.yaml" | kubectl apply -n "$NS" -f -
    sed -e "s/namespace: istio-operator/namespace: $NS/g" "${ROOT}/deploy/src/service.yaml" | kubectl apply -n "$NS" -f -

    openssl req -x509 -newkey rsa:4096 -keyout /tmp/key.pem -out /tmp/cert.pem -sha256 -days 365 -nodes -subj "/CN=istio-operator" -addext "subjectAltName = DNS:maistra-admission-controller.$NS.svc"

    kubectl create -n "$NS" secret tls maistra-operator-serving-cert --key=/tmp/key.pem --cert=/tmp/cert.pem
    kubectl create -n "$NS" configmap maistra-operator-cabundle --from-file=service-ca.crt=/tmp/cert.pem

    sed -e "s@quay.io/maistra/istio-ubi8-operator:${MAISTRA_VERSION}@${OPERATOR_IMAGE}@g" \
        -e "s@namespace: istio-operator@namespace: $NS@g" \
        -e "s@quay.io/maistra/istio-cni-ubi8:${MAISTRA_VERSION}@${ISTIO_CNI_IMAGE_NAME}@g" \
        -e "s@quay.io/maistra/pilot-ubi8:${MAISTRA_VERSION}@${PILOT_IMAGE_NAME}@g" \
        -e "s@quay.io/maistra/proxyv2-ubi8:${MAISTRA_VERSION}@${PROXY_IMAGE_NAME}@g" \
        "${ROOT}/deploy/src/deployment-maistra.yaml" \
        | tee "/tmp/deployment.yaml"
        kubectl apply -f /tmp/deployment.yaml

    # check istio-operator pod running
    kubectl wait --for condition=Ready -n "${NS}" pod -l name=istio-operator --timeout 180s
}

install-operator-k8s
