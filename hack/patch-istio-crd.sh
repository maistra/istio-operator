#!/bin/bash

set -e -u

CUR_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

: "${YQ:=yq}"
: "${API_VERSION:=v1alpha1}"
: "${VERSIONS_FILE:=${CUR_DIR}/../versions.yaml}"
: "${CRD_FILE:=${CUR_DIR}/../config/crd/bases/operator.istio.io_istios.yaml}"

values_yaml_path=".spec.versions.[] | select(.name == \"${API_VERSION}\") | .schema.openAPIV3Schema.properties.spec.properties.values"

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

  # Getting the values_types.proto url from the latest version
  values_types_proto_file_url="$(${YQ} 'to_entries | .[0].value | .repo + "/" + .commit + "/operator/pkg/apis/istio/v1alpha1/values_types.proto"' "${VERSIONS_FILE}"  | sed "s/github.com/raw.githubusercontent.com/")"
  curl --silent "${values_types_proto_file_url}" --output "${dst_dir}/values_types.proto"
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
    | awk '/;/{gsub(";","");print $0}' \
    | awk '{if ($1=="repeated") {type=$2;name=$3} else if ($1=="map<string,") {type="object";name=$3} else if ($0~/json_name/) {type=$1;name=substr(substr($5,0,length($5)-2),13)} else {type=$1;name=$2;}{print type":"name}}'
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
  ${YQ} -i "( ${values_yaml_path}.properties.${value_name}.type ) = \"object\"" "${crd_file}"

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
    ${YQ} -i "( ${values_yaml_path}.properties.${value_name}.properties.${name}.type ) = \"$(convert_type_to_yaml "${type}")\"" "${crd_file}"
  done
}

# Gets the nested configurations full path
# Params:
#   $1: The name of the nested configuration
# Example of the full formatted path of a nested configuration:
# .spec.versions.0.schema.openAPIV3Schema.properties.spec.properties.values.properties.gateways.properties.istio-egressgateway
function get_nested_config_paths() {
  if [ $# -ne 1 ]; then
    echo "Usage: get_nested_config_paths <config_name>"
    exit 1
  fi

  local config_name="${1}"

  # Counting the number of paths for a specific nested configuration
  total_configs=$(${YQ} "( ${values_yaml_path} | .. | select(. == \"${config_name}\") | [{\"path\":path}] )" "${crd_file}" | \
    grep -c "path:")

  # Formatting the path from the yq yaml output
  for config_number in $(seq 0 "$(( "${total_configs}" - 1))"); do
    ${YQ} "( ${values_yaml_path} | .. | select(. == \"${config_name}\") | [{\"path\":path}] )" "${crd_file}" | \
      ${YQ} ".${config_number}.path" | sed -e 's/.*type.*//g' -e 's/-\ /./g' | tr -d '\n'
    echo
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
#         istio-egressgateway:
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

  paths="$(get_nested_config_paths "${config}")"

  for nested_config_path in ${paths}; do
    for field in ${config_fields}; do
      type=$(echo "$field" | awk -F':' '{print $1}')
      name=$(echo "$field" | awk -F':' '{print $2}')
      if [ -n "${type}" ] && [ -n "${name}" ]; then
        # Adding every field_name and field_type of the nested configuration
        # Example:
        # values:
        #   properties:
        #     gateways:
        #       properties:
        #         istio-egressgateway:
        #           type: EgressGatewayConfig
        #           properties:
        #             name:
        #               type: string
        ${YQ} -i "( ${nested_config_path}.properties.${name}.type ) = \"$(convert_type_to_yaml "${type}")\"" "${crd_file}"
      fi
    done

    # Changing the <nested_config>.type to object
    # Example:
    # values:
    #   properties:
    #     gateways:
    #       properties:
    #         istio-egressgateway:
    #           type: object
    #           properties:
    #             name:
    #               type: string
    ${YQ} -i "( ${nested_config_path}.type ) = \"object\"" "${crd_file}"
  done
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