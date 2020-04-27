#!/usr/bin/env bash

set -e

# include sed_wrap
source $(dirname ${BASH_SOURCE})/sed-wrapper.sh

function patch_container_image() {
    local container=$1
    local file=$2
    if [ -f "$file" ]; then
        if grep -q 'if contains "/" .Values.image' "${file}"; then return 0; fi
        sed_wrap -i -re '/^ *containers: *$/,/^ *volumes:/ {
            /- name: *'"${container}"' *$/,/imagePullPolicy:/ {
                /name:|imagePullPolicy:/!d
                /imagePullPolicy:/s+^(( *)imagePullPolicy:.*)$+\{\{- if contains "/" .Values.image \}\}\
\2image: "\{\{ .Values.image \}\}"\
\{\{- else \}\}\
\2image: "\{\{ .Values.global.hub \}\}/\{\{ .Values.image \}\}:\{\{ .Values.global.tag \}\}"\
\{\{- end \}\}\
\1+
            }
        }' "${file}"
    else
        echo "ERROR: file does not exist: ${file}"
        return 1
    fi
}

function patch_oauth_proxy_image() {
    local container=$1
    local file=$2
    if [ -f "$file" ]; then
        if grep -q 'if contains "/" .Values.global.oauthproxy.image' "${file}"; then return 0; fi
        sed_wrap -i -re '/^ *containers: *$/,/^ *volumes:/ {
            /- name: *'"${container}"' *$/,/imagePullPolicy:/ {
                /name:|imagePullPolicy:/!d
                /imagePullPolicy:/s+^(( *)imagePullPolicy:.*)$+\{\{- if contains "/" .Values.global.oauthproxy.image \}\}\
\2image: \{\{ .Values.global.oauthproxy.image \}\}\
\{\{- else \}\}\
\2image: {{ .Values.global.oauthproxy.hub }}/{{ .Values.global.oauthproxy.image }}:{{ .Values.global.oauthproxy.tag }}\
\{\{- end \}\}\
\1+
            }
        }' "${file}"
    else
        echo "ERROR: file does not exist: ${file}"
        return 1
    fi
}

function patch_3scale_container_image() {
    local file=$1
    if [ -f "$file" ]; then
        if grep -q 'if contains "/" .Values.image' "${file}"; then return 0; fi
        sed_wrap -i -re '/^ *- image:/,/^ *name: *3scale-istio-adapter *$/ {
            /image:/!d
            /image:/s+^( *)- image:.*$+\1- name: 3scale-istio-adapter\
\{\{- if contains "/" .Values.image \}\}\
\1  image: "\{\{ .Values.image \}\}"\
\{\{- else \}\}\
\1  image: "\{\{ .Values.hub \}\}/\{\{ .Values.image \}\}:\{\{ .Values.tag \}\}"\
\{\{- end \}\}\
\1  imagePullPolicy: Always+
        }' "${file}"
    else
        echo "ERROR: file does not exist: ${file}"
        return 1
    fi
}

patch_container_image galley "${HELM_DIR}/istio/charts/galley/templates/deployment.yaml"
patch_container_image '\{\{ .Chart.Name \}\}' "${HELM_DIR}/istio/charts/grafana/templates/deployment.yaml"
patch_container_image citadel "${HELM_DIR}/istio/charts/security/templates/deployment.yaml"
patch_container_image sidecar-injector-webhook "${HELM_DIR}/istio/charts/sidecarInjectorWebhook/templates/deployment.yaml"
patch_oauth_proxy_image grafana-proxy "${HELM_DIR}/istio/charts/grafana/templates/deployment.yaml"
patch_oauth_proxy_image prometheus-proxy "${HELM_DIR}/istio/charts/prometheus/templates/deployment.yaml"
patch_3scale_container_image "${HELM_DIR}/maistra-threescale/templates/deployment.yaml"
