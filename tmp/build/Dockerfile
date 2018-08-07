FROM alpine:3.6

RUN adduser -D istio-operator
USER istio-operator

ADD tmp/_output/bin/istio-operator /usr/local/bin/istio-operator
