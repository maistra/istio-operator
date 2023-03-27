package versions

import (
	"context"
	"fmt"
	"strings"

	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/helm/pkg/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	configv1alpha2 "github.com/maistra/istio-operator/pkg/apis/external/istio/config/v1alpha2"
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
)

// these components have to be installed in the specified order
var v1_1ChartOrder = [][]string{
	{"istio"}, // core istio resources
	{"istio/charts/security"},
	{"istio/charts/prometheus"},
	{"istio/charts/tracing"},
	{"istio/charts/galley"},
	{"istio/charts/mixer", "istio/charts/pilot", "istio/charts/gateways", "istio/charts/sidecarInjectorWebhook"},
	{"istio/charts/grafana"},
	{"istio/charts/kiali"},
}

type versionStrategyV1_1 struct {
	Ver
	renderImpl     v1xRenderingStrategy
	conversionImpl v1xConversionStrategy
}

var _ VersionStrategy = (*versionStrategyV1_1)(nil)

func (v *versionStrategyV1_1) SetImageValues(ctx context.Context, cr *common.ControllerResources, smcpSpec *v1.ControlPlaneSpec) error {
	if err := common.UpdateField(smcpSpec.Istio, "security.image", common.Config.OLM.Images.V1_1.Citadel); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "galley.image", common.Config.OLM.Images.V1_1.Galley); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "grafana.image", common.Config.OLM.Images.V1_1.Grafana); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "mixer.image", common.Config.OLM.Images.V1_1.Mixer); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "pilot.image", common.Config.OLM.Images.V1_1.Pilot); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "prometheus.image", common.Config.OLM.Images.V1_1.Prometheus); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "global.proxy_init.image", common.Config.OLM.Images.V1_1.ProxyInit); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "global.proxy.image", common.Config.OLM.Images.V1_1.ProxyV2); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "sidecarInjectorWebhook.image", common.Config.OLM.Images.V1_1.SidecarInjector); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.ThreeScale, "image", common.Config.OLM.Images.V1_1.ThreeScale); err != nil {
		return err
	}
	if err := common.UpdateField(smcpSpec.Istio, "gateways.istio-ingressgateway.ior_image", common.Config.OLM.Images.V1_1.IOR); err != nil {
		return err
	}
	return nil
}

func (v *versionStrategyV1_1) IsClusterScoped(spec *v2.ControlPlaneSpec) (bool, error) {
	return false, nil
}

func (v *versionStrategyV1_1) ValidateV1(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	logger := logf.Log.WithName("smcp-validator-1.1")
	var allErrors []error

	if zipkinAddress, ok, _ := smcp.Spec.Istio.GetString("global.tracer.zipkin.address"); ok && len(zipkinAddress) > 0 {
		tracer, ok, _ := smcp.Spec.Istio.GetString("global.proxy.tracer")
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
		if err := errForEnabledValue(smcp.Spec.Istio, "tracing.enabled"); err != nil {
			// tracing.enabled must be false
			allErrors = append(allErrors, fmt.Errorf("tracing.enabled must be false if global.tracer.zipkin.address is set"))
		}

		if err := errForEnabledValue(smcp.Spec.Istio, "kiali.enabled"); err != nil {
			if jaegerInClusterURL, ok, _ := smcp.Spec.Istio.GetString("kiali.jaegerInClusterURL"); !ok || len(jaegerInClusterURL) == 0 {
				allErrors = append(allErrors, fmt.Errorf("kiali.jaegerInClusterURL must be defined if global.tracer.zipkin.address is set"))
			}
		}
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
	if err := errForEnabledValue(smcp.Spec.Istio, "telemetry.enabled"); err != nil {
		if err := errForEnabledValue(smcp.Spec.Istio, "telemetry.v2.enabled"); err != nil {
			allErrors = append(allErrors, err)
		}
	}

	smmr := &v1.ServiceMeshMemberRoll{}
	err := cl.Get(ctx, client.ObjectKey{Name: common.MemberRollName, Namespace: smcp.GetNamespace()}, smmr)
	if err != nil {
		if !errors.IsNotFound(err) {
			// log error, but don't fail validation: we'll just assume that the control plane namespace is the only namespace for now
			logger.Error(err, "failed to retrieve SMMR for SMCP")
			smmr = nil
		}
	}

	meshNamespaces := common.GetMeshNamespaces(smcp.GetNamespace(), smmr)
	for _, gateway := range getMapKeys(smcp.Spec.Istio, "gateways") {
		if err := errForStringValue(smcp.Spec.Istio, "gateways."+gateway+".namespace", meshNamespaces); err != nil {
			allErrors = append(allErrors, fmt.Errorf("%v: namespace must be part of the mesh", err))
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

func (v *versionStrategyV1_1) ValidateV2(ctx context.Context, cl client.Client, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec) error {
	var allErrors []error
	allErrors = validatePolicyType(spec, v.Ver, allErrors)
	allErrors = validateTelemetryType(spec, v.Ver, allErrors)
	allErrors = validateGateways(ctx, meta, spec, cl, allErrors)
	return NewValidationError(allErrors...)
}

func (v *versionStrategyV1_1) ValidateDowngrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	// this should never be called
	return fmt.Errorf("downgrading to a version below v1.1 is not supported")
}

// These are unsupported in v1.1
var unsupportedOldResourcesV1_1 = []runtime.Object{
	&configv1alpha2.HTTPAPISpecBindingList{},
	&configv1alpha2.HTTPAPISpecList{},
	&configv1alpha2.QuotaSpecBindingList{},
	&configv1alpha2.QuotaSpecList{},
	&configv1alpha2.BypassList{},
	&configv1alpha2.CirconusList{},
	&configv1alpha2.DenierList{},
	&configv1alpha2.FluentdList{},
	&configv1alpha2.KubernetesenvList{},
	&configv1alpha2.ListcheckerList{},
	&configv1alpha2.MemquotaList{},
	&configv1alpha2.NoopList{},
	&configv1alpha2.OpaList{},
	&configv1alpha2.PrometheusList{},
	&configv1alpha2.RbacList{},
	&configv1alpha2.RedisquotaList{},
	&configv1alpha2.SignalfxList{},
	&configv1alpha2.SolarwindsList{},
	&configv1alpha2.StackdriverList{},
	&configv1alpha2.StatsdList{},
	&configv1alpha2.StdioList{},
	&configv1alpha2.ApikeyList{},
	&configv1alpha2.AuthorizationList{},
	&configv1alpha2.ChecknothingList{},
	&configv1alpha2.KubernetesList{},
	&configv1alpha2.ListentryList{},
	&configv1alpha2.LogentryList{},
	&configv1alpha2.EdgeList{},
	&configv1alpha2.MetricList{},
	&configv1alpha2.QuotaList{},
	&configv1alpha2.ReportnothingList{},
	&configv1alpha2.TracespanList{},
	&configv1alpha2.CloudwatchList{},
	&configv1alpha2.DogstatsdList{},
	&configv1alpha2.ZipkinList{},
}

func (v *versionStrategyV1_1) ValidateUpgrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	var allErrors []error

	meshNamespaces := sets.NewString(smcp.GetNamespace())
	smmr := &v1.ServiceMeshMemberRoll{}
	err := cl.Get(ctx, client.ObjectKey{Namespace: smcp.GetNamespace(), Name: common.MemberRollName}, smmr)
	if err != nil {
		if !errors.IsNotFound(err) {
			return pkgerrors.Wrap(err, "error retrieving ServiceMeshMemberRoll for mesh")
		}
	}
	meshNamespaces.Insert(smmr.Status.ConfiguredMembers...)

	// return error if any deprecated mixer resources are being used
	for _, list := range unsupportedOldResourcesV1_1 {
		list = list.DeepCopyObject()
		// XXX: do we list all in the cluster, or list for each member namespace?
		if err := cl.List(ctx, list); err != nil {
			if !meta.IsNoMatchError(err) && !errors.IsNotFound(err) {
				return pkgerrors.Wrapf(err, "error listing %T resources", list)
			}
		}
		if err := meta.EachListItem(list, func(obj runtime.Object) error {
			metaObj, err := meta.Accessor(obj)
			if err != nil {
				allErrors = append(allErrors, pkgerrors.Wrapf(err, "error accessing object metadata for %s resource",
					list.GetObjectKind().GroupVersionKind().String()))
			}
			// we only care about resources in this mesh, which aren't being managed by the operator directly
			if meshNamespaces.Has(metaObj.GetNamespace()) && !metav1.IsControlledBy(metaObj, smcp) {
				allErrors = append(allErrors, fmt.Errorf("%s/%s of type %s is not supported in newer version",
					metaObj.GetNamespace(), metaObj.GetName(), list.GetObjectKind().GroupVersionKind().String()))
			}
			return nil
		}); err != nil {
			return err
		}
	}

	// Any service ports using 443 are using http/http2 in their name (http not allowed on port 443)
	for namespace := range meshNamespaces {
		memberServices := &corev1.ServiceList{}
		// listing for each member namespace, as we expect a large number of services in the whole cluster
		if err := cl.List(ctx, memberServices, client.InNamespace(namespace)); err != nil {
			return pkgerrors.Wrapf(err, "error listing Service resources in namespace %s", namespace)
		}
		for _, service := range memberServices.Items {
			for _, port := range service.Spec.Ports {
				if port.Port == 443 && (port.Name == "http" || port.Name == "http2" || strings.HasPrefix(port.Name, "http-") || strings.HasPrefix(port.Name, "http2-")) {
					allErrors = append(allErrors, fmt.Errorf("port 443 is not allowed for http/http2 protocols on Service %s/%s", service.Namespace, service.Name))
				}
			}
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

func (v *versionStrategyV1_1) ValidateUpdate(ctx context.Context, cl client.Client, oldSMCP, newSMCP metav1.Object) error {
	return nil
}

func (v *versionStrategyV1_1) ValidateRequest(ctx context.Context, cl client.Client, req admission.Request, smcp metav1.Object) admission.Response {
	return admission.ValidationResponse(true, "")
}

func (v *versionStrategyV1_1) GetChartInstallOrder() [][]string {
	return v1_1ChartOrder
}

func (v *versionStrategyV1_1) Render(ctx context.Context, cr *common.ControllerResources, cniConfig cni.Config,
	smcp *v2.ServiceMeshControlPlane,
) (map[string][]manifest.Manifest, error) {
	return v.renderImpl.render(ctx, v.Ver, cr, cniConfig, smcp)
}

func (v *versionStrategyV1_1) GetExpansionPorts() []corev1.ServicePort {
	return v.conversionImpl.GetExpansionPorts()
}

func (v *versionStrategyV1_1) GetTelemetryType(in *v1.HelmValues, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled bool) v2.TelemetryType {
	return v.conversionImpl.GetTelemetryType(in, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled)
}

func (v *versionStrategyV1_1) GetPolicyType(in *v1.HelmValues, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled bool) v2.PolicyType {
	return v.conversionImpl.GetPolicyType(in, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled)
}

func (v *versionStrategyV1_1) GetTrustDomainFieldPath() string {
	return v.conversionImpl.GetTrustDomainFieldPath()
}
