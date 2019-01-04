FROM centos:7

RUN adduser istio-operator
USER istio-operator

ADD tmp/_output/bin/istio-operator /usr/local/bin/istio-operator
COPY tmp/_output/helm/istio-releases/istio-1.1.0 /etc/istio-operator/1.1.0/helm
