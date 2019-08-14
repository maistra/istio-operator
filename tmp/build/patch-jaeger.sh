#!/usr/bin/env bash

function jaeger_patch_values() {
	# update jaeger zipkin port name
  sed -i -e '/service:$/,/externalPort:/ {
    s/name:.*$/name: zipkin/
	}' -e '/jaeger:$/,/^[^ 	]/ {
          /jaeger:/a\
  # include elasticsearch to support default configurations\
  elasticsearch: {}
          /hub:/d
          /tag:/d
        }' ${HELM_DIR}/istio/charts/tracing/values.yaml
}

function jaeger_remove_files() {
  rm -f ${HELM_DIR}/istio/charts/tracing/templates/deployment-jaeger.yaml
  rm -f ${HELM_DIR}/istio/charts/tracing/templates/service-jaeger.yaml
  rm -f ${HELM_DIR}/istio/charts/tracing/templates/ingress.yaml
}

function JaegerPatch() {
  echo "Patching Jaeger"

  jaeger_remove_files
}

jaeger_patch_values
JaegerPatch
