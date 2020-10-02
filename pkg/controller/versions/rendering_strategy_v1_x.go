package versions

import (
	"context"
	"fmt"
	"path"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/common/helm"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/helm/pkg/manifest"
)

type v1xRenderingStrategy struct{}

func (rs *v1xRenderingStrategy) render(ctx context.Context, v version, cr *common.ControllerResources, cniConfig cni.Config, smcp *v2.ServiceMeshControlPlane) (map[string][]manifest.Manifest, error) {
	log := common.LogFromContext(ctx)
	//Generate the spec
	v1spec := &v1.ControlPlaneSpec{}
	if err := cr.Scheme.Convert(&smcp.Spec, v1spec, nil); err != nil {
		return nil, err
	}
	v1spec.Version = v.String()

	if v1spec.Istio == nil {
		v1spec.Istio = v1.NewHelmValues(make(map[string]interface{}))
	}

	var err error
	smcp.Status.AppliedValues, err = v.applyProfiles(ctx, cr, v1spec, smcp.GetNamespace())
	if err != nil {
		log.Error(err, "warning: failed to apply ServiceMeshControlPlane templates")

		return nil, err
	}

	spec := &smcp.Status.AppliedValues

	if spec.ThreeScale == nil {
		spec.ThreeScale = v1.NewHelmValues(make(map[string]interface{}))
	}

	err = spec.Istio.SetField("global.operatorNamespace", common.GetOperatorNamespace())
	if err != nil {
		return nil, err
	}

	err = spec.Istio.SetField("istio_cni.enabled", cniConfig.Enabled)
	if err != nil {
		return nil, fmt.Errorf("Could not set field status.lastAppliedConfiguration.istio.istio_cni.enabled: %v", err)
	}
	err = spec.Istio.SetField("istio_cni.istio_cni_network", v.GetCNINetworkName())
	if err != nil {
		return nil, fmt.Errorf("Could not set field status.lastAppliedConfiguration.istio.istio_cni.istio_cni_network: %v", err)
	}

	// MAISTRA-1330
	err = spec.Istio.SetField("global.istioNamespace", smcp.GetNamespace())
	if err != nil {
		return nil, fmt.Errorf("Could not set field status.lastAppliedConfiguration.istio.global.istioNamespace: %v", err)
	}

	// convert back to the v2 type
	smcp.Status.AppliedSpec = v2.ControlPlaneSpec{}
	err = cr.Scheme.Convert(&smcp.Status.AppliedValues, &smcp.Status.AppliedSpec, nil)
	if err != nil {
		return nil, fmt.Errorf("Unexpected error setting Status.AppliedSpec: %v", err)
	}

	//Render the charts
	allErrors := []error{}
	var threeScaleRenderings map[string][]manifest.Manifest
	log.Info("rendering helm charts")
	log.V(2).Info("rendering Istio charts")
	istioRenderings, _, err := helm.RenderChart(path.Join(v.GetChartsDir(), "istio"), smcp.GetNamespace(), spec.Istio.GetContent())
	if err != nil {
		allErrors = append(allErrors, err)
	}
	if isEnabled(spec.ThreeScale) {
		log.V(2).Info("rendering 3scale charts")
		threeScaleRenderings, _, err = helm.RenderChart(path.Join(v.GetChartsDir(), "maistra-threescale"), smcp.GetNamespace(), spec.ThreeScale.GetContent())
		if err != nil {
			allErrors = append(allErrors, err)
		}
	} else {
		threeScaleRenderings = map[string][]manifest.Manifest{}
	}

	if len(allErrors) > 0 {
		return nil, utilerrors.NewAggregate(allErrors)
	}

	// merge the rendernings
	renderings := map[string][]manifest.Manifest{}
	for key, value := range istioRenderings {
		renderings[key] = value
	}
	for key, value := range threeScaleRenderings {
		renderings[key] = value
	}
	return renderings, nil
}
