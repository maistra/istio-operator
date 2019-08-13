FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ARG build_type=maistra

ADD tmp/_output/bin/istio-operator /usr/local/bin/istio-operator
ADD tmp/_output/resources/default-templates/ /usr/local/share/istio-operator/default-templates
ADD manifests-${build_type} /manifests
COPY tmp/_output/helm/istio-releases/istio-1.1.0 /usr/local/share/istio-operator/helm/1.1.0
