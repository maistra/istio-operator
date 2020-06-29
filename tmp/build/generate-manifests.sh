#!/usr/bin/env bash
# to generate Maistra OLM metadata: MAISTRA_VERSION=1.0.8 REPLACES_CSV=1.0.6 tmp/build/generate-manifests.sh
# to generate ServiceMesh OLM metadata: COMMUNITY=false MAISTRA_VERSION=1.0.11 REPLACES_CSV=1.0.10 tmp/build/generate-manifests.sh

set -e

COMMUNITY=${COMMUNITY:-"true"}
: ${MAISTRA_VERSION:?"Need to set maistra version, e.g. 1.0.11"}
if [[ ${COMMUNITY} == "true" ]]; then
  BUILD_TYPE="maistra"
  JAEGER_TEMPLATE="all-in-one"
  DESCRIPTION="The Maistra Operator enables you to install, configure, and manage an instance of Maistra service mesh. Maistra is based on the open source Istio project."
else
  BUILD_TYPE="servicemesh"
  JAEGER_TEMPLATE="all-in-one"
  DESCRIPTION="The OpenShift Service Mesh Operator enables you to install, configure, and manage an instance of Red Hat OpenShift Service Mesh. OpenShift Service Mesh is based on the open source Istio project."
fi
: ${DEPLOYMENT_FILE:=deploy/${BUILD_TYPE}-operator.yaml}
: ${MANIFESTS_DIR:=manifests-${BUILD_TYPE}}
BUNDLE_DIR=${MANIFESTS_DIR}/${MAISTRA_VERSION}
OPERATOR_NAME=${BUILD_TYPE}operator
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

  local csv_path=${BUNDLE_DIR}/${OPERATOR_NAME}.v${MAISTRA_VERSION}.clusterserviceversion.yaml
  cp ${MY_LOCATION}/manifest-templates/clusterserviceversion.yaml ${csv_path}

  sed -i -e 's/__NAME__/'${OPERATOR_NAME}'/g' ${csv_path}
  sed -i -e 's/__VERSION__/'${MAISTRA_VERSION}'/g' ${csv_path}
  sed -i -e 's/__DESCRIPTION__/'"$DESCRIPTION"'/' ${csv_path}
  sed -i -e 's/__JAEGER_TEMPLATE__/'${JAEGER_TEMPLATE}'/' ${csv_path}
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
  if [ -z "$REPLACES_CSV" ]; then
    sed -i '/__REPLACES_CSV__/d' ${csv_path}
  else
    sed -i -e 's+__REPLACES_CSV__+'"  replaces: $OPERATOR_NAME.v$REPLACES_CSV"'+g' ${csv_path}
  fi
}

function generatePackage() {
  local package_path=${MANIFESTS_DIR}/${BUILD_TYPE}.package.yaml
  cp ${MY_LOCATION}/manifest-templates/package.yaml ${package_path}
  sed -i -e 's/__NAME__/'${OPERATOR_NAME}'/g' ${package_path}
  sed -i -e 's/__VERSION__/'${MAISTRA_VERSION}'/g' ${package_path}
}

checkDependencies
generateServiceMeshControlPlanesCrd
generateServiceMeshMemberRollsCrd
generateCSV
generatePackage

