#!/usr/bin/env bash

function jaeger_patch_values() {
  # update jaeger image hub
  if [[ "${COMMUNITY,,}" == "true" ]]; then
    sed -i -e 's+hub: docker.io/jaegertracing+hub: jaegertracing+g' \
           -e 's+tag: 1.9+tag: 1.12+g' ${HELM_DIR}/istio/charts/tracing/values.yaml
  else
    sed -i -e 's+hub: docker.io/jaegertracing+hub: registry.redhat.io/distributed-tracing-tech-preview+g' \
           -e 's+tag: 1.9+tag: 1.12.0+g' ${HELM_DIR}/istio/charts/tracing/values.yaml
  fi

  # add default template
  sed -i -e '/^jaeger:/a\
\  template: production-elasticsearch' ${HELM_DIR}/istio/charts/tracing/values.yaml

  # update jaeger zipkin port name
  sed -i -e '/service:$/,/externalPort:/ {
    s/name:.*$/name: zipkin/
}' ${HELM_DIR}/istio/charts/tracing/values.yaml
}

function jaeger_remove_files() {
  if [ -f ${HELM_DIR}/istio/charts/tracing/templates/deployment-jaeger.yaml ]; then
    rm ${HELM_DIR}/istio/charts/tracing/templates/deployment-jaeger.yaml
  fi

  if [ -f ${HELM_DIR}/istio/charts/tracing/templates/service-jaeger.yaml ]; then
    rm ${HELM_DIR}/istio/charts/tracing/templates/service-jaeger.yaml
  fi
}

function jaeger_patch_production() {
  if [[ "${COMMUNITY,,}" != "true" ]]; then
    sed -i -e 's/\(image:.*\/\)all-in-one/\1jaeger-all-in-one/' ${HELM_DIR}/istio/charts/tracing/templates/jaeger-all-in-one.yaml
  fi
}

function JaegerPatch() {
  echo "Patching Jaeger"

  jaeger_patch_values
  jaeger_patch_production
  jaeger_remove_files
}

JaegerPatch