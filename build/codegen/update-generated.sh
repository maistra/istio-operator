#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
set -x

# FIXME: https://issues.redhat.com/browse/OSSM-2367
CODE_GENERATOR_PACKAGE="k8s.io/code-generator"
CODE_GENERATOR_VERSION="v0.18.3"
CODE_GENERATOR_PATH="$(go env GOMODCACHE)/${CODE_GENERATOR_PACKAGE}@${CODE_GENERATOR_VERSION}"
go mod tidy

bash "${CODE_GENERATOR_PATH}/generate-groups.sh" \
deepcopy \
github.com/maistra/istio-operator/pkg/generated \
github.com/maistra/istio-operator/pkg/apis \
"maistra:v1,v2" \
--go-header-file "./build/codegen/boilerplate.go.txt"

go run k8s.io/code-generator/cmd/deepcopy-gen \
    -i github.com/maistra/istio-operator/pkg/apis/maistra/status \
    --go-header-file "./build/codegen/boilerplate.go.txt" \
    -O zz_generated.deepcopy

go run k8s.io/code-generator/cmd/conversion-gen \
    -i ./pkg/apis/maistra/conversion \
    -p github.com/maistra/istio-operator/pkg/maistra/conversion \
    --go-header-file "./build/codegen/boilerplate.go.txt" \
    -O zz_generated.conversion

bash "${CODE_GENERATOR_PATH}/generate-groups.sh" \
    deepcopy \
    github.com/maistra/istio-operator/pkg/generated \
    github.com/maistra/istio-operator/pkg/apis/external/istio \
    "config:v1alpha2 networking:v1alpha3 security:v1beta1" \
    --go-header-file "./build/codegen/boilerplate.go.txt"

bash "${CODE_GENERATOR_PATH}/generate-groups.sh" \
    deepcopy \
    github.com/maistra/istio-operator/pkg/generated \
    github.com/maistra/istio-operator/pkg/apis/external \
    "jaeger:v1 kiali:v1alpha1" \
    --go-header-file "./build/codegen/boilerplate.go.txt"
