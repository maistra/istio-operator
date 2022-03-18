package versions

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/helm/pkg/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
)

// these components have to be installed in the specified order
var v1_0ChartOrder = [][]string{
	{"istio"}, // core istio resources
	{"istio/charts/security"},
	{"istio/charts/prometheus"},
	{"istio/charts/tracing"},
	{"istio/charts/galley"},
	{"istio/charts/mixer", "istio/charts/pilot", "istio/charts/gateways", "istio/charts/sidecarInjectorWebhook"},
	{"istio/charts/grafana"},
	{"istio/charts/kiali"},
}

type versionStrategyV1_0 struct {
	version
	renderImpl     v1xRenderingStrategy
	conversionImpl v1xConversionStrategy
}

var _ VersionStrategy = (*versionStrategyV1_0)(nil)

func (v *versionStrategyV1_0) SetImageValues(ctx context.Context, cr *common.ControllerResources, smcpSpec *v1.ControlPlaneSpec) error {
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
func (v *versionStrategyV1_0) ValidateV1(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
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
	// Istiod not supported
	if err := errForValue(smcp.Spec.Istio, "policy.implementation", string(v2.PolicyTypeIstiod)); err != nil {
		allErrors = append(allErrors, err)
	}
	if err := errForValue(smcp.Spec.Istio, "telemetry.implementation", string(v2.PolicyTypeIstiod)); err != nil {
		allErrors = append(allErrors, err)
	}
	// XXX: i don't think this is supported in the helm charts
	// telemetry.v2.enabled=true (values.yaml, in-proxy metrics)
	if err := errForEnabledValue(smcp.Spec.Istio, "telemetry.enabled", true); err != nil {
		if err := errForEnabledValue(smcp.Spec.Istio, "telemetry.v2.enabled", true); err != nil {
			allErrors = append(allErrors, err)
		}
	}
	// global.proxy.envoyAccessLogService.enabled=true (Envoy ALS enabled)
	if err := errForEnabledValue(smcp.Spec.Istio, "global.proxy.envoyAccessLogService.enabled", true); err != nil {
		allErrors = append(allErrors, err)
	}

	// kiali.jaegerInClusterURL is only supported in v1.1
	if err := errForEnabledValue(smcp.Spec.Istio, "kiali.enabled", true); err != nil {
		if jaegerInClusterURL, ok, _ := smcp.Spec.Istio.GetString("kiali.jaegerInClusterURL"); ok && len(jaegerInClusterURL) > 0 {
			allErrors = append(allErrors, fmt.Errorf("kiali.jaegerInClusterURL is not supported on v1.0 control planes"))
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

func (v *versionStrategyV1_0) ValidateV2(ctx context.Context, cl client.Client, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec) error {
	var allErrors []error
	allErrors = validatePolicyType(nil, meta, spec, v.version, allErrors)
	allErrors = validateTelemetryType(nil, meta, spec, v.version, allErrors)
	return NewValidationError(allErrors...)
}

func (v *versionStrategyV1_0) ValidateDowngrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	// nothing to downgrade to
	return nil
}

func (v *versionStrategyV1_0) ValidateUpgrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	// nothing to upgrade from
	return nil
}

func (v *versionStrategyV1_0) GetChartInstallOrder() [][]string {
	return v1_0ChartOrder
}

func (v *versionStrategyV1_0) Render(ctx context.Context, cr *common.ControllerResources, cniConfig cni.Config, smcp *v2.ServiceMeshControlPlane) (map[string][]manifest.Manifest, error) {
	return v.renderImpl.render(ctx, v.version, cr, cniConfig, smcp)
}

func (v *versionStrategyV1_0) GetExpansionPorts() []corev1.ServicePort {
	return v.conversionImpl.GetExpansionPorts()
}

func (v *versionStrategyV1_0) GetTelemetryType(in *v1.HelmValues, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled bool) v2.TelemetryType {
	return v.conversionImpl.GetTelemetryType(in, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled)
}

func (v *versionStrategyV1_0) GetPolicyType(in *v1.HelmValues, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled bool) v2.PolicyType {
	return v.conversionImpl.GetPolicyType(in, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled)
}

func (v *versionStrategyV1_0) GetTrustDomainFieldPath() string {
	return v.conversionImpl.GetTrustDomainFieldPath()
}
