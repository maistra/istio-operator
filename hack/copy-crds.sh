#!/usr/bin/env bash

set -e -x -u -o pipefail

: "${CHARTS_DIR:=$1}"

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPO_ROOT=$(dirname "${SCRIPT_DIR}")
CONFIG_DIR="${REPO_ROOT}/config"

function copyCRDs() {
  # Split the YAML file into separate CRD files
  csplit -s --suppress-matched -f "${CONFIG_DIR}/crd/bases/istio-crd" -z "${CHARTS_DIR}/base/crds/crd-all.gen.yaml" '/^---$/' '{*}'

  # To hide istio CRDs in the OpenShift Console, we add them to the intenral-objects annotation in the CSV
  internalObjects=""

  # Rename the split files to <api group>_<resource name>.yaml
  for file in "${CONFIG_DIR}/crd/bases/istio-crd"*; do
    # Extract the group and resource from each CRD
    group=$(grep -oP '^\s*group:\s*\K.*' "$file" | tr -d '[:space:]')
    resource=$(grep -oP '^\s*plural:\s*\K.*' "$file" | tr -d '[:space:]')
    # Add the CRD to the list of internal objects
    internalObjects+="\"${resource}.${group}\","
    # Rename the file to <group>_<resource>.yaml
    mv "$file" "${CONFIG_DIR}/crd/bases/${group}_${resource}.yaml"
  done

  # Remove existing list of CRD files from kustomization.yaml
  sed -i '/resources:/,/#+kubebuilder:scaffold:crdkustomizeresource/ {
    /resources:/n
    /#+kubebuilder:scaffold:crdkustomizeresource/!d
    }' "${CONFIG_DIR}/crd/kustomization.yaml"

  # Create YAML snippet containing list of CRD files
  pushd "${CONFIG_DIR}/crd"
  { find "bases/"*.yaml | sed 's/^/- /'; } > "${CONFIG_DIR}/crd/crdfiles.yaml"
  popd

  # Insert snippet into kustomization.yaml
  sed -i '/resources:/r '"${CONFIG_DIR}/crd/crdfiles.yaml" "${CONFIG_DIR}/crd/kustomization.yaml"

  # Remove snippet file
  rm "${CONFIG_DIR}/crd/crdfiles.yaml"

  # Update internal-objects annotation in CSV
  sed -i "/operators\.operatorframework\.io\/internal-objects/ c\    operators.operatorframework.io/internal-objects: '[${internalObjects%?}]'" "${CONFIG_DIR}/manifests/bases/sailoperator.clusterserviceversion.yaml"
}

copyCRDs