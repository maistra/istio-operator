FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ADD bin/istio-operator /usr/local/bin/istio-operator
ADD manifests /manifests
COPY helm /etc/istio-operator/1.1.0/helm
