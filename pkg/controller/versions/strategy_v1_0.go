package versions

import (
	"context"
	"fmt"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

type versionStrategyV1_0 struct {
	version
}

var _ VersionStrategy = (*versionStrategyV1_0)(nil)

func (v *versionStrategyV1_0) SetImageValues(ctx context.Context, cl client.Client, smcpSpec *v1.ControlPlaneSpec) error {
	common.UpdateField(smcpSpec.Istio, "security.image", common.Config.OLM.Images.V1_0.Citadel)
	common.UpdateField(smcpSpec.Istio, "galley.image", common.Config.OLM.Images.V1_0.Galley)
	common.UpdateField(smcpSpec.Istio, "grafana.image", common.Config.OLM.Images.V1_0.Grafana)
	common.UpdateField(smcpSpec.Istio, "mixer.image", common.Config.OLM.Images.V1_0.Mixer)
	common.UpdateField(smcpSpec.Istio, "pilot.image", common.Config.OLM.Images.V1_0.Pilot)
	common.UpdateField(smcpSpec.Istio, "prometheus.image", common.Config.OLM.Images.V1_0.Prometheus)
	common.UpdateField(smcpSpec.Istio, "global.proxy_init.image", common.Config.OLM.Images.V1_0.ProxyInit)
	common.UpdateField(smcpSpec.Istio, "global.proxy.image", common.Config.OLM.Images.V1_0.ProxyV2)
	common.UpdateField(smcpSpec.Istio, "sidecarInjectorWebhook.image", common.Config.OLM.Images.V1_0.SidecarInjector)
	common.UpdateField(smcpSpec.ThreeScale, "image", common.Config.OLM.Images.V1_0.ThreeScale)
	return nil
}
func (v *versionStrategyV1_0) Validate(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
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

	// kiali.jaegerInClusterURL is only supported in v1.1
	if err := errForEnabledValue(smcp.Spec.Istio, "kiali.enabled", true); err != nil {
		if jaegerInClusterURL, ok, _ := smcp.Spec.Istio.GetString("kiali.jaegerInClusterURL"); ok && len(jaegerInClusterURL) > 0 {
			allErrors = append(allErrors, fmt.Errorf("kiali.jaegerInClusterURL is not supported on v1.0 control planes"))
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

func (v *versionStrategyV1_0) ValidateDowngrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	// nothing to downgrade to
	return nil
}

func (v *versionStrategyV1_0) ValidateUpgrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	// nothing to upgrade from
	return nil
}
