package validation

import (
	"context"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
)

func (v *ControlPlaneValidator) validateV1_0(ctx context.Context, smcp *maistrav1.ServiceMeshControlPlane) error {
	var allErrors []error

	// sidecarInjectorWebhook.alwaysInjectSelector is being used (values.yaml)
	if err := errForEnabledValue(smcp.Spec.Istio, "global.proxy.alwaysInjectSelector", true); err != nil {
		allErrors = append(allErrors, err)
	}
	// sidecarInjectorWebhook.neverInjectSelector is being used (values.yaml)
	if err := errForEnabledValue(smcp.Spec.Istio, "global.proxy.neverInjectSelector", true); err != nil {
		allErrors = append(allErrors, err)
	}
	// global.proxy.envoyAccessLogService.enabled=true (Envoy ALS enabled)
	if err := errForEnabledValue(smcp.Spec.Istio, "global.proxy.envoyAccessLogService.enabled", true); err != nil {
		allErrors = append(allErrors, err)
	}
	// XXX: i don't think this is supported in the helm charts
	// telemetry.v2.enabled=true (values.yaml, in-proxy metrics)
	if err := errForEnabledValue(smcp.Spec.Istio, "telemetry.enabled", true); err != nil {
		if err := errForEnabledValue(smcp.Spec.Istio, "telemetry.v2.enabled", true); err != nil {
			allErrors = append(allErrors, err)
		}
	}

	return utilerrors.NewAggregate(allErrors)
}
