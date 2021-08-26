#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
MAIN_DIR="$( cd "${SCRIPT_DIR}/.." && pwd )"

OPERATOR_NS="istio-operator"
CONTROL_PLANE_NS="istio-system"
BOOKINFO_NS="bookinfo"
OCP_VERSION="${OCP_VERSION:-4.7}"
ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
MAISTRA_BRANCH="${MAISTRA_BRANCH:-maistra-2.1}"

: "${OCP_PULL_SECRET:?"Empty variable"}"
: "${QUAY_PULL_SECRET:?"Empty variable"}"
: "${AWS_ACCESS_KEY_ID:?"Empty variable"}"
: "${AWS_SECRET_ACCESS_KEY:?"Empty variable"}"
: "${AWS_DEFAULT_PROFILE:?"Empty variable"}"
AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-west-1}"
AWS_DEFAULT_OUTPUT="${AWS_DEFAULT_OUTPUT:-text}"

INSTALLER_DIR=$(mktemp -d)

function gen_cluster_name() {
    local random_chars
    random_chars=$(LC_CTYPE=C tr -dc a-z0-9 < /dev/urandom | fold -w 5 | head -n 1)
    if [ -n "${PULL_NUMBER:-}" ]; then
        random_chars="pr${PULL_NUMBER}-${random_chars}"
    fi

    echo "jwendell-${OCP_VERSION}-${random_chars}"
}

function get_default_route() {
    oc -n openshift-image-registry get route default-route -o jsonpath='{ .spec.host }'
}

function update_quay_secret() {
  oc -n openshift-config extract secret/pull-secret --confirm
  sed -i .dockerconfigjson -e 's+}}$+,"quay.io/maistra":{"auth":"'"${QUAY_PULL_SECRET}"'","email":""}}}+'
  oc -n openshift-config set data secret/pull-secret --from-file=.dockerconfigjson
  rm .dockerconfigjson
}

function deploy_ocp() {
    local cluster_name

    cp "${SCRIPT_DIR}/install-config.yaml" "${INSTALLER_DIR}"

    cluster_name=$(gen_cluster_name)
    sed -i "s|CLUSTER_NAME|${cluster_name}|" "${INSTALLER_DIR}/install-config.yaml"
    sed -i "s|OCP_PULL_SECRET|${OCP_PULL_SECRET}|" "${INSTALLER_DIR}/install-config.yaml"

    echo "Deploying OCP version ${OCP_VERSION} (cluster name: ${cluster_name}) on AWS..."
    "openshift-install-${OCP_VERSION}" create cluster --dir="${INSTALLER_DIR}" --log-level=error
    echo "Dumping metadata.json:"
    cat "${INSTALLER_DIR}/metadata.json"

    echo "Configuring OCP..."
    export KUBECONFIG="${INSTALLER_DIR}/auth/kubeconfig"
    oc login -u kubeadmin -p "$(cat "${INSTALLER_DIR}/auth/kubeadmin-password")" --insecure-skip-tls-verify=true
    oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge

    update_quay_secret
    sleep 10

    echo "Configuring docker..."
    local default_route
    default_route="$(get_default_route)"
    echo "{\"insecure-registries\" : [\"${default_route}\"]}" > /etc/docker/daemon.json
    pkill -HUP dockerd
    docker login -u kubeadmin -p "$(oc whoami -t)" "${default_route}"

}

function deploy_operator() {
    echo "Deploying operator..."
    oc new-project "${OPERATOR_NS}" || true

    cd "${MAIN_DIR}"
    IMAGE="$(get_default_route)/${OPERATOR_NS}/op:latest" COMMUNITY=false make image.push

    local deployment_file internalImage
    deployment_file="$(mktemp)"
    internalImage=$(oc -n ${OPERATOR_NS} get is op -o jsonpath='{ .status.dockerImageRepository }')

    # Use quay.io latest QE builds
    sed "s|image: registry.redhat.io/openshift-service-mesh/istio-rhel8-operator.*|image: ${internalImage}:latest|" "deploy/servicemesh-operator.yaml" > "${deployment_file}"
    sed -i "s|: registry\.redhat\.io/openshift-service-mesh/|: quay.io/maistra/|" "${deployment_file}"
    sed -i "s|:2\.1\.[0-9]*$|:latest-2.1-qe|" "${deployment_file}"
    sed -i "s|:2\.0\.[0-9]*$|:latest-2.0-qe|" "${deployment_file}"
    sed -i "s|:1\.1\.[0-9]*$|:latest-1.1-qe|" "${deployment_file}"

    # Increase log verbosity
    sed -i '/imagePullPolicy: Always/ i\
        - --zap-devel\
        - --zap-level=2' "${deployment_file}"

    oc -n "${OPERATOR_NS}" apply -f "${deployment_file}"
    rm -f "${deployment_file}"
    oc -n "${OPERATOR_NS}" wait --for=condition=Available --timeout=120s deploy --all
    oc -n "${OPERATOR_NS}" wait --for=condition=Ready --timeout=120s pod --all
    sleep 20 # apparently operator pod is ready but not so ready :/
}

function run_tests() {
    echo "Deploying control plane..."
    oc new-project "${CONTROL_PLANE_NS}" || true
    cd "${SCRIPT_DIR}"
    oc -n "${CONTROL_PLANE_NS}" apply -f smcp-basic.yaml
    oc -n "${CONTROL_PLANE_NS}" wait --for=condition=Ready --timeout=600s smcp/basic

    # create smmr
    oc new-project "${BOOKINFO_NS}" || true
    oc -n "${CONTROL_PLANE_NS}" apply -f smmr-basic.yaml
    oc -n "${CONTROL_PLANE_NS}" wait --for=condition=Ready --timeout=120s smmr/default

    echo "Deploying bookinfo..."
    oc -n "${BOOKINFO_NS}" apply -f "https://raw.githubusercontent.com/maistra/istio/${MAISTRA_BRANCH}/samples/bookinfo/platform/kube/bookinfo.yaml"
    oc -n "${BOOKINFO_NS}" apply -f "https://raw.githubusercontent.com/maistra/istio/${MAISTRA_BRANCH}/samples/bookinfo/networking/bookinfo-gateway.yaml"
    oc -n "${BOOKINFO_NS}" wait --for=condition=Available --timeout=120s deploy --all

    echo "Testing bookinfo"
    local gateway_url
    gateway_url=$(oc -n "${CONTROL_PLANE_NS}" get route istio-ingressgateway -o jsonpath='{.spec.host}')
    curl -s "http://${gateway_url}/productpage" | grep -o "<title>.*</title>"
}

function destroy_cluster() {
    echo "Destroying OCP cluster..."
    "openshift-install-${OCP_VERSION}" destroy cluster --dir="${INSTALLER_DIR}" --log-level=error
}

function dump() {
    echo "Dumping cluster state..."
    oc cluster-info dump --namespaces="${OPERATOR_NS},${CONTROL_PLANE_NS},${BOOKINFO_NS}" > "${ARTIFACTS}/cluster-dump.json"
}

function cleanup() {
    dump || true
    destroy_cluster || true
}

function main() {
    time deploy_ocp
    time deploy_operator
    time run_tests
    echo "Test succeeded"
}

trap cleanup EXIT
time main
