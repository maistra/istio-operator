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

# generate a comma-separated list of all profiles across all versions in resources/
profiles=$(find resources/*/profiles -type f -name "*.yaml" -print0 | xargs -0 -n1 basename | sort | uniq | sed 's/\.yaml$//' | tr $'\n' ',' | sed 's/,$//')

selectValues=""
enumValues=""

IFS=',' read -ra elements <<< "${profiles}"
for element in "${elements[@]}"; do
  if [[ "$element" != "openshift" ]]; then
    # skip openshift profile in the drop-down, since it's always applied;
    # default is also applied, but we preserve it so that users can deselect a profile after they select it
    selectValues+=', "urn:alm:descriptor:com.tectonic.ui:select:'$element'"'
  fi
  enumValues+=$element';'
done

enumValues=${enumValues::-1}    # remove last semicolon


sed -i -E \
  -e "/\+sail:profile/,/Profile string/ s/(\/\/ \+operator-sdk:csv:customresourcedefinitions:type=spec,displayName=\"Profile\",xDescriptors=\{.*fieldGroup:General\")[^}]*(})/\1$selectValues}/g" \
  -e "/\+sail:profile/,/Profile string/ s/(\/\/ \+kubebuilder:validation:Enum=)(.*)/\1$enumValues/g" \
  -e "/\+sail:profile/,/Profile string/ s/(\/\/ Must be one of:)(.*)/\1 ${profiles//,/, }./g" \
  api/v1alpha1/istio_types.go api/v1alpha1/istiocni_types.go
