#!/usr/bin/env bash
# to generate Maistra OLM metadata: COMMUNITY=true MAISTRA_VERSION=1.1.5 REPLACES_CSV=1.1.3 build/generate-manifests.sh
# to generate ServiceMesh OLM metadata: COMMUNITY=false MAISTRA_VERSION=1.1.9 REPLACES_CSV=1.1.8 build/generate-manifests.sh

set -e

: ${COMMUNITY:-"true"}
: ${MAISTRA_VERSION:?"Need to set maistra version, e.g. 1.0.1"}
if [[ $MAISTRA_VERSION =~ ([0-9]+\.[0-9]+\.[0-9]+).* ]] ; then
  MAISTRA_STRIPPED_VERSION=${BASH_REMATCH[1]}
else
  MAISTRA_STRIPPED_VERSION=${MAISTRA_VERSION}
fi
MAISTRA_NAME_VERSION=${MAISTRA_VERSION//+/.}
REPLACES_CSV=${REPLACES_CSV//+/.}

if [[ ${COMMUNITY} == "true" ]]; then
  BUILD_TYPE="maistra"
  JAEGER_TEMPLATE="all-in-one"
  CSV_DESCRIPTION="The Maistra Operator enables you to install, configure, and manage an instance of Maistra service mesh. Maistra is based on the open source Istio project."
  APP_DESCRIPTION="Maistra is a platform that provides behavioral insight and operational control over a service mesh, providing a uniform way to connect, secure, and monitor microservice applications."
  DISPLAY_NAME="Maistra Service Mesh"
  DOCUMENTATION_URL="https://maistra.io/"
  BUG_URL="https://issues.redhat.com/projects/MAISTRA"
else
  BUILD_TYPE="servicemesh"
  JAEGER_TEMPLATE="all-in-one"
  CSV_DESCRIPTION="The OpenShift Service Mesh Operator enables you to install, configure, and manage an instance of Red Hat OpenShift Service Mesh. OpenShift Service Mesh is based on the open source Istio project."
  APP_DESCRIPTION="Red Hat OpenShift Service Mesh is a platform that provides behavioral insight and operational control over a service mesh, providing a uniform way to connect, secure, and monitor microservice applications."
  DISPLAY_NAME="Red Hat OpenShift Service Mesh"
  DOCUMENTATION_URL="https://docs.openshift.com/container-platform/latest/service_mesh/servicemesh-release-notes.html"
  BUG_URL="https://issues.redhat.com/projects/OSSM"
fi
: ${DEPLOYMENT_FILE:=deploy/${BUILD_TYPE}-operator.yaml}
: ${MANIFESTS_DIR:=manifests-${BUILD_TYPE}}
BUNDLE_DIR=${MANIFESTS_DIR}/${MAISTRA_NAME_VERSION}
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
  yq -s -y --indentless '.[] | select(.kind=="CustomResourceDefinition" and .metadata.name=="servicemeshcontrolplanes.maistra.io") | .' ${DEPLOYMENT_FILE} > ${BUNDLE_DIR}/servicemeshcontrolplanes.crd.yaml
}

function generateServiceMeshMemberRollsCrd() {
  yq -s -y --indentless '.[] | select(.kind=="CustomResourceDefinition" and .metadata.name=="servicemeshmemberrolls.maistra.io") | .' ${DEPLOYMENT_FILE} > ${BUNDLE_DIR}/servicemeshmemberrolls.crd.yaml
}

function generateServiceMeshMembersCrd() {
  yq -s -y --indentless '.[] | select(.kind=="CustomResourceDefinition" and .metadata.name=="servicemeshmembers.maistra.io") | .' ${DEPLOYMENT_FILE} > ${BUNDLE_DIR}/servicemeshmembers.crd.yaml
}

function generateCSV() {
  IMAGE_SRC=$(yq -s -r '.[] | select(.kind=="Deployment" and .metadata.name=="istio-operator") | .spec.template.spec.containers[0].image' ${DEPLOYMENT_FILE})
  if [ "$IMAGE_SRC" == "" ]; then
     echo "generateCSV(): Operator image source is empty, please verify source yaml/path to the field."
     exit 1
  fi

  DEPLOYMENT_SPEC=$(yq -s -r -y --indentless '.[] | select(.kind=="Deployment" and .metadata.name=="istio-operator") | .spec' ${DEPLOYMENT_FILE} | sed 's/^/          /')
  if [ "$DEPLOYMENT_SPEC" == "" ]; then
     echo "generateCSV(): Operator deployment spec is empty, please verify source yaml/path to the field."
     exit 1
  fi

  CLUSTER_ROLE_RULES=$(yq -s -y --indentless '.[] | select(.kind=="ClusterRole" and .metadata.name=="istio-operator") | .rules' ${DEPLOYMENT_FILE} | sed 's/^/        /')
  if [ "$CLUSTER_ROLE_RULES" == "null" ]; then
     echo "generateCSV(): istio-operator cluster role source is empty, please verify source yaml/path to the field."
     exit 1
  fi

  RELATED_IMAGES=$(yq -s -y --indentless '.[] | select(.kind=="Deployment" and .metadata.name=="istio-operator") | .spec.template.metadata.annotations' ${DEPLOYMENT_FILE} | \
    sed -n 's/olm\.relatedImage\.\([^:]*\): *\([^ ]*\)/- name: \1\
  image: \2/p' | \
    sed 's/^/  /')
  if [ "$RELATED_IMAGES" == "" ]; then
     echo "generateCSV(): Operator deployment contains no olm.relatedImage annotations, please verify source yaml/path to the field."
     exit 1
  fi

  ICON=$(cat ${MY_LOCATION}/manifest-templates/${BUILD_TYPE}_rgb_icon_default_128px.png | base64 | sed -e 's+^+      +')
  local csv_path=${BUNDLE_DIR}/${OPERATOR_NAME}.v${MAISTRA_VERSION}.clusterserviceversion.yaml
  cp ${MY_LOCATION}/manifest-templates/clusterserviceversion.yaml ${csv_path}

  sed -i -e '/__DEPLOYMENT_SPEC__/{
    s/__DEPLOYMENT_SPEC__//
    r '<(echo "$DEPLOYMENT_SPEC")'
  }' ${csv_path}
  sed -i -e 's/__NAME__/'${OPERATOR_NAME}'/g' ${csv_path}
  sed -i -e 's/__VERSION__/'${MAISTRA_VERSION}'/g' ${csv_path}
  sed -i -e 's/__STRIPPED_VERSION__/'${MAISTRA_STRIPPED_VERSION}'/g' ${csv_path}
  sed -i -e 's/__NAME_VERSION__/'${MAISTRA_NAME_VERSION}'/g' ${csv_path}
  sed -i -e 's/__DISPLAY_NAME__/'"$DISPLAY_NAME"'/' ${csv_path}
  sed -i -e 's/__CSV_DESCRIPTION__/'"$CSV_DESCRIPTION"'/' ${csv_path}
  sed -i -e 's/__APP_DESCRIPTION__/'"$APP_DESCRIPTION"'/' ${csv_path}
  sed -i -e 's+__DOCUMENTATION_URL__+'"$DOCUMENTATION_URL"'+' ${csv_path}
  sed -i -e 's+__BUG_URL__+'"$BUG_URL"'+' ${csv_path}
  sed -i -e 's/__JAEGER_TEMPLATE__/'${JAEGER_TEMPLATE}'/' ${csv_path}
  sed -i -e 's/__DATE__/'$(date +%Y-%m-%dT%H:%M:%S%Z)'/g' ${csv_path}
  sed -i -e 's+__IMAGE_SRC__+'${IMAGE_SRC}'+g' ${csv_path}
  sed -i -e '/__RELATED_IMAGES__/{
    r '<(echo "$RELATED_IMAGES")'
    d
  }' ${csv_path}
  sed -i -e '/__CLUSTER_ROLE_RULES__/{
    s/__CLUSTER_ROLE_RULES__//
    r '<(echo "$CLUSTER_ROLE_RULES")'
  }' ${csv_path}
  sed -i -e '/__ICON__/{
    s/__ICON__//
    r '<(echo "$ICON")'
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
  sed -i -e 's/__NAME_VERSION__/'${MAISTRA_NAME_VERSION}'/g' ${package_path}
}

checkDependencies
generateServiceMeshControlPlanesCrd
generateServiceMeshMemberRollsCrd
generateServiceMeshMembersCrd
generateCSV
generatePackage

