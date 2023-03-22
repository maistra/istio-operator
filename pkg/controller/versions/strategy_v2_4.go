package versions

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"time"

	pkgerrors "github.com/pkg/errors"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/manifest"
	apiv1 "maistra.io/api/core/v1"
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

var v2_4ChartMapping = map[string]chartRenderingDetails{
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
	PrometheusChart: {
		path:         "istio-telemetry/prometheus",
		enabledField: "prometheus",
	},
	TracingChart: {
		path:         "istio-telemetry/tracing",
		enabledField: "tracing",
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
	RLSChart: {
		path:         "rls",
		enabledField: "rateLimiting.rls",
	},
}

var v2_4ChartOrder = [][]string{
	{DiscoveryChart},
	{MeshConfigChart},
	{TelemetryCommonChart, PrometheusChart},
	{TracingChart, GatewayIngressChart, GatewayEgressChart, GrafanaChart},
	{KialiChart},
	{ThreeScaleChart, RLSChart},
}

type versionStrategyV2_4 struct {
	Ver
	conversionImpl v2xConversionStrategy
}

var _ VersionStrategy = (*versionStrategyV2_4)(nil)

func (v *versionStrategyV2_4) SetImageValues(_ context.Context, _ *common.ControllerResources, smcpSpec *v1.ControlPlaneSpec) error {
	if err := common.UpdateField(smcpSpec.Istio, "grafana.image", common.Config.OLM.Images.V2_4.Grafana); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "pilot.image", common.Config.OLM.Images.V2_4.Pilot); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "prometheus.image", common.Config.OLM.Images.V2_4.Prometheus); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "global.proxy.image", common.Config.OLM.Images.V2_4.ProxyV2); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "rateLimiting.rls.image", common.Config.OLM.Images.V2_4.RLS); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.ThreeScale, "image", common.Config.OLM.Images.V2_4.ThreeScale); err != nil {
		return err
	}
	return nil
}

func (v *versionStrategyV2_4) IsClusterScoped(spec *v2.ControlPlaneSpec) (bool, error) {
	return spec.Mode == v2.ClusterWideMode, nil
}

func (v *versionStrategyV2_4) ValidateV1(_ context.Context, _ client.Client, _ *v1.ServiceMeshControlPlane) error {
	return fmt.Errorf("must use v2 ServiceMeshControlPlane resource for v2.0+ installations")
}

func (v *versionStrategyV2_4) ValidateV2(ctx context.Context, cl client.Client, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec) error {
	var allErrors []error
	allErrors = v.validateGlobal(ctx, v.Version(), meta, spec, cl, allErrors)
	allErrors = validateGateways(ctx, meta, spec, cl, allErrors)
	allErrors = validatePolicyType(spec, v.Ver, allErrors)
	allErrors = validateTelemetryType(spec, v.Ver, allErrors)
	allErrors = validateProtocolDetection(spec, allErrors)
	allErrors = v.validateRuntime(spec, allErrors)
	allErrors = v.validateMixerDisabled(spec, allErrors)
	allErrors = v.validateAddons(spec, allErrors)
	allErrors = v.validateExtensionProviders(spec, allErrors)
	return NewValidationError(allErrors...)
}

func (v *versionStrategyV2_4) validateRuntime(spec *v2.ControlPlaneSpec, allErrors []error) []error {
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
					".spec.runtime.components.pod.affinity.podAntiAffinity.requiredDuringScheduling and preferredDuringScheduling "+
					"is not supported for the %q component", v2.ControlPlaneComponentNameKiali))
			}
		} else {
			if config.Pod.Affinity.NodeAffinity != nil {
				allErrors = append(allErrors, fmt.Errorf("nodeAffinity is only supported for the %q component", v2.ControlPlaneComponentNameKiali))
			}
			if config.Pod.Affinity.PodAffinity != nil {
				allErrors = append(allErrors, fmt.Errorf("podAffinity is only supported for the %q component", v2.ControlPlaneComponentNameKiali))
			}
			if config.Pod.Affinity.PodAntiAffinity.PodAntiAffinity != nil {
				allErrors = append(allErrors, fmt.Errorf("PodAntiAffinity configured via "+
					".spec.runtime.components.pod.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution "+
					"and preferredDuringSchedulingIgnoredDuringExecution is only supported for the %q component", v2.ControlPlaneComponentNameKiali))
			}
		}
	}
	return allErrors
}

func (v *versionStrategyV2_4) validateMixerDisabled(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.Policy != nil && (spec.Policy.Type == v2.PolicyTypeMixer || spec.Policy.Mixer != nil) {
		allErrors = append(allErrors, fmt.Errorf("support for policy.type %q and policy.Mixer options "+
			"have been removed in v2.1, please use another alternative", v2.PolicyTypeMixer))
	}
	if spec.Telemetry != nil && (spec.Telemetry.Type == v2.TelemetryTypeMixer || spec.Telemetry.Mixer != nil) {
		allErrors = append(allErrors, fmt.Errorf("support for telemetry.type %q and telemetry.Mixer options "+
			"have been removed in v2.1, please use another alternative", v2.TelemetryTypeMixer))
	}
	return allErrors
}

func (v *versionStrategyV2_4) validateAddons(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.Addons == nil {
		return allErrors
	}

	if spec.Addons.ThreeScale != nil {
		allErrors = append(allErrors, fmt.Errorf("support for 3scale has been removed in v2.1; "+
			"please remove the spec.addons.3scale section from the SMCP and configure the 3scale WebAssembly adapter using a ServiceMeshExtension resource"))
	}
	return allErrors
}

func (v *versionStrategyV2_4) validateExtensionProviders(spec *v2.ControlPlaneSpec, allErrors []error) []error {
	if spec.ExtensionProviders == nil {
		return allErrors
	}
	for _, ext := range spec.ExtensionProviders {
		if ext.Prometheus == nil && ext.EnvoyExtAuthzHTTP == nil {
			allErrors = append(allErrors, fmt.Errorf("extension provider %s does not define any provider - "+
				"it must specify one of: prometheus or envoyExtAuthzHttp", ext.Name))
		}
		if ext.Prometheus != nil && ext.EnvoyExtAuthzHTTP != nil {
			allErrors = append(allErrors, fmt.Errorf("extension provider %s must specify only one type of provider: "+
				"prometheus or envoyExtAuthzHttp", ext.Name))
		}
		if ext.Name == "" {
			allErrors = append(allErrors, fmt.Errorf("extension provider name cannot be empty"))
		}
		if ext.EnvoyExtAuthzHTTP != nil {
			if ext.EnvoyExtAuthzHTTP.Timeout != nil {
				if _, err := time.ParseDuration(*ext.EnvoyExtAuthzHTTP.Timeout); err != nil {
					allErrors = append(allErrors, fmt.Errorf("invalid extension provider: envoyExtAuthzHttp.timeout "+
						"must be specified in the duration format - got %s", *ext.EnvoyExtAuthzHTTP.Timeout))
				}
			}
		}
	}
	return allErrors
}

func (v *versionStrategyV2_4) validateServiceMeshExtensionsRemoved(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	serviceMeshExtensions := &apiv1.ServiceMeshExtensionList{}
	if err := cl.List(ctx, serviceMeshExtensions); err != nil {
		if !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return NewValidationError(fmt.Errorf("upgrade validation failed: failed to list ServiceMeshExtensions in cluster (error: %s)",
				err,
			))
		}
	}
	if len(serviceMeshExtensions.Items) > 0 {
		smmr := &v1.ServiceMeshMemberRoll{}
		err := cl.Get(ctx, client.ObjectKey{Name: common.MemberRollName, Namespace: smcp.GetNamespace()}, smmr)
		if err != nil {
			if !errors.IsNotFound(err) {
				return NewValidationError(fmt.Errorf("upgrade validation failed: failed to retrieve SMMR for SMCP (error: %s)",
					err,
				))
			}
		}
		meshNamespaces := common.GetMeshNamespaces(smcp.GetNamespace(), smmr)
		for _, sme := range serviceMeshExtensions.Items {
			if meshNamespaces.Has(sme.Namespace) {
				return NewValidationError(fmt.Errorf("found a ServiceMeshExtension '%s' in namespace '%s'. "+
					"ServiceMeshExtension support has been removed; please migrate existing ServiceMeshExtensions to WasmPlugin",
					sme.Name,
					sme.Namespace,
				))
			}
		}
	}
	return nil
}

func (v *versionStrategyV2_4) ValidateV2Full(ctx context.Context, cl client.Client, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec) error {
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

func (v *versionStrategyV2_4) ValidateDowngrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	// TODO: what might prevent us from downgrading?
	return nil
}

func (v *versionStrategyV2_4) ValidateUpgrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	return v.validateServiceMeshExtensionsRemoved(ctx, cl, smcp)
}

func (v *versionStrategyV2_4) ValidateUpdate(ctx context.Context, cl client.Client, oldSMCPObject, newSMCPObject metav1.Object) error {
	oldSMCP, err := toSMCP(oldSMCPObject)
	if err != nil {
		return err
	}
	newSMCP, err := toSMCP(newSMCPObject)
	if err != nil {
		return err
	}

	oldClusterScoped, err := v.IsClusterScoped(&oldSMCP.Spec)
	if err != nil {
		return err
	}
	newClusterScoped, err := v.IsClusterScoped(&newSMCP.Spec)
	if err != nil {
		return err
	}
	if oldClusterScoped != newClusterScoped {
		return fmt.Errorf("field spec.mode is immutable; to change its value, delete the ServiceMeshControlPlane and recreate it")
	}
	return nil
}

func (v *versionStrategyV2_4) ValidateRequest(ctx context.Context, cl client.Client, req admission.Request, obj metav1.Object) admission.Response {
	smcp, err := toSMCP(obj)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	clusterScoped, err := v.IsClusterScoped(&smcp.Spec)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if clusterScoped {
		isClusterAdmin, err := v.isRequesterClusterAdmin(ctx, cl, req)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if !isClusterAdmin {
			return admission.ValidationResponse(false, "a cluster-scoped SMCP may only be created by users with cluster-admin permissions")
		}
	}

	return admission.ValidationResponse(true, "")
}

func (v *versionStrategyV2_4) isRequesterClusterAdmin(ctx context.Context, cl client.Client, req admission.Request) (bool, error) {
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   req.AdmissionRequest.UserInfo.Username,
			UID:    req.AdmissionRequest.UserInfo.UID,
			Extra:  common.ConvertUserInfoExtra(req.AdmissionRequest.UserInfo.Extra),
			Groups: req.AdmissionRequest.UserInfo.Groups,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:     "*",
				Group:    "*",
				Resource: "*",
			},
		},
	}
	err := cl.Create(ctx, sar)
	if err != nil {
		return false, err
	}
	return sar.Status.Allowed && !sar.Status.Denied, nil
}

func (v *versionStrategyV2_4) GetChartInstallOrder() [][]string {
	return v2_4ChartOrder
}

// TODO: consider consolidating this with 2.0 rendering logic
func (v *versionStrategyV2_4) Render(ctx context.Context, cr *common.ControllerResources, cniConfig cni.Config,
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
	if isComponentEnabled(spec.Istio, v2_4ChartMapping[TracingChart].enabledField) {
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
	if isComponentEnabled(spec.Istio, v2_4ChartMapping[KialiChart].enabledField) {
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
	for name, chartDetails := range v2_4ChartMapping {
		if specialCharts.Has(name) {
			continue
		}
		if chartDetails.enabledField == "" || isComponentEnabled(spec.Istio, chartDetails.enabledField) {
			log.V(2).Info(fmt.Sprintf("rendering %s chart", name))
			chart := path.Join(v.GetChartsDir(), v2_4ChartMapping[name].path)
			if chartRenderings, _, err := helm.RenderChart(chart, smcp.GetNamespace(), kubeVersion, values); err == nil {
				if name == "istio-discovery" {
					renderings[name] = chartRenderings["istiod"] // quick dirty workaround (istio-discovery chart now has the name "istiod" in Chart.yaml)
				} else {
					renderings[name] = chartRenderings[name]
				}
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
				ns := &corev1.Namespace{}
				if err := cr.Client.Get(ctx, types.NamespacedName{Name: smcp.GetNamespace()}, ns); err == nil {
					userIDAutoassigned := ns.Annotations["openshift.io/sa.scc.uid-range"] != ""

					log.V(2).Info("rendering ingress gateway chart for istio-ingressgateway")
					if ingressRenderings, _, err := v.renderIngressGateway("istio-ingressgateway",
						smcp.GetNamespace(), kubeVersion, origGatewaysMap, spec.Istio, userIDAutoassigned); err == nil {
						renderings[GatewayIngressChart] = ingressRenderings[GatewayIngressChart]
					} else {
						allErrors = append(allErrors, err)
					}
					log.V(2).Info("rendering egress gateway chart for istio-egressgateway")
					if egressRenderings, _, err := v.renderEgressGateway("istio-egressgateway",
						smcp.GetNamespace(), kubeVersion, origGatewaysMap, spec.Istio, userIDAutoassigned); err == nil {
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
							if ingressRenderings, _, err := v.renderIngressGateway(name, smcp.GetNamespace(), kubeVersion,
								origGatewaysMap, spec.Istio, userIDAutoassigned); err == nil {
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
							if egressRenderings, _, err := v.renderEgressGateway(name, smcp.GetNamespace(), kubeVersion,
								origGatewaysMap, spec.Istio, userIDAutoassigned); err == nil {
								renderings[GatewayEgressChart] = append(renderings[GatewayEgressChart], egressRenderings[GatewayEgressChart]...)
							} else {
								allErrors = append(allErrors, err)
							}
						}
					}
				} else {
					allErrors = append(allErrors, err)
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
		if chartRenderings, _, err := helm.RenderChart(
			path.Join(v.GetChartsDir(), v2_4ChartMapping[ThreeScaleChart].path),
			smcp.GetNamespace(), kubeVersion,
			spec.ThreeScale.GetContent()); err == nil {
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

func (v *versionStrategyV2_4) renderIngressGateway(name, namespace, kubeVersion string, gateways map[string]interface{},
	values *v1.HelmValues, userIDAutoassigned bool,
) (map[string][]manifest.Manifest, map[string]interface{}, error) {
	return v.renderGateway(name, namespace, kubeVersion, v2_4ChartMapping[GatewayIngressChart].path, "istio-ingressgateway", gateways, values, userIDAutoassigned)
}

func (v *versionStrategyV2_4) renderEgressGateway(name, namespace, kubeVersion string, gateways map[string]interface{},
	values *v1.HelmValues, userIDAutoassigned bool,
) (map[string][]manifest.Manifest, map[string]interface{}, error) {
	return v.renderGateway(name, namespace, kubeVersion, v2_4ChartMapping[GatewayEgressChart].path, "istio-egressgateway", gateways, values, userIDAutoassigned)
}

func (v *versionStrategyV2_4) renderGateway(name, namespace, kubeVersion string, chartPath string, typeName string,
	gateways map[string]interface{}, values *v1.HelmValues, userIDAutoassigned bool,
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
	if !userIDAutoassigned {
		if gateway["runAsUser"] == nil {
			gateway["runAsUser"] = "1337"
		}
		if gateway["runAsGroup"] == nil {
			gateway["runAsGroup"] = "1337"
		}
		if gateway["fsGroup"] == nil {
			gateway["fsGroup"] = "1337"
		}
	}
	newGateways := make(map[string]interface{})
	newGateways["revision"] = gateways["revision"]
	newGateways[typeName] = gateway
	if err := values.SetField("gateways", newGateways); err != nil {
		return nil, nil, err
	}
	return helm.RenderChart(path.Join(v.GetChartsDir(), chartPath), namespace, kubeVersion, values)
}

func (v *versionStrategyV2_4) GetExpansionPorts() []corev1.ServicePort {
	return v.conversionImpl.GetExpansionPorts()
}

func (v *versionStrategyV2_4) GetTelemetryType(in *v1.HelmValues, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled bool) v2.TelemetryType {
	return v.conversionImpl.GetTelemetryType(in, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled)
}

func (v *versionStrategyV2_4) GetPolicyType(in *v1.HelmValues, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled bool) v2.PolicyType {
	return v.conversionImpl.GetPolicyType(in, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled)
}

func (v *versionStrategyV2_4) GetTrustDomainFieldPath() string {
	return "meshConfig.trustDomain"
}

func (v *versionStrategyV2_4) validateGlobal(
	ctx context.Context, version Ver, meta *metav1.ObjectMeta,
	spec *v2.ControlPlaneSpec, cl client.Client, allErrors []error,
) []error {
	if spec.Mode != "" {
		if spec.Mode != v2.ClusterWideMode && spec.Mode != v2.MultiTenantMode {
			return append(allErrors,
				fmt.Errorf("spec.mode must be either %s or %s",
					v2.MultiTenantMode, v2.ClusterWideMode))
		}
	} else if spec.TechPreview != nil {
		if _, found, _ := spec.TechPreview.GetString(v2.TechPreviewControlPlaneModeKey); found {
			return append(allErrors,
				fmt.Errorf("the spec.techPreview.%s field is not supported in version 2.4+; use spec.mode",
					v2.TechPreviewControlPlaneModeKey))
		}
	}

	return validateGlobal(ctx, version, meta, spec, cl, allErrors)
}
