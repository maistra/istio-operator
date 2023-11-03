#!/bin/env bash

# generate a comma-separated list of all profiles across all versions in resources/
profiles=$(find resources/*/profiles -type f -name "*.yaml" -print0 | xargs -0 -n1 basename | sort | uniq | sed 's/\.yaml$//' | tr $'\n' ',' | sed 's/,$//')

selectValues=""
enumValues=""

IFS=',' read -ra elements <<< "${profiles}"
for element in "${elements[@]}"; do
  if [[ "$element" != "default" && "$element" != "openshift" ]]; then
    # skip default and openshift profiles in the drop-down, since these profiles are always applied
    selectValues+=', "urn:alm:descriptor:com.tectonic.ui:select:'$element'"'
  fi
  enumValues+=$element';'
done

enumValues=${enumValues::-1}    # remove last semicolon


sed -i -E \
  -e "/\+sail:profile/,/Profile string/ s/(\/\/ \+operator-sdk:csv:customresourcedefinitions:type=spec,displayName=\"Profile\",xDescriptors=\{.*fieldGroup:General\")[^}]*(})/\1$selectValues}/g" \
  -e "/\+sail:profile/,/Profile string/ s/(\/\/ \+kubebuilder:validation:Enum=)(.*)/\1$enumValues/g" \
  -e "/\+sail:profile/,/Profile string/ s/(\/\/ Must be one of:)(.*)/\1 ${profiles//,/, }./g" \
  api/v1alpha1/istio_types.go
