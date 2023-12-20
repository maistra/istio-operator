#!/bin/bash

# Copyright Istio Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

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