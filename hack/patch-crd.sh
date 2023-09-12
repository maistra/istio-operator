#!/bin/bash

set -e -u

CUR_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

source hack/sed_wrapper.sh

: "${YQ:=${CUR_DIR}/../bin/yq}"
: "${VALUES_TYPES_PROTO_FILE_URL:=https://raw.githubusercontent.com/istio/istio/master/operator/pkg/apis/istio/v1alpha1/values_types.proto}"
: "${CRD_FILE:=${CUR_DIR}/../bundle/manifests/operator.istio.io_istios.yaml}"

values_yaml_path=".spec.versions.[] | select(.name == strenv(API_VERSION)) | .schema.openAPIV3Schema.properties.spec.properties.values"

declare -A values

function download_values_types_proto_file() {
  if [ $# -ne 1 ]; then
    echo "Usage: download_values_types_proto_file <destination_directory>"
    exit 1
  fi

  dst_dir="${1}"
  curl --silent "${VALUES_TYPES_PROTO_FILE_URL}" --output "${dst_dir}/values_types.proto"
  echo "${dst_dir}/values_types.proto"
}

function get_fields() {  
  if [ $# -ne 2 ]; then
    echo "Usage: get_fields <proto_file> <config>"
    exit 1
  fi

  local proto_file="${1}"
  local config="${2}"

  awk "/message ${config}/{ f = 1 } f; /}/{ f = 0 }" "${proto_file}" \
    | grep "^  [a-z]\|^  .*Config" \
    | grep -v "//*\|\[deprecated=true\]" \
    | awk '/;/{if ($1=="repeated") {printf "%s:%s ",$2,$3} else if ($1=="map<string,") {printf "object:%s ",$3} else {printf "%s:%s ",$1,$2}}'
}

function set_values() {
  if [ $# -ne 1 ]; then
    echo "Usage: set_values <proto_file>"
    exit 1
  fi

  local proto_file="${1}"

  local config_name
  local config_value

  values_fields="$(awk "/message Values/{ f = 1 } f; /}/{ f = 0 }" "${proto_file}" \
    | grep -v "//*\|\[deprecated=true\]" \
    | awk '/;/{if ($1=="repeated") {printf "%s:%s ",$2,$3} else {printf "%s:%s ",$1,$2}}')"
  for field in ${values_fields}; do
    config_name=$(echo "$field" | awk -F':' '{print $1}')
    config_value=$(echo "$field" | awk -F':' '{print $2}')
    if [[ "${config_name}" =~ .*Config ]]; then
      values["${config_name}"]="${config_value}"
    fi
  done
}

function convert_type_to_yaml () {
  if [ $# -ne 1 ]; then
    echo "Usage: convert_type_to_yaml <type_value>"
    exit 1
  fi

  config="${1}"

  case "${config}" in
    "google.protobuf.BoolValue")
      echo "boolean" 
      ;;
    "google.protobuf.Value")
      echo "string"
      ;;
    "string")
      echo "string"
      ;;
    "uint32")
      echo "integer"
      ;;
    *"Config")
      echo "${config}"
      ;;
    *)
      echo "object"
      ;;
  esac
}

function set_fields() {
  if [ $# -ne 3 ]; then
    echo "Usage: set_fields <proto_file> <crd_file> <config>"
    exit 1
  fi

  local proto_file="${1}"
  local crd_file="${2}"
  local config="${3}"

  set_values "${proto_file}"

  ${YQ} -i "( ${values_yaml_path}.properties.${values["${config}"]}.type ) = \"object\"" "${CRD_FILE}"

  local config_fields
  config_fields="$(get_fields "${proto_file}" "${config}")"

  for field in ${config_fields}; do
    type=$(echo "$field" | awk -F':' '{print $1}')
    name=$(echo "$field" | awk -F':' '{print $2}')
    ${YQ} -i "( ${values_yaml_path}.properties.${values["${config}"]}.properties.${name}.type ) = \"$(convert_type_to_yaml "${type}")\"" "${CRD_FILE}"
  done
}

function get_nested_config_fields() {
  if [ $# -ne 1 ]; then
    echo "Usage: get_nested_config_fields <file>"
    exit 1
  fi

  local file="${1}"

  grep -e "type: .*Config" "${file}" | awk '{print $2}' | sort | uniq
}

function set_nested_config_fields() {
  if [ $# -ne 3 ]; then
    echo "Usage: set_nested_config_fields <crd_file> <config_name>"
    exit 1
  fi

  local proto_file="${1}"
  local crd_file="${2}"
  local config="${3}"

  local config_fields
  
  config_fields="$(get_fields "${proto_file}" "${config}")"

  sed_wrap -i -e 's/^\([[:space:]]*\)type: '"${config}"'$/&\n\1properties:/' "${crd_file}"

  for field in ${config_fields}; do
    type=$(echo "$field" | awk -F':' '{print $1}')
    name=$(echo "$field" | awk -F':' '{print $2}')
    sed_wrap -i -e '/type: '"${config}"'/,/properties:/ {s/^\([[:space:]]*\)properties:$/&\n\1  '"${name}"':\n\1    type: '"$(convert_type_to_yaml "${type}")"'/}' "${crd_file}"
  done

  sed_wrap -i -e 's/type: '"${config}"'/type: object/' "${crd_file}"
}

## MAIN

# download values_types.proto file
dir="$(mktemp -d)"
values_types_proto_file="$(download_values_types_proto_file "${dir}")"

# modify values field
${YQ} -i "( ${values_yaml_path}.type ) = \"object\" |
          ( del(${values_yaml_path}.x-kubernetes-preserve-unknown-fields ))
         " "${CRD_FILE}"

set_values "${values_types_proto_file}"

for config in "${!values[@]}"; do
  set_fields "${values_types_proto_file}" "${CRD_FILE}" "${config}"
done

# set the nested fields
while true;  do
  nested_configs="$(get_nested_config_fields "${CRD_FILE}")"
  if [[ -z "${nested_configs}" ]]; then
    break
  fi
  for nested_config in ${nested_configs}; do
    set_nested_config_fields "${values_types_proto_file}" "${CRD_FILE}" "${nested_config}"
  done
done