package versions

import (
	"context"
	"fmt"
	"path"

	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	jaegerv1 "github.com/maistra/istio-operator/pkg/apis/external/jaeger/v1"
	kialiv1alpha1 "github.com/maistra/istio-operator/pkg/apis/external/kiali/v1alpha1"
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/common/helm"
)

var v2_1ChartMapping = map[string]chartRenderingDetails{
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
	WASMExtensionsChart: {
		path:         "wasm-extensions",
		enabledField: "wasmExtensions",
	},
	RLSChart: {
		path:         "rls",
		enabledField: "rateLimiting.rls",
	},
}

var v2_1ChartOrder = [][]string{
	{DiscoveryChart},
	{MeshConfigChart},
	{TelemetryCommonChart, PrometheusChart},
	{MixerPolicyChart, MixerTelemetryChart, TracingChart, GatewayIngressChart, GatewayEgressChart, GrafanaChart},
	{KialiChart},
	{ThreeScaleChart, WASMExtensionsChart, RLSChart},
}

type versionStrategyV2_1 struct {
	Ver
	conversionImpl v2xConversionStrategy
}

var _ VersionStrategy = (*versionStrategyV2_1)(nil)

func (v *versionStrategyV2_1) SetImageValues(ctx context.Context, cr *common.ControllerResources, smcpSpec *v1.ControlPlaneSpec) error {
	if err := common.UpdateField(smcpSpec.Istio, "grafana.image", common.Config.OLM.Images.V2_1.Grafana); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "pilot.image", common.Config.OLM.Images.V2_1.Pilot); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "prometheus.image", common.Config.OLM.Images.V2_1.Prometheus); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "global.proxy_init.image", common.Config.OLM.Images.V2_1.ProxyInit); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "global.proxy.image", common.Config.OLM.Images.V2_1.ProxyV2); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "wasmExtensions.cacher.image", common.Config.OLM.Images.V2_1.WASMCacher); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "rateLimiting.rls.image", common.Config.OLM.Images.V2_1.RLS); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.ThreeScale, "image", common.Config.OLM.Images.V2_1.ThreeScale); err != nil {
		return err
	}
	return nil
}

func (v *versionStrategyV2_1) IsClusterScoped(spec *v2.ControlPlaneSpec) (bool, error) {
	return false, nil
}

func (v *versionStrategyV2_1) ValidateV1(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	return fmt.Errorf("must use v2 ServiceMeshControlPlane resource for v2.0+ installations")
}

func (v *versionStrategyV2_1) ValidateV2(ctx context.Context, cl client.Client, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec) error {
	var allErrors []error
	allErrors = v.validateGlobal(spec, allErrors)
	allErrors = validateGateways(ctx, meta, spec, cl, allErrors)
	allErrors = validatePolicyType(spec, v.Ver, allErrors)
	allErrors = validateTelemetryType(spec, v.Ver, allErrors)
	allErrors = v.validateProtocolDetection(spec, allErrors)
	allErrors = v.validateRuntime(spec, allErrors)
	allErrors = v.validateMixerDisabled(spec, allErrors)
	allErrors = v.validateAddons(spec, allErrors)
	return NewValidationError(allErrors...)
}

func (v *versionStrategyV2_1) validateProtocolDetection(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.Proxy == nil || spec.Proxy.Networking == nil || spec.Proxy.Networking.Protocol == nil || spec.Proxy.Networking.Protocol.AutoDetect == nil {
		return allErrors
	}
	autoDetect := spec.Proxy.Networking.Protocol.AutoDetect
	if autoDetect.Inbound != nil && *autoDetect.Inbound {
		allErrors = append(allErrors, fmt.Errorf("automatic protocol detection is not supported in %s; "+
			"if specified, spec.proxy.networking.protocol.autoDetect.inbound must be set to false", v.String()))
	}
	if autoDetect.Outbound != nil && *autoDetect.Outbound {
		allErrors = append(allErrors, fmt.Errorf("automatic protocol detection is not supported in %s; "+
			"if specified, spec.proxy.networking.protocol.autoDetect.outbound must be set to false", v.String()))
	}
	return allErrors
}

func (v *versionStrategyV2_1) validateRuntime(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.Runtime == nil || spec.Runtime.Components == nil {
		return allErrors
	}
	for component, config := range spec.Runtime.Components {
		if config.Pod == nil || config.Pod.Affinity == nil {
			continue
		}
		if component == v2.ControlPlaneComponentNameKiali {
			if config.Pod.Affinity.PodAntiAffinity.RequiredDuringScheduling != nil || config.Pod.Affinity.PodAntiAffinity.PreferredDuringScheduling != nil {
				allErrors = append(allErrors, fmt.Errorf("PodAntiAffinity configured via "+
					".spec.runtime.components.pod.affinity.podAntiAffinity.requiredDuringScheduling "+
					"and preferredDuringScheduling is not supported for the %q component", v2.ControlPlaneComponentNameKiali))
			}
		} else {
			if config.Pod.Affinity.NodeAffinity != nil {
				allErrors = append(allErrors, fmt.Errorf("nodeAffinity is only supported for the %q component", v2.ControlPlaneComponentNameKiali))
			}
			if config.Pod.Affinity.PodAffinity != nil {
				allErrors = append(allErrors, fmt.Errorf("podAffinity is only supported for the %q component", v2.ControlPlaneComponentNameKiali))
			}
			if config.Pod.Affinity.PodAntiAffinity.PodAntiAffinity != nil {
				allErrors = append(allErrors, fmt.Errorf("podAntiAffinity configured via "+
					".spec.runtime.components.pod.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution "+
					"and preferredDuringSchedulingIgnoredDuringExecution is only supported for the %q component", v2.ControlPlaneComponentNameKiali))
			}
		}
	}
	return allErrors
}

func (v *versionStrategyV2_1) validateMixerDisabled(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.Policy != nil && (spec.Policy.Type == v2.PolicyTypeMixer || spec.Policy.Mixer != nil) {
		allErrors = append(allErrors, fmt.Errorf("support for policy.type %q and policy.Mixer options have been removed in v2.1, "+
			"please use another alternative", v2.PolicyTypeMixer))
	}
	if spec.Telemetry != nil && (spec.Telemetry.Type == v2.TelemetryTypeMixer || spec.Telemetry.Mixer != nil) {
		allErrors = append(allErrors, fmt.Errorf("support for telemetry.type %q and telemetry.Mixer options have been removed in v2.1, "+
			"please use another alternative", v2.TelemetryTypeMixer))
	}
	return allErrors
}

func (v *versionStrategyV2_1) validateAddons(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.Addons == nil {
		return allErrors
	}

	if spec.Addons.ThreeScale != nil {
		allErrors = append(allErrors, fmt.Errorf("support for 3scale has been removed in v2.1; "+
			"please remove the spec.addons.3scale section from the SMCP and configure the 3scale WebAssembly adapter using a ServiceMeshExtension resource"))
	}
	return allErrors
}

func (v *versionStrategyV2_1) ValidateV2Full(ctx context.Context, cl client.Client, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec) error {
	var allErrors []error
	err := v.ValidateV2(ctx, cl, meta, spec)
	if err != nil {
		if validationErr, ok := err.(ValidationError); ok {
			allErrors = validationErr.Errors()
		} else {
			return err
		}
	}
	// additional validation checks that are only performed just before reconciliation
	allErrors = validatePrometheusEnabledWhenKialiEnabled(spec, allErrors)
	return NewValidationError(allErrors...)
}

func (v *versionStrategyV2_1) ValidateDowngrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	// TODO: what might prevent us from downgrading?
	return nil
}

func (v *versionStrategyV2_1) ValidateUpgrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	// TODO: what might prevent us from upgrading?
	return nil
}

func (v *versionStrategyV2_1) ValidateUpdate(ctx context.Context, cl client.Client, oldSMCP, newSMCP metav1.Object) error {
	return nil
}

func (v *versionStrategyV2_1) ValidateRequest(ctx context.Context, cl client.Client, req admission.Request, smcp metav1.Object) admission.Response {
	return admission.ValidationResponse(true, "")
}

func (v *versionStrategyV2_1) GetChartInstallOrder() [][]string {
	return v2_1ChartOrder
}

// TODO: consider consolidating this with 2.0 rendering logic
func (v *versionStrategyV2_1) Render(ctx context.Context, cr *common.ControllerResources, cniConfig cni.Config,
	smcp *v2.ServiceMeshControlPlane,
) (map[string][]manifest.Manifest, error) {
	log := common.LogFromContext(ctx)
	// Generate the spec
	// XXX: we should apply v2 templates first, then convert to values.yaml (v1)
	v1smcp := &v1.ServiceMeshControlPlane{}
	if err := v1smcp.ConvertFrom(smcp); err != nil {
		return nil, err
	}
	v1spec := &v1smcp.Spec
	v1spec.Version = v.String()

	if v1spec.Istio == nil {
		v1spec.Istio = v1.NewHelmValues(make(map[string]interface{}))
	}

	var err error
	smcp.Status.AppliedValues, err = v.ApplyProfiles(ctx, cr, v1spec, smcp.GetNamespace())
	if err != nil {
		log.Error(err, "warning: failed to apply ServiceMeshControlPlane profiles")

		return nil, err
	}

	spec := &smcp.Status.AppliedValues

	if spec.ThreeScale == nil {
		spec.ThreeScale = v1.NewHelmValues(make(map[string]interface{}))
	}

	err = spec.Istio.SetField("revision", smcp.GetName())
	if err != nil {
		return nil, err
	}

	err = spec.Istio.SetField("istio_cni.enabled", cniConfig.Enabled)
	if err != nil {
		return nil, fmt.Errorf("could not set field status.lastAppliedConfiguration.istio.istio_cni.enabled: %v", err)
	}
	err = spec.Istio.SetField("istio_cni.istio_cni_network", v.GetCNINetworkName())
	if err != nil {
		return nil, fmt.Errorf("could not set field status.lastAppliedConfiguration.istio.istio_cni.istio_cni_network: %v", err)
	}

	// Override these globals to match the install namespace
	err = spec.Istio.SetField("global.istioNamespace", smcp.GetNamespace())
	if err != nil {
		return nil, fmt.Errorf("could not set field status.lastAppliedConfiguration.istio.global.istioNamespace: %v", err)
	}
	err = spec.Istio.SetField("meshConfig.rootNamespace", smcp.GetNamespace())
	if err != nil {
		return nil, fmt.Errorf("could not set field status.lastAppliedConfiguration.istio.meshConfig.rootNamespace: %v", err)
	}
	err = spec.Istio.SetField("global.prometheusNamespace", smcp.GetNamespace())
	if err != nil {
		return nil, fmt.Errorf("could not set field status.lastAppliedConfiguration.istio.global.prometheusNamespace: %v", err)
	}
	err = spec.Istio.SetField("global.configRootNamespace", smcp.GetNamespace())
	if err != nil {
		return nil, fmt.Errorf("could not set field status.lastAppliedConfiguration.istio.global.configRootNamespace: %v", err)
	}
	err = spec.Istio.SetField("global.configNamespace", smcp.GetNamespace())
	if err != nil {
		return nil, fmt.Errorf("could not set field status.lastAppliedConfiguration.istio.global.configNamespace: %v", err)
	}
	err = spec.Istio.SetField("meshConfig.ingressControllerMode", "OFF")
	if err != nil {
		return nil, fmt.Errorf("could not set field meshConfig.ingressControllerMode: %v", err)
	}

	// XXX: using values.yaml settings, as things may have been overridden in profiles/templates
	if isComponentEnabled(spec.Istio, v2_1ChartMapping[TracingChart].enabledField) {
		if provider, _, _ := spec.Istio.GetString("tracing.provider"); provider == "jaeger" {
			// if we're not installing the jaeger resource, we need to determine what has been installed,
			// so control plane rules are created correctly
			jaegerResource, _, _ := spec.Istio.GetString("tracing.jaeger.resourceName")
			if jaegerResource == "" {
				jaegerResource = "jaeger"
			}

			// set the correct zipkin address
			err = spec.Istio.SetField("meshConfig.defaultConfig.tracing.zipkin.address", fmt.Sprintf("%s-collector.%s.svc:9411", jaegerResource, smcp.GetNamespace()))
			if err != nil {
				return nil, fmt.Errorf("could not set field istio.meshConfig.defaultConfig.tracing.zipkin.address: %v", err)
			}

			jaeger := &jaegerv1.Jaeger{}
			jaeger.SetName(jaegerResource)
			jaeger.SetNamespace(smcp.GetNamespace())
			if err := cr.Client.Get(ctx, client.ObjectKey{Name: jaeger.GetName(), Namespace: jaeger.GetNamespace()}, jaeger); err == nil {
				if metav1.IsControlledBy(jaeger, smcp) {
					// we're managing this install, so we'll update it
					if err := spec.Istio.SetField("tracing.jaeger.install", true); err != nil {
						return nil, fmt.Errorf("error enabling jaeger install")
					}
				} else {
					// if the resource exists, we never overwrite it
					if err := spec.Istio.SetField("tracing.jaeger.install", false); err != nil {
						return nil, fmt.Errorf("error disabling jaeger install")
					}
					if strategy, _, _ := jaeger.Spec.GetString("strategy"); strategy == "" || strategy == "allInOne" {
						if err := spec.Istio.SetField("tracing.jaeger.template", "all-in-one"); err != nil {
							return nil, err
						}
					} else {
						// we just want it to not be all-in-one.  see the charts
						if err := spec.Istio.SetField("tracing.jaeger.template", "production-elasticsearch"); err != nil {
							return nil, err
						}
					}
				}
			} else if !errors.IsNotFound(err) {
				if meta.IsNoMatchError(err) {
					return nil, NewDependencyMissingError("Jaeger CRD", err)
				}
				return nil, pkgerrors.Wrapf(err, "error retrieving jaeger resource \"%s/%s\"", smcp.GetNamespace(), jaegerResource)
			} else if err := spec.Istio.SetField("tracing.jaeger.install", true); err != nil {
				return nil, pkgerrors.Wrapf(err, "error enabling jaeger install")
			}
		}
	}
	if isComponentEnabled(spec.Istio, v2_1ChartMapping[KialiChart].enabledField) {
		kialiResource, _, _ := spec.Istio.GetString("kiali.resourceName")
		if kialiResource == "" {
			kialiResource = "kiali"
		}
		kiali := &kialiv1alpha1.Kiali{}
		kiali.SetName(kialiResource)
		kiali.SetNamespace(smcp.GetNamespace())
		if err := cr.Client.Get(ctx, client.ObjectKey{Name: kiali.GetName(), Namespace: kiali.GetNamespace()}, kiali); err == nil {
			if metav1.IsControlledBy(kiali, smcp) {
				// we're managing this install, so we'll update it
				if err := spec.Istio.SetField("kiali.install", true); err != nil {
					return nil, fmt.Errorf("unexpected error disabling kiali install")
				}
			} else {
				if err := spec.Istio.SetField("kiali.install", false); err != nil {
					return nil, fmt.Errorf("unexpected error disabling kiali install")
				}
			}
		} else if !errors.IsNotFound(err) {
			if meta.IsNoMatchError(err) {
				return nil, NewDependencyMissingError("Kiali CRD", err)
			}
			return nil, pkgerrors.Wrapf(err, "error retrieving kiali resource \"%s/%s\"", smcp.GetNamespace(), kialiResource)
		} else if err := spec.Istio.SetField("kiali.install", true); err != nil {
			return nil, pkgerrors.Wrapf(err, "error enabling kiali install")
		}
	}

	// convert back to the v2 type
	smcp.Status.AppliedSpec = v2.ControlPlaneSpec{}
	err = cr.Scheme.Convert(&smcp.Status.AppliedValues, &smcp.Status.AppliedSpec, nil)
	if err != nil {
		return nil, fmt.Errorf("unexpected error setting Status.AppliedSpec: %v", err)
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

	// Update spec.Istio back with the previous content merged with global.yaml
	spec.Istio = v1.NewHelmValues(values)

	// Validate the final AppliedSpec
	err = v.ValidateV2Full(ctx, cr.Client, &smcp.ObjectMeta, &smcp.Status.AppliedSpec)
	if err != nil {
		return nil, err
	}

	if err := validateAndConfigureRLS(spec.Istio); err != nil {
		return nil, err
	}

	serverVersion, err := cr.DiscoveryClient.ServerVersion()
	if err != nil {
		return nil, err
	}
	kubeVersion := serverVersion.String()

	// Render the charts
	allErrors := []error{}
	renderings := make(map[string][]manifest.Manifest)
	log.Info("rendering helm charts")
	for name, chartDetails := range v2_1ChartMapping {
		if specialCharts.Has(name) {
			continue
		}
		if chartDetails.enabledField == "" || isComponentEnabled(spec.Istio, chartDetails.enabledField) {
			log.V(2).Info(fmt.Sprintf("rendering %s chart", name))
			chart := path.Join(v.GetChartsDir(), v2_1ChartMapping[name].path)
			if chartRenderings, _, err := helm.RenderChart(chart, smcp.GetNamespace(), kubeVersion, values); err == nil {
				renderings[name] = chartRenderings[name]
			} else {
				allErrors = append(allErrors, err)
			}
		} else {
			log.V(2).Info(fmt.Sprintf("skipping disabled %s chart", name))
		}
	}

	if isComponentEnabled(spec.Istio, "gateways") {
		log.V(2).Info("rendering gateways charts")
		if origGateways, ok := values.AsMap()["gateways"]; ok {
			if origGatewaysMap, ok := origGateways.(map[string]interface{}); ok {
				log.V(2).Info("rendering ingress gateway chart for istio-ingressgateway")
				if ingressRenderings, _, err := v.renderIngressGateway("istio-ingressgateway", smcp.GetNamespace(), kubeVersion, origGatewaysMap, spec.Istio); err == nil {
					renderings[GatewayIngressChart] = ingressRenderings[GatewayIngressChart]
				} else {
					allErrors = append(allErrors, err)
				}
				log.V(2).Info("rendering egress gateway chart for istio-egressgateway")
				if egressRenderings, _, err := v.renderEgressGateway("istio-egressgateway", smcp.GetNamespace(), kubeVersion, origGatewaysMap, spec.Istio); err == nil {
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
						if ingressRenderings, _, err := v.renderIngressGateway(name, smcp.GetNamespace(), kubeVersion, origGatewaysMap, spec.Istio); err == nil {
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
						if egressRenderings, _, err := v.renderEgressGateway(name, smcp.GetNamespace(), kubeVersion, origGatewaysMap, spec.Istio); err == nil {
							renderings[GatewayEgressChart] = append(renderings[GatewayEgressChart], egressRenderings[GatewayEgressChart]...)
						} else {
							allErrors = append(allErrors, err)
						}
					}
				}
				if err := spec.Istio.SetField("gateways", origGateways); err != nil {
					allErrors = append(allErrors, err)
				}
			}
		} else {
			allErrors = append(allErrors, fmt.Errorf("error retrieving values for gateways charts"))
		}
	} else {
		log.V(2).Info("skipping disabled gateways charts")
	}

	if isEnabled(spec.ThreeScale) {
		log.V(2).Info("rendering 3scale charts")
		if chartRenderings, _, err := helm.RenderChart(path.Join(v.GetChartsDir(), v2_1ChartMapping[ThreeScaleChart].path),
			smcp.GetNamespace(), kubeVersion, spec.ThreeScale.GetContent()); err == nil {
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

func (v *versionStrategyV2_1) renderIngressGateway(name, namespace, kubeVersion string, gateways map[string]interface{},
	values *v1.HelmValues,
) (map[string][]manifest.Manifest, map[string]interface{}, error) {
	return v.renderGateway(name, namespace, kubeVersion, v2_1ChartMapping[GatewayIngressChart].path, "istio-ingressgateway", gateways, values)
}

func (v *versionStrategyV2_1) renderEgressGateway(name, namespace, kubeVersion string, gateways map[string]interface{},
	values *v1.HelmValues,
) (map[string][]manifest.Manifest, map[string]interface{}, error) {
	return v.renderGateway(name, namespace, kubeVersion, v2_1ChartMapping[GatewayEgressChart].path, "istio-egressgateway", gateways, values)
}

func (v *versionStrategyV2_1) renderGateway(name, namespace, kubeVersion string, chartPath string, typeName string,
	gateways map[string]interface{}, values *v1.HelmValues,
) (map[string][]manifest.Manifest, map[string]interface{}, error) {
	gateway, ok, _ := unstructured.NestedMap(gateways, name)
	// if 'app' label is not provided, set it to gateway name
	if _, found, _ := unstructured.NestedString(gateway, "labels", "app"); !found {
		if err := unstructured.SetNestedField(gateway, name, "labels", "app"); err != nil {
			return nil, nil, err
		}
	}
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
	return helm.RenderChart(path.Join(v.GetChartsDir(), chartPath), namespace, kubeVersion, values)
}

func (v *versionStrategyV2_1) GetExpansionPorts() []corev1.ServicePort {
	return v.conversionImpl.GetExpansionPorts()
}

func (v *versionStrategyV2_1) GetTelemetryType(in *v1.HelmValues, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled bool) v2.TelemetryType {
	return v.conversionImpl.GetTelemetryType(in, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled)
}

func (v *versionStrategyV2_1) GetPolicyType(in *v1.HelmValues, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled bool) v2.PolicyType {
	return v.conversionImpl.GetPolicyType(in, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled)
}

func (v *versionStrategyV2_1) GetTrustDomainFieldPath() string {
	return v.conversionImpl.GetTrustDomainFieldPath()
}

func validateAndConfigureRLS(spec *v1.HelmValues) error {
	if enabled, found, _ := spec.GetBool(string(v2.ControlPlaneComponentNameRateLimiting) + ".enabled"); !found || !enabled {
		return nil
	}

	if storageBackend, found, _ := spec.GetString(string(v2.ControlPlaneComponentNameRateLimiting) + ".storageBackend"); found {
		if storageAddress, found, _ := spec.GetString(string(v2.ControlPlaneComponentNameRateLimiting) + ".storageAddress"); found {
			variables := make(map[string]string)

			switch storageBackend {
			case "redis":
				variables["REDIS_SOCKET_TYPE"] = "tcp"
				variables["REDIS_URL"] = storageAddress
			case "memcache":
				variables["BACKEND_TYPE"] = "memcache"
				variables["MEMCACHE_HOST_PORT"] = storageAddress
			default:
				return NewValidationError(fmt.Errorf("invalid value %q for %s.storageBackend. It must be one of: {redis, memcache}",
					storageBackend, v2.ControlPlaneComponentNameRateLimiting))
			}

			for k, v := range variables {
				field := fmt.Sprintf("%s.env.%s", v2.ControlPlaneComponentNameRateLimiting, k)
				if err := spec.SetField(field, v); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (v *versionStrategyV2_1) validateGlobal(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	return checkControlPlaneModeNotSet(spec, allErrors)
}
