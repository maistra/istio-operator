FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ARG build_type=maistra

ADD tmp/_output/bin/istio-operator /usr/local/bin/istio-operator
ADD tmp/_output/templates/ /usr/share/istio-operator/default-templates
ADD manifests-${build_type} /manifests
COPY tmp/_output/helm/istio-releases/istio-1.1.0 /etc/istio-operator/1.1.0/helm
