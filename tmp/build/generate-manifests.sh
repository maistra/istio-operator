#!/usr/bin/env bash
# to generate Maistra OLM metadata: MAISTRA_VERSION=1.0.0 DEPLOYMENT_FILE=deploy/maistra-operator.yaml MANIFESTS_DIR=manifests-maistra tmp/build/generate-manifests.sh
# to generate ServiceMesh OLM metadata: MAISTRA_VERSION=1.0.0 DEPLOYMENT_FILE=deploy/servicemesh-operator.yaml MANIFESTS_DIR=manifests-servicemesh tmp/build/generate-manifests.sh

set -e

: ${DEPLOYMENT_FILE:?"Need to set location of the source deployment file, e.g. deploy/maistra-operator.yaml"}
: ${MANIFESTS_DIR:?"Need to set location of the manifests directory, e.g. manifests/"}
: ${MAISTRA_VERSION:?"Need to set maistra version, e.g. 1.0.0"}

BUNDLE_DIR=${MANIFESTS_DIR}/${MAISTRA_VERSION}
MY_LOCATION="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

mkdir -p "$BUNDLE_DIR"

function checkDependencies() {
  if ! [ -x "$(command -v jq)" ]; then
    echo "Please install jq package.'"
    exit 1
  fi

  if ! [ -x "$(command -v yq)" ]; then
    echo "Please install yq package, e.g. 'pip install --user yq'"
    exit 1
  fi
}

function generateServiceMeshControlPlanesCrd() {
  yq -s -y '.[] | select(.kind=="CustomResourceDefinition" and .metadata.name=="servicemeshcontrolplanes.maistra.io") | .' ${DEPLOYMENT_FILE} > ${BUNDLE_DIR}/servicemeshcontrolplanes.crd.yaml
}

function generateServiceMeshMemberRollsCrd() {
  yq -s -y '.[] | select(.kind=="CustomResourceDefinition" and .metadata.name=="servicemeshmemberrolls.maistra.io") | .' ${DEPLOYMENT_FILE} > ${BUNDLE_DIR}/servicemeshmemberrolls.crd.yaml
}

function generateCSV() {
  IMAGE_SRC=$(yq -s -r '.[] | select(.kind=="Deployment" and .metadata.name=="istio-operator") | .spec.template.spec.containers[0].image' ${DEPLOYMENT_FILE})
  if [ "$IMAGE_SRC" == "" ]; then
     echo "generateCSV(): Operator image source is empty, please verify source yaml/path to the field."
     exit 1
  fi

  DEPLOYMENT_SPEC=$(yq -s -r -y '.[] | select(.kind=="Deployment" and .metadata.name=="istio-operator") | .spec' ${DEPLOYMENT_FILE} | sed 's/^/          /')
  if [ "$DEPLOYMENT_SPEC" == "" ]; then
     echo "generateCSV(): Operator deployment spec is empty, please verify source yaml/path to the field."
     exit 1
  fi

  CLUSTER_ROLE_RULES=$(yq -s -y '.[] | select(.kind=="ClusterRole" and .metadata.name=="istio-operator") | .rules' ${DEPLOYMENT_FILE} | sed 's/^/        /')
  if [ "$CLUSTER_ROLE_RULES" == "null" ]; then
     echo "generateCSV(): istio-operator cluster role source is empty, please verify source yaml/path to the field."
     exit 1
  fi

  local csv_path=${BUNDLE_DIR}/maistra.v${MAISTRA_VERSION}.clusterserviceversion.yaml
  cp ${MY_LOCATION}/manifest-templates/clusterserviceversion.yaml ${csv_path}

  sed -i -e 's/__VERSION__/'${MAISTRA_VERSION}'/g' ${csv_path}
  sed -i -e 's/__DATE__/'$(date +%Y-%m-%dT%H:%M:%S%Z)'/g' ${csv_path}
  sed -i -e 's+__IMAGE_SRC__+'${IMAGE_SRC}'+g' ${csv_path}
  sed -i -e '/__CLUSTER_ROLE_RULES__/{
    s/__CLUSTER_ROLE_RULES__//
    r '<(echo "$CLUSTER_ROLE_RULES")'
  }' ${csv_path}
  sed -i -e '/__DEPLOYMENT_SPEC__/{
    s/__DEPLOYMENT_SPEC__//
    r '<(echo "$DEPLOYMENT_SPEC")'
  }' ${csv_path}
}

function generatePackage() {
  local package_path=${MANIFESTS_DIR}/maistra.package.yaml
  cp ${MY_LOCATION}/manifest-templates/package.yaml ${package_path}
  sed -i -e 's/__VERSION__/'${MAISTRA_VERSION}'/g' ${package_path}
}

checkDependencies
generateServiceMeshControlPlanesCrd
generateServiceMeshMemberRollsCrd
generateCSV
generatePackage
