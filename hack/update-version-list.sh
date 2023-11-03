#!/bin/env bash

echo "Updating version list in istio_types.go..."

selectValues=$(yq e 'keys | .[] | ", \"urn:alm:descriptor:com.tectonic.ui:select:" + . + "\""' versions.yaml | tr -d '\n')
versionsEnum=$(yq e 'keys | .[]' versions.yaml | tr '\n' ';' | sed 's/;$//g')
versions=$(yq e 'keys | .[]' versions.yaml | tr '\n' ',' | sed -e 's/,/, /g' -e 's/, $//g')

sed -i -E \
  -e "/\+sail:version/,/Version string/ s/(\/\/ \+operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName=\"Istio Version\",xDescriptors=\{.*fieldGroup:General\")[^}]*(})/\1$selectValues}/g" \
  -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:validation:Enum=)(.*)/\1$versionsEnum/g" \
  -e "/\+sail:version/,/Version string/ s/(\/\/ \Must be one of:)(.*)/\1 $versions./g" \
  api/v1alpha1/istio_types.go
