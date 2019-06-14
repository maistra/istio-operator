#!/bin/sh

# KIALI_CURRENT_VERSION is the version currently in the kiali-operator.yaml.
# KIALI_VERSION_TO_USE is the version you want to use.
KIALI_CURRENT_VERSION="v0.21.0"
KIALI_VERSION_TO_USE="${KIALI_VERSION_TO_USE:-${KIALI_CURRENT_VERSION}}" # Change this to "dev" if you want to use a local dev build of kiali operator
echo Using Kiali Operator version ${KIALI_VERSION_TO_USE}

oc create namespace kiali-operator
oc create namespace istio-operator
oc create namespace istio-system

KIALI_VERSION_SED_EXPR="s/${KIALI_CURRENT_VERSION}/${KIALI_VERSION_TO_USE}/"
cat ./kiali-operator.yaml | sed -e "${KIALI_VERSION_SED_EXPR}" | oc create -n kiali-operator -f -

MAISTRA_IMAGE_SED_EXPR="s/image: maistra.*istio-operator.*$/image: ${USER}\/istio-operator\:latest/"
cat ../../maistra-operator.yaml | sed -e 's/imagePullPolicy: Always/imagePullPolicy: Never/' -e "${MAISTRA_IMAGE_SED_EXPR}" | oc create -n istio-operator -f -

oc create -f ../maistra_v1_servicemeshcontrolplane_cr_basic.yaml -n istio-system
