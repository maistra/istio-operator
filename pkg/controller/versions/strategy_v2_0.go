package versions

import (
	"context"
	"fmt"
	"path"

	"github.com/ghodss/yaml"
	pkgerrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/gengo/examples/set-gen/sets"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/helm"
)

const (
	DiscoveryChart       = "istio-discovery"
	GatewayEgressChart   = "istio-egress"
	GatewayIngressChart  = "istio-ingress"
	GrafanaChart         = "grafana"
	KialiChart           = "kiali"
	MeshConfigChart      = "mesh-config"
	MixerPolicyChart     = "istio-policy"
	MixerTelemetryChart  = "mixer-telemetry"
	PrometheusChart      = "prometheus"
	TelemetryCommonChart = "telemetry-common"
	ThreeScaleChart      = "maistra-threescale"
	TracingChart         = "tracing"
)

type chartRenderingDetails struct {
	path         string
	enabledField string
}

var (
	v2_0ChartMapping = map[string]chartRenderingDetails{
		DiscoveryChart: {
			path:         "istio-control/istio-discovery",
			enabledField: "",
		},
		GatewayIngressChart: {
			path:         "gateways/istio-ingress",
			enabledField: "",
		},
		GatewayEgressChart: {
			path:         "gateways/istio-egress",
			enabledField: "",
		},
		TelemetryCommonChart: {
			path:         "istio-telemetry/telemetry-common",
			enabledField: "",
		},
		MixerTelemetryChart: {
			path:         "istio-telemetry/mixer-telemetry",
			enabledField: "mixer.telemetry",
		},
		PrometheusChart: {
			path:         "istio-telemetry/prometheus",
			enabledField: "prometheus",
		},
		TracingChart: {
			path:         "istio-telemetry/tracing",
			enabledField: "tracing",
		},
		MixerPolicyChart: {
			path:         "istio-policy",
			enabledField: "mixer.policy",
		},
		GrafanaChart: {
			path:         "istio-telemetry/grafana",
			enabledField: "grafana",
		},
		KialiChart: {
			path:         "istio-telemetry/kiali",
			enabledField: "kiali",
		},
		ThreeScaleChart: {
			path:         "maistra-threescale",
			enabledField: "",
		},
		MeshConfigChart: {
			path:         "mesh-config",
			enabledField: "",
		},
	}
)

var specialCharts = sets.NewString(GatewayIngressChart, GatewayEgressChart, ThreeScaleChart)

var v2_0ChartOrder = [][]string{
	{DiscoveryChart},
	{MeshConfigChart},
	{PrometheusChart},
	{MixerPolicyChart, TelemetryCommonChart, MixerTelemetryChart, TracingChart},
	{GrafanaChart},
	{KialiChart},
	{ThreeScaleChart},
}

type versionStrategyV2_0 struct {
	version
}

var _ VersionStrategy = (*versionStrategyV2_0)(nil)

func (v *versionStrategyV2_0) SetImageValues(ctx context.Context, cr *common.ControllerResources, smcpSpec *v1.ControlPlaneSpec) error {
	common.UpdateField(smcpSpec.Istio, "grafana.image", common.Config.OLM.Images.V2_0.Grafana)
	common.UpdateField(smcpSpec.Istio, "mixer.image", common.Config.OLM.Images.V2_0.Mixer)
	common.UpdateField(smcpSpec.Istio, "pilot.image", common.Config.OLM.Images.V2_0.Pilot)
	common.UpdateField(smcpSpec.Istio, "prometheus.image", common.Config.OLM.Images.V2_0.Prometheus)
	common.UpdateField(smcpSpec.Istio, "global.proxy_init.image", common.Config.OLM.Images.V2_0.ProxyInit)
	common.UpdateField(smcpSpec.Istio, "global.proxy.image", common.Config.OLM.Images.V2_0.ProxyV2)
	common.UpdateField(smcpSpec.ThreeScale, "image", common.Config.OLM.Images.V2_0.ThreeScale)

	common.UpdateField(smcpSpec.Istio, "gateways.istio-ingressgateway.ior_image", common.Config.OLM.Images.V2_0.IOR)
	return nil
}

func (v *versionStrategyV2_0) Validate(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	// TODO: XXX
	return V1_1.Strategy().Validate(ctx, cl, smcp)
}

func (v *versionStrategyV2_0) ValidateDowngrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	// TODO: XXX
	return nil
}

func (v *versionStrategyV2_0) ValidateUpgrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	// TODO: XXX
	return nil
}

func (v *versionStrategyV2_0) GetChartInstallOrder() [][]string {
	return v2_0ChartOrder
}

func (v *versionStrategyV2_0) Render(ctx context.Context, cr *common.ControllerResources, smcp *v2.ServiceMeshControlPlane) (map[string][]manifest.Manifest, error) {
	log := common.LogFromContext(ctx)
	//Generate the spec
	// XXX: we should apply v2 templates first, then convert to values.yaml (v1)
	v1spec := &v1.ControlPlaneSpec{}
	if err := cr.Scheme.Convert(&smcp.Spec, v1spec, nil); err != nil {
		return nil, err
	}
	v1spec.Version = v.String()

	var err error
	smcp.Status.LastAppliedConfiguration, err = v.applyTemplates(ctx, cr, *v1spec)
	if err != nil {
		log.Error(err, "warning: failed to apply ServiceMeshControlPlane templates")

		return nil, err
	}

	spec := &smcp.Status.LastAppliedConfiguration

	if spec.Istio == nil {
		spec.Istio = v1.NewHelmValues(nil)
	}

	if spec.ThreeScale == nil {
		spec.ThreeScale = v1.NewHelmValues(nil)
	}

	err = spec.Istio.SetField("global.operatorNamespace", common.GetOperatorNamespace())
	if err != nil {
		return nil, err
	}

	err = spec.Istio.SetField("istio_cni.istio_cni_network", v.GetCNINetworkName())
	if err != nil {
		return nil, fmt.Errorf("Could not set field status.lastAppliedConfiguration.istio.istio_cni.istio_cni_network: %v", err)
	}

	// XXX: using values.yaml settings, as things may have been overridden in profiles/templates
	if isComponentEnabled(spec.Istio, v2_0ChartMapping[TracingChart].enabledField) {
		// if we're not installing the jaeger resource, we need to determine what has been installed,
		// so control plane rules are created correctly
		if provider, _, _ := spec.Istio.GetString("tracing.provider"); provider == "jaeger" {
			if install, _, _ := spec.Istio.GetBool("tracing.jaeger.install"); !install {
				jaegerResource, _, _ := spec.Istio.GetString("tracing.jaeger.resourceName")
				if jaegerResource == "" {
					jaegerResource = "jaeger"
				}
				jaeger := &unstructured.Unstructured{}
				jaeger.SetGroupVersionKind(schema.GroupVersionKind{Group: "jaegertracing.io", Kind: "Jaeger", Version: "v1"})
				jaeger.SetName(jaegerResource)
				jaeger.SetNamespace(smcp.GetNamespace())
				if err := cr.Client.Get(ctx, client.ObjectKey{Name: jaeger.GetName(), Namespace: jaeger.GetNamespace()}, jaeger); err == nil {
					if strategy, _, _ := unstructured.NestedString(jaeger.UnstructuredContent(), "spec", "strategy"); strategy == "" || strategy == "allInOne" {
						spec.Istio.SetField("tracing.jaeger.template", "all-in-one")
					} else {
						// we just want it to not be all-in-one.  see the charts
						spec.Istio.SetField("tracing.jaeger.template", "production-elasticsearch")
					}
				} else if !(errors.IsNotFound(err) || errors.IsGone(err)) {
					return nil, pkgerrors.Wrapf(err, "error retrieving jaeger resource \"%s/%s\"", smcp.GetNamespace(), jaegerResource)
				}
			}
		}
	}

	// Read in global.yaml
	values, err := chartutil.ReadValuesFile(path.Join(v.GetChartsDir(), "global.yaml"))
	if err != nil {
		return nil, fmt.Errorf("error reading global.yaml file")
	}
	values.MergeInto(spec.Istio.GetContent())
	if log.V(5).Enabled() {
		rawValues, _ := yaml.Marshal(values)
		log.V(5).Info(fmt.Sprintf("rendering values:\n%s", string(rawValues)))
	}

	//Render the charts
	allErrors := []error{}
	renderings := make(map[string][]manifest.Manifest)
	log.Info("rendering helm charts")
	for name, chartDetails := range v2_0ChartMapping {
		if specialCharts.Has(name) {
			continue
		}
		if chartDetails.enabledField == "" || isComponentEnabled(spec.Istio, chartDetails.enabledField) {
			log.V(2).Info(fmt.Sprintf("rendering %s chart", name))
			if chartRenderings, _, err := helm.RenderChart(path.Join(v.GetChartsDir(), v2_0ChartMapping[name].path), smcp.GetNamespace(), values); err == nil {
				renderings[name] = chartRenderings[name]
			} else {
				allErrors = append(allErrors, err)
			}
		} else {
			log.V(2).Info(fmt.Sprintf("skipping disabled %s chart", name))
		}
	}

	log.V(2).Info("rendering gateways charts")
	if origGateways, ok := values.AsMap()["gateways"]; ok {
		if origGatewaysMap, ok := origGateways.(map[string]interface{}); ok {
			log.V(2).Info("rendering ingress gateway chart for istio-ingressgateway")
			if ingressRenderings, _, err := v.renderIngressGateway("istio-ingressgateway", smcp.GetNamespace(), origGatewaysMap, v1.NewHelmValues(values)); err == nil {
				renderings[GatewayIngressChart] = ingressRenderings[GatewayIngressChart]
			} else {
				allErrors = append(allErrors, err)
			}
			log.V(2).Info("rendering egress gateway chart for istio-egressgateway")
			if egressRenderings, _, err := v.renderEgressGateway("istio-egressgateway", smcp.GetNamespace(), origGatewaysMap, v1.NewHelmValues(values)); err == nil {
				renderings[GatewayEgressChart] = egressRenderings[GatewayEgressChart]
			} else {
				allErrors = append(allErrors, err)
			}
			if smcp.Spec.Gateways != nil {
				for name, gateway := range smcp.Spec.Gateways.IngressGateways {
					if gateway.Enabled == nil || !*gateway.Enabled {
						continue
					}
					log.V(2).Info(fmt.Sprintf("rendering ingress gateway chart for %s", name))
					if ingressRenderings, _, err := v.renderIngressGateway(name, smcp.GetNamespace(), origGatewaysMap, v1.NewHelmValues(values)); err == nil {
						renderings[GatewayIngressChart] = append(renderings[GatewayIngressChart], ingressRenderings[GatewayIngressChart]...)
					} else {
						allErrors = append(allErrors, err)
					}
				}
				for name, gateway := range smcp.Spec.Gateways.EgressGateways {
					if gateway.Enabled == nil || !*gateway.Enabled {
						continue
					}
					log.V(2).Info(fmt.Sprintf("rendering egress gateway chart for %s", name))
					if egressRenderings, _, err := v.renderEgressGateway(name, smcp.GetNamespace(), origGatewaysMap, v1.NewHelmValues(values)); err == nil {
						renderings[GatewayEgressChart] = append(renderings[GatewayEgressChart], egressRenderings[GatewayEgressChart]...)
					} else {
						allErrors = append(allErrors, err)
					}
				}
			}
			v1.NewHelmValues(values).SetField("gateways", origGateways)
		}
	} else {
		allErrors = append(allErrors, fmt.Errorf("error retrieving values for gateways charts"))
	}

	if isEnabled(spec.ThreeScale) {
		log.V(2).Info("rendering 3scale charts")
		if chartRenderings, _, err := helm.RenderChart(path.Join(v.GetChartsDir(), v2_0ChartMapping[ThreeScaleChart].path), smcp.GetNamespace(), spec.ThreeScale.GetContent()); err == nil {
			renderings[ThreeScaleChart] = chartRenderings[ThreeScaleChart]
		} else {
			allErrors = append(allErrors, err)
		}
	}

	if len(allErrors) > 0 {
		return nil, utilerrors.NewAggregate(allErrors)
	}

	return renderings, nil
}

func (v *versionStrategyV2_0) renderIngressGateway(name string, namespace string, gateways map[string]interface{}, values *v1.HelmValues) (map[string][]manifest.Manifest, map[string]interface{}, error) {
	return v.renderGateway(name, namespace, v2_0ChartMapping[GatewayIngressChart].path, "istio-ingressgateway", gateways, values)
}

func (v *versionStrategyV2_0) renderEgressGateway(name string, namespace string, gateways map[string]interface{}, values *v1.HelmValues) (map[string][]manifest.Manifest, map[string]interface{}, error) {
	return v.renderGateway(name, namespace, v2_0ChartMapping[GatewayEgressChart].path, "istio-egressgateway", gateways, values)
}

func (v *versionStrategyV2_0) renderGateway(name string, namespace string, chartPath string, typeName string, gateways map[string]interface{}, values *v1.HelmValues) (map[string][]manifest.Manifest, map[string]interface{}, error) {
	gateway, ok, _ := unstructured.NestedMap(gateways, name)
	if !ok {
		// XXX: return an error?
		return map[string][]manifest.Manifest{}, nil, nil
	}
	if enabled, ok, _ := unstructured.NestedBool(gateway, "enabled"); !(ok && enabled) {
		// XXX: return an error?
		return map[string][]manifest.Manifest{}, nil, nil
	}
	newGateways := make(map[string]interface{})
	newGateways["revision"] = gateways["revision"]
	newGateways[typeName] = gateway
	if err := values.SetField("gateways", newGateways); err != nil {
		return nil, nil, err
	}
	return helm.RenderChart(path.Join(v.GetChartsDir(), chartPath), namespace, values)
}
