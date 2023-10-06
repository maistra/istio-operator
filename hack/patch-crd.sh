#!/bin/bash

set -e -u

CUR_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

source hack/sed_wrapper.sh

: "${YQ:=${CUR_DIR}/../bin/yq}"
: "${ISTIO_RELEASE:=master}"
: "${VALUES_TYPES_PROTO_FILE_URL:=https://raw.githubusercontent.com/istio/istio/${ISTIO_RELEASE}/operator/pkg/apis/istio/v1alpha1/values_types.proto}"
: "${CRD_FILE:=${CUR_DIR}/../bundle/manifests/operator.istio.io_istios.yaml}"

values_yaml_path=".spec.versions.[] | select(.name == strenv(API_VERSION)) | .schema.openAPIV3Schema.properties.spec.properties.values"

declare -A values

# Downloads the values_types.proto file from ${VALUES_TYPES_PROTO_FILE_URL} url
# Params:
#   $1: The full path of the output directory where values_types.proto file is stored
function download_values_types_proto_file() {
  if [ $# -ne 1 ]; then
    echo "Usage: download_values_types_proto_file <destination_directory>"
    exit 1
  fi

  dst_dir="${1}"
  curl --silent "${VALUES_TYPES_PROTO_FILE_URL}" --output "${dst_dir}/values_types.proto"
  echo "${dst_dir}/values_types.proto"
}

# Gets all the fields of a configuration from the values_types.proto file
# Params:
#   $1: The full path of the values_types.proto file
#   $2: The configuration name from which the fields are extracted
function get_fields() {  
  if [ $# -ne 2 ]; then
    echo "Usage: get_fields <proto_file> <config>"
    exit 1
  fi

  local proto_file="${1}"
  local config="${2}"

  # The format of a field is field_type:field_name. Ex: string:hub
  awk "/message ${config}/{ f = 1 } f; /}/{ f = 0 }" "${proto_file}" \
    | grep "^  [a-z]\|^  .*Config" \
    | grep -v "//*\|\[deprecated=true\]" \
    | awk '/;/{if ($1=="repeated") {printf "%s:%s ",$2,$3} else if ($1=="map<string,") {printf "object:%s ",$3} else {printf "%s:%s ",$1,$2}}'
}

# Adds all the main configuration values into the values array
# Params:
#   $1: The full of path of the values_types.proto file
function set_values() {
  if [ $# -ne 1 ]; then
    echo "Usage: set_values <proto_file>"
    exit 1
  fi

  local proto_file="${1}"

  local config_name
  local config_value

  # The format of a configuration is config_name:config_value. Ex: PilotConfig:pilot
  values_fields="$(awk "/message Values/{ f = 1 } f; /}/{ f = 0 }" "${proto_file}" \
    | grep -v "//*\|\[deprecated=true\]" \
    | awk '/;/{if ($1=="repeated") {printf "%s:%s ",$2,$3} else {printf "%s:%s ",$1,$2}}')"
  for field in ${values_fields}; do
    config_name=$(echo "$field" | awk -F':' '{print $1}')
    config_value=$(echo "$field" | awk -F':' '{print $2}')
    if [[ "${config_name}" =~ .*Config ]]; then
      values["${config_value}"]="${config_name}"
    fi
  done
}

# Converts the protobuf type to a compatible yaml type
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

# Adds all the fields of a value into the CRD file
# Params:
#   $1: The full of path of the values_types.proto file
#   $2: The output CRD file
#   $3: The name of the value which is added to the CRD yaml file with its fields
function set_fields() {
  if [ $# -ne 3 ]; then
    echo "Usage: set_fields <proto_file> <crd_file> <value_name>"
    exit 1
  fi

  local proto_file="${1}"
  local crd_file="${2}"
  local value_name="${3}"

  set_values "${proto_file}"

  # Adding values.properties.<value_name>.type: object
  # Example:
  # values:
  #   properties:
  #     base:
  #       type: object
  ${YQ} -i "( ${values_yaml_path}.properties.${value_name}.type ) = \"object\"" "${CRD_FILE}"

  local config_fields
  config_fields="$(get_fields "${proto_file}" "${values["${value_name}"]}")"

  for field in ${config_fields}; do
    type=$(echo "$field" | awk -F':' '{print $1}')
    name=$(echo "$field" | awk -F':' '{print $2}')
    # Adding values.properties.<value_name>.properties.<field_name>.type: <field_type>
    # Ex: values.properties.base.properties.enableCRDTemplates.type: boolean
    # Example:
    # values:
    #   properties:
    #     base:
    #       type: object
    #       properties:
    #         enableCRDTemplates:
    #           type: boolean
    ${YQ} -i "( ${values_yaml_path}.properties.${value_name}.properties.${name}.type ) = \"$(convert_type_to_yaml "${type}")\"" "${CRD_FILE}"
  done
}

# Gets all the nested configurations from the modified values_types.proto file
# Params:
#   $1: The full path of the values_types.proto file
# Example of a nested configuration:
# values:
#   properties:
#     gateways:
#       properties:
#         istio_egressgateway:
#           type: EgressGatewayConfig #here it is a nested configuration
function get_nested_config_fields() {
  if [ $# -ne 1 ]; then
    echo "Usage: get_nested_config_fields <file>"
    exit 1
  fi

  local file="${1}"

  grep -e "type: .*Config" "${file}" | awk '{print $2}' | sort | uniq
}

# Adds all the fields of a nested configuration into the CRD file
# Params:
#   $1: The full of path of the values_types.proto file
#   $2: The output CRD file
#   $3: The name of the nested configuration which is added to the CRD yaml file with its fields
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

  # Adding <nested_config>.properties with the proper indent
  # Example:
  # values:
  #   properties:
  #     gateways:
  #       properties:
  #         istio_egressgateway:
  #           type: EgressGatewayConfig
  #           properties:
  sed_wrap -i -e 's/^\([[:space:]]*\)type: '"${config}"'$/&\n\1properties:/' "${crd_file}"

  for field in ${config_fields}; do
    type=$(echo "$field" | awk -F':' '{print $1}')
    name=$(echo "$field" | awk -F':' '{print $2}')
    # Adding every field_name and field_type of the nested configuration
    # Example:
    # values:
    #   properties:
    #     gateways:
    #       properties:
    #         istio_egressgateway:
    #           type: EgressGatewayConfig
    #           properties:
    #             name:
    #               type: string
    sed_wrap -i -e '/type: '"${config}"'/,/properties:/ {s/^\([[:space:]]*\)properties:$/&\n\1  '"${name}"':\n\1    type: '"$(convert_type_to_yaml "${type}")"'/}' "${crd_file}"
  done

  # Changing the <nested_config>.type to object
  # Example:
  # values:
  #   properties:
  #     gateways:
  #       properties:
  #         istio_egressgateway:
  #           type: object
  #           properties:
  #             name:
  #               type: string
  sed_wrap -i -e 's/type: '"${config}"'/type: object/' "${crd_file}"
}

## MAIN

# Download values_types.proto file
dir="$(mktemp -d)"
values_types_proto_file="$(download_values_types_proto_file "${dir}")"

# Add values.type: object and remove values.x-kubernetes-preserve-unknown-fields
${YQ} -i "( ${values_yaml_path}.type ) = \"object\" |
          ( del(${values_yaml_path}.x-kubernetes-preserve-unknown-fields ))
         " "${CRD_FILE}"

set_values "${values_types_proto_file}"

for value in "${!values[@]}"; do
  set_fields "${values_types_proto_file}" "${CRD_FILE}" "${value}"
done

# Set the nested fields
while true;  do
  nested_configs="$(get_nested_config_fields "${CRD_FILE}")"
  if [[ -z "${nested_configs}" ]]; then
    break
  fi
  for nested_config in ${nested_configs}; do
    set_nested_config_fields "${values_types_proto_file}" "${CRD_FILE}" "${nested_config}"
  done
done

# Sort alphabetically values.properties.* recursively
${YQ} -i "( eval( ${values_yaml_path}.properties) | sort_keys(..) )" "${CRD_FILE}"