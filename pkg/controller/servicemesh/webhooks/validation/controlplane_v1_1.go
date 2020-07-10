package validation

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

func (v *ControlPlaneValidator) validateV1_1(ctx context.Context, smcp *maistrav1.ServiceMeshControlPlane) error {
	logger := logf.Log.WithName("smcp-validator")
	var allErrors []error

	if zipkinAddress, ok, _ := unstructured.NestedString(smcp.Spec.Istio, strings.Split("global.tracer.zipkin.address", ".")...); ok && len(zipkinAddress) > 0 {
		tracer, ok, _ := unstructured.NestedString(smcp.Spec.Istio, strings.Split("global.proxy.tracer", ".")...)
		if ok && tracer != "zipkin" {
			// tracer must be "zipkin"
			allErrors = append(allErrors, fmt.Errorf("global.proxy.tracer must equal 'zipkin' if global.tracer.zipkin.address is set"))

		}
		// if an address is set, it must point to the same namespace the SMCP resides in
		addressParts := strings.Split(zipkinAddress, ".")
		if len(addressParts) == 1 {
			allErrors = append(allErrors, fmt.Errorf("global.tracer.zipkin.address must include a namespace"))
		} else if len(addressParts) > 1 {
			namespace := addressParts[1]
			if len(addressParts) == 2 {
				// there might be a port :9411 or similar at the end. make sure to ignore for namespace comparison
				namespacePortParts := strings.Split(namespace, ":")
				namespace = namespacePortParts[0]
			}
			if namespace != smcp.GetObjectMeta().GetNamespace() {
				allErrors = append(allErrors, fmt.Errorf("global.tracer.zipkin.address must point to a service in same namespace as SMCP"))
			}
		}
		if err := errForEnabledValue(smcp.Spec.Istio, "tracing.enabled", true); err != nil {
			// tracing.enabled must be false
			allErrors = append(allErrors, fmt.Errorf("tracing.enabled must be false if global.tracer.zipkin.address is set"))
		}

		if err := errForEnabledValue(smcp.Spec.Istio, "kiali.enabled", true); err != nil {
			if jaegerInClusterURL, ok, _ := unstructured.NestedString(smcp.Spec.Istio, strings.Split("kiali.jaegerInClusterURL", ".")...); !ok || len(jaegerInClusterURL) == 0 {
				allErrors = append(allErrors, fmt.Errorf("kiali.jaegerInClusterURL must be defined if global.tracer.zipkin.address is set"))
			}
		}
	}

	smmr, err := v.getSMMR(smcp)
	if err != nil {
		if !errors.IsNotFound(err) {
			// log error, but don't fail validation: we'll just assume that the control plane namespace is the only namespace for now
			logger.Error(err, "failed to retrieve SMMR for SMCP")
			smmr = nil
		}
	}

	meshNamespaces := common.GetMeshNamespaces(smcp, smmr)
	for _, gateway := range getMapKeys(smcp.Spec.Istio, "gateways") {
		if err := errForStringValue(smcp.Spec.Istio, "gateways."+gateway+".namespace", meshNamespaces); err != nil {
			allErrors = append(allErrors, fmt.Errorf("%v: namespace must be part of the mesh", err))
		}
	}

	return utilerrors.NewAggregate(allErrors)
}
