#!/bin/sh

oc patch kiali kiali -n istio-system -p '{"metadata":{"finalizers": []}}' --type=merge

oc delete -f ../maistra_v1_servicemeshmemberroll_cr.yaml -n istio-system
oc delete -f ../maistra_v1_servicemeshcontrolplane_cr_basic.yaml -n istio-system
oc delete -f ../../maistra-operator.yaml -n istio-operator
oc delete -f kiali-operator.yaml -n kiali-operator

oc delete namespace istio-system
oc delete namespace istio-operator
oc delete namespace kiali-operator
