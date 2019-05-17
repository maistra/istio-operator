#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

vendor/k8s.io/code-generator/generate-groups.sh \
deepcopy \
github.com/maistra/istio-operator/pkg/generated \
github.com/maistra/istio-operator/pkg/apis \
"istio:v1alpha3 maistra:v1" \
--go-header-file "./tmp/codegen/boilerplate.go.txt"
