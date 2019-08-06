#!/usr/bin/env bash

function jaeger_patch_values() {
	# update jaeger zipkin port name
  sed -i -e '/service:$/,/externalPort:/ {
    s/name:.*$/name: zipkin/
	}' ${HELM_DIR}/istio/charts/tracing/values.yaml
}

function jaeger_remove_files() {
  rm -f ${HELM_DIR}/istio/charts/tracing/templates/deployment-jaeger.yaml
  rm -f ${HELM_DIR}/istio/charts/tracing/templates/service-jaeger.yaml
  rm -f ${HELM_DIR}/istio/charts/tracing/templates/ingress.yaml
}

function jaeger_patch_production() {
  if [[ "${COMMUNITY,,}" != "true" ]]; then
    sed -i -e 's/\(image:.*\/\)all-in-one/\1jaeger-all-in-one/' ${HELM_DIR}/istio/charts/tracing/templates/jaeger-all-in-one.yaml
  fi
}

function JaegerPatch() {
  echo "Patching Jaeger"

  jaeger_patch_production
  jaeger_remove_files
}

jaeger_patch_values
JaegerPatch
