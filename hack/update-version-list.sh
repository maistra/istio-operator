#!/bin/env bash

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

function updateVersionsInIstioTypeComment() {
    selectValues=$(yq '.versions[].name | ", \"urn:alm:descriptor:com.tectonic.ui:select:" + . + "\""' versions.yaml | tr -d '\n')
    versionsEnum=$(yq '.versions[].name' versions.yaml | tr '\n' ';' | sed 's/;$//g')
    versions=$(yq '.versions[].name' versions.yaml | tr '\n' ',' | sed -e 's/,/, /g' -e 's/, $//g')

    sed -i -E \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName=\"Istio Version\",xDescriptors=\{.*fieldGroup:General\")[^}]*(})/\1$selectValues}/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:validation:Enum=)(.*)/\1$versionsEnum/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \Must be one of:)(.*)/\1 $versions./g" \
      api/v1alpha1/istio_types.go api/v1alpha1/istiorevision_types.go
}

function updateVersionsInCSVDescription() {
    tmpFile=$(mktemp)

    # 1. generate version list from versions.yaml into temporary file
    # yq command does the following:
    # - stores latest commit in $latestCommit
    # - iterates over keys and prints them; if the key is "latest", appends the hash stored in $latestCommit
    # shellcheck disable=SC2016
    yq '(.versions[] | select(.name == "latest") | .commit) as $latestCommit | .versions[].name | (select(. == "latest") | . + " (" + $latestCommit + ")") // .' versions.yaml > "$tmpFile"

    # truncate the latest commit hash to 8 characters
    sed -i -E 's/(latest \(.{8}).*\)/\1\)/g' "$tmpFile"

    # 2. replace the version list in the CSV description
    csv="config/manifests/bases/sailoperator.clusterserviceversion.yaml"
    awk '
        /This version of the operator supports the following Istio versions:/ {
            in_version_list = 1;
            print;
            print "";
            while (getline < "'"$tmpFile"'") print "    - " $0;
            print "";
        }
        /See this page/ {
            if (in_version_list) {
                in_version_list = 0;
            }
        }
        !in_version_list {
            print;
        }
    ' "$csv" > "$csv.tmp" && mv "$csv.tmp" "$csv"

    rm "$tmpFile"
}

updateVersionsInIstioTypeComment
updateVersionsInCSVDescription
