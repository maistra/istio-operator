#!/usr/bin/env bash

function updateVersionsInIstioTypeComment() {
    selectValues=$(yq '.versions | keys | .[] | ", \"urn:alm:descriptor:com.tectonic.ui:select:" + . + "\""' versions.yaml | tr -d '\n')
    versionsEnum=$(yq '.versions | keys | .[]' versions.yaml | tr '\n' ';' | sed 's/;$//g')
    versions=$(yq '.versions | keys | .[]' versions.yaml | tr '\n' ',' | sed -e 's/,/, /g' -e 's/, $//g')

    sed -i -E \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName=\"Istio Version\",xDescriptors=\{.*fieldGroup:General\")[^}]*(})/\1$selectValues}/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:validation:Enum=)(.*)/\1$versionsEnum/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \Must be one of:)(.*)/\1 $versions./g" \
      api/v1alpha1/istio_types.go
}

function updateVersionsInCSVDescription() {
    tmpFile=$(mktemp)

    # 1. generate version list from versions.yaml into temporary file
    # yq command does the following:
    # - stores latest commit in $latestCommit
    # - iterates over keys and prints them; if the key is "latest", appends the hash stored in $latestCommit
    # shellcheck disable=SC2016
    yq '.versions | .latest.commit as $latestCommit | keys | .[] | (select(. == "latest") | . + " (" + $latestCommit + ")") // .' versions.yaml > "$tmpFile"

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
