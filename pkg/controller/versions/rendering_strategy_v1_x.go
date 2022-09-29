package versions

import (
	"context"
	"fmt"
	"path"

	pkgerrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/helm/pkg/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jaegerv1 "github.com/maistra/istio-operator/pkg/apis/external/jaeger/v1"
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/common/helm"
)

type v1xRenderingStrategy struct{}

func (rs *v1xRenderingStrategy) render(ctx context.Context, v version, cr *common.ControllerResources, cniConfig cni.Config, smcp *v2.ServiceMeshControlPlane) (map[string][]manifest.Manifest, error) {
	log := common.LogFromContext(ctx)
	// Generate the spec
	v1spec := &v1.ControlPlaneSpec{}
	if err := cr.Scheme.Convert(&smcp.Spec, v1spec, nil); err != nil {
		return nil, err
	}
	v1spec.Version = v.String()

	if v1spec.Istio == nil {
		v1spec.Istio = v1.NewHelmValues(make(map[string]interface{}))
	}

	var err error
	smcp.Status.AppliedValues, err = v.ApplyProfiles(ctx, cr, v1spec, smcp.GetNamespace())
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

	// MAISTRA-2014 - external jaeger with v2 resource
	// note, if converted from v1 resource that was already specifying zipkin address and in-cluster url,
	// tracing will already be disabled, so none of this is necessary
	if isComponentEnabled(spec.Istio, "tracing") {
		if provider, _, _ := spec.Istio.GetString("tracing.provider"); provider == "jaeger" {
			// if we're not installing the jaeger resource, we need to determine what has been installed,
			// so control plane rules are created correctly
			jaegerResource, _, _ := spec.Istio.GetString("tracing.jaeger.resourceName")
			if jaegerResource == "" {
				jaegerResource = "jaeger"
			}

			// set the correct zipkin address
			spec.Istio.SetField("global.tracer.zipkin.address", fmt.Sprintf("%s-collector.%s.svc:9411", jaegerResource, smcp.GetNamespace()))

			jaeger := &jaegerv1.Jaeger{}
			jaeger.SetName(jaegerResource)
			jaeger.SetNamespace(smcp.GetNamespace())
			if err := cr.Client.Get(ctx, client.ObjectKey{Name: jaeger.GetName(), Namespace: jaeger.GetNamespace()}, jaeger); err == nil {
				if !metav1.IsControlledBy(jaeger, smcp) {
					// if the resource exists, we never overwrite it
					if err := spec.Istio.SetField("tracing.enabled", false); err != nil {
						return nil, fmt.Errorf("error disabling jaeger install")
					}
					if jaegerInClusterURL, _, _ := spec.Istio.GetString("kiali.jaegerInClusterURL"); jaegerInClusterURL == "" {
						// we won't override any user value
						spec.Istio.SetField("kiali.jaegerInClusterURL", fmt.Sprintf("https://%s-query.%s.svc", jaegerResource, smcp.GetNamespace()))
					}
					if strategy, _, _ := jaeger.Spec.GetString("strategy"); strategy == "" || strategy == "allInOne" {
						spec.Istio.SetField("tracing.jaeger.template", "all-in-one")
					} else {
						// we just want it to not be all-in-one.  see the charts
						spec.Istio.SetField("tracing.jaeger.template", "production-elasticsearch")
					}
				}
			} else if !(errors.IsNotFound(err) || errors.IsGone(err)) {
				if meta.IsNoMatchError(err) {
					return nil, NewDependencyMissingError("Jaeger CRD", err)
				}
				return nil, pkgerrors.Wrapf(err, "error retrieving jaeger resource \"%s/%s\"", smcp.GetNamespace(), jaegerResource)
			} else if err := spec.Istio.SetField("tracing.jaeger.install", true); err != nil {
				return nil, pkgerrors.Wrapf(err, "error enabling jaeger install")
			}
		}
	}

	// convert back to the v2 type
	smcp.Status.AppliedSpec = v2.ControlPlaneSpec{}
	err = cr.Scheme.Convert(&smcp.Status.AppliedValues, &smcp.Status.AppliedSpec, nil)
	if err != nil {
		return nil, fmt.Errorf("Unexpected error setting Status.AppliedSpec: %v", err)
	}

	serverVersion, err := cr.DiscoveryClient.ServerVersion()
	if err != nil {
		return nil, err
	}
	kubeVersion := serverVersion.String()

	// Render the charts
	allErrors := []error{}
	var threeScaleRenderings map[string][]manifest.Manifest
	log.Info("rendering helm charts")
	log.V(2).Info("rendering Istio charts")
	istioRenderings, _, err := helm.RenderChart(path.Join(v.GetChartsDir(), "istio"), smcp.GetNamespace(), kubeVersion, spec.Istio.GetContent())
	if err != nil {
		allErrors = append(allErrors, err)
	}
	if isEnabled(spec.ThreeScale) {
		log.V(2).Info("rendering 3scale charts")
		threeScaleRenderings, _, err = helm.RenderChart(path.Join(v.GetChartsDir(), "maistra-threescale"), smcp.GetNamespace(), kubeVersion, spec.ThreeScale.GetContent())
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
