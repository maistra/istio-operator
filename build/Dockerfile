FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

ARG build_type=maistra

ADD tmp/_output/bin/istio-operator /usr/local/bin/istio-operator
ADD tmp/_output/resources/default-templates/ /usr/local/share/istio-operator/default-templates
ADD manifests-${build_type} /manifests
COPY tmp/_output/resources/helm/ /usr/local/share/istio-operator/helm/
