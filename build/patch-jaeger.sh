#!/usr/bin/env bash

function jaeger_patch_values() {
	# update jaeger zipkin port name
  sed_wrap -i -e '/service:$/,/externalPort:/ {
    s/name:.*$/name: zipkin/
	}' ${HELM_DIR}/istio-telemetry/tracing/values.yaml
  sed_wrap -i -e '/jaeger:$/,/^[^ 	]/ {
          /jaeger:/a\
  # include elasticsearch to support default configurations\
  elasticsearch: {}
          /hub:/d
          /tag:/d
        }' ${HELM_DIR}/istio-telemetry/tracing/values.yaml
}

function jaeger_remove_files() {
  rm -f ${HELM_DIR}/istio-telemetry/tracing/templates/deployment-jaeger.yaml
  rm -f ${HELM_DIR}/istio-telemetry/tracing/templates/service-jaeger.yaml
  rm -f ${HELM_DIR}/istio-telemetry/tracing/templates/ingress.yaml
  rm -f ${HELM_DIR}/istio-telemetry/tracing/templates/pvc.yaml
}

function JaegerPatch() {
  echo "Patching Jaeger"

  jaeger_remove_files
}

jaeger_patch_values
JaegerPatch
