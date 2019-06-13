#!/usr/bin/env bash

function jaeger_patch_values() {
  # update jaeger image hub
  if [[ "${COMMUNITY,,}" == "true" ]]; then
    sed -i -e 's+hub: docker.io/jaegertracing+hub: jaegertracing+g' \
           -e 's+tag: 1.9+tag: 1.11+g' ${HELM_DIR}/istio/charts/tracing/values.yaml
  else
    sed -i -e 's+hub: docker.io/jaegertracing+hub: distributed-tracing-tech-preview+g' \
           -e 's+tag: 1.9+tag: 1.11.0+g' ${HELM_DIR}/istio/charts/tracing/values.yaml
  fi

  # update jaeger zipkin port name
  sed -i -e '/service:$/,/externalPort:/ {
    s/name:.*$/name: jaeger-collector-zipkin/
}' ${HELM_DIR}/istio/charts/tracing/values.yaml

  # add annotations
  sed -i \
    -e 's|  annotations: {}|  annotations:\n    service.alpha.openshift.io/serving-cert-secret-name: jaeger-query-tls|' \
    ${HELM_DIR}/istio/charts/tracing/values.yaml

}

function JaegerPatch() {
  echo "Patching Jaeger"

  jaeger_patch_values
}

JaegerPatch