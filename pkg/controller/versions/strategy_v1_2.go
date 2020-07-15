package versions

import (
	"context"
	"fmt"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"k8s.io/helm/pkg/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type versionStrategyV1_2 struct {
	version
}

var _ VersionStrategy = (*versionStrategyV1_2)(nil)

func (v *versionStrategyV1_2) SetImageValues(ctx context.Context, cr *common.ControllerResources, smcpSpec *v1.ControlPlaneSpec) error {
	common.UpdateField(smcpSpec.Istio, "security.image", common.Config.OLM.Images.V1_2.Citadel)
	common.UpdateField(smcpSpec.Istio, "galley.image", common.Config.OLM.Images.V1_2.Galley)
	common.UpdateField(smcpSpec.Istio, "grafana.image", common.Config.OLM.Images.V1_2.Grafana)
	common.UpdateField(smcpSpec.Istio, "mixer.image", common.Config.OLM.Images.V1_2.Mixer)
	common.UpdateField(smcpSpec.Istio, "pilot.image", common.Config.OLM.Images.V1_2.Pilot)
	common.UpdateField(smcpSpec.Istio, "prometheus.image", common.Config.OLM.Images.V1_2.Prometheus)
	common.UpdateField(smcpSpec.Istio, "global.proxy_init.image", common.Config.OLM.Images.V1_2.ProxyInit)
	common.UpdateField(smcpSpec.Istio, "global.proxy.image", common.Config.OLM.Images.V1_2.ProxyV2)
	common.UpdateField(smcpSpec.Istio, "sidecarInjectorWebhook.image", common.Config.OLM.Images.V1_2.SidecarInjector)
	common.UpdateField(smcpSpec.ThreeScale, "image", common.Config.OLM.Images.V1_2.ThreeScale)

	common.UpdateField(smcpSpec.Istio, "gateways.istio-ingressgateway.ior_image", common.Config.OLM.Images.V1_2.IOR)
return nil
}
func (v *versionStrategyV1_2) Validate(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
    // TODO: XXX
	return V1_1.Strategy().Validate(ctx, cl, smcp)
}
func (v *versionStrategyV1_2) ValidateDowngrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
    // TODO: XXX
	return nil
}
func (v *versionStrategyV1_2) ValidateUpgrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
    // TODO: XXX
	return nil
}

func (v *versionStrategyV1_2) Render(ctx context.Context, cr *common.ControllerResources, smcp *v2.ServiceMeshControlPlane) (map[string][]manifest.Manifest, error) {
	return nil, fmt.Errorf("implement me")
}
