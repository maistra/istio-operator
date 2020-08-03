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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/helm/pkg/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	configv1alpha2 "github.com/maistra/istio-operator/pkg/apis/istio/simple/config/v1alpha2"
	networkingv1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/simple/networking/v1alpha3"
	securityv1beta1 "github.com/maistra/istio-operator/pkg/apis/istio/simple/security/v1beta1"
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
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
	version
	renderImpl v1xRenderingStrategy
}

var _ VersionStrategy = (*versionStrategyV1_1)(nil)

func (v *versionStrategyV1_1) SetImageValues(ctx context.Context, cr *common.ControllerResources, smcpSpec *v1.ControlPlaneSpec) error {
	common.UpdateField(smcpSpec.Istio, "security.image", common.Config.OLM.Images.V1_1.Citadel)
	common.UpdateField(smcpSpec.Istio, "galley.image", common.Config.OLM.Images.V1_1.Galley)
	common.UpdateField(smcpSpec.Istio, "grafana.image", common.Config.OLM.Images.V1_1.Grafana)
	common.UpdateField(smcpSpec.Istio, "mixer.image", common.Config.OLM.Images.V1_1.Mixer)
	common.UpdateField(smcpSpec.Istio, "pilot.image", common.Config.OLM.Images.V1_1.Pilot)
	common.UpdateField(smcpSpec.Istio, "prometheus.image", common.Config.OLM.Images.V1_1.Prometheus)
	common.UpdateField(smcpSpec.Istio, "global.proxy_init.image", common.Config.OLM.Images.V1_1.ProxyInit)
	common.UpdateField(smcpSpec.Istio, "global.proxy.image", common.Config.OLM.Images.V1_1.ProxyV2)
	common.UpdateField(smcpSpec.Istio, "sidecarInjectorWebhook.image", common.Config.OLM.Images.V1_1.SidecarInjector)
	common.UpdateField(smcpSpec.Istio, "image", common.Config.OLM.Images.V1_1.ThreeScale)

	common.UpdateField(smcpSpec.Istio, "gateways.istio-ingressgateway.ior_image", common.Config.OLM.Images.V1_1.IOR)
	return nil
}
func (v *versionStrategyV1_1) Validate(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
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
		if err := errForEnabledValue(smcp.Spec.Istio, "tracing.enabled", true); err != nil {
			// tracing.enabled must be false
			allErrors = append(allErrors, fmt.Errorf("tracing.enabled must be false if global.tracer.zipkin.address is set"))
		}

		if err := errForEnabledValue(smcp.Spec.Istio, "kiali.enabled", true); err != nil {
			if jaegerInClusterURL, ok, _ := smcp.Spec.Istio.GetString("kiali.jaegerInClusterURL"); !ok || len(jaegerInClusterURL) == 0 {
				allErrors = append(allErrors, fmt.Errorf("kiali.jaegerInClusterURL must be defined if global.tracer.zipkin.address is set"))
			}
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

var (
	// These are unsupported in v1.0
	unsupportedNewResourcesV1_0 = []runtime.Object{
		&securityv1beta1.AuthorizationPolicyList{},
	}
)

func (v *versionStrategyV1_1) ValidateDowngrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	var allErrors []error
	meshNamespaces := sets.NewString(smcp.GetNamespace())

	memberNamespaces := &corev1.NamespaceList{}
	if err := cl.List(ctx, memberNamespaces, client.MatchingLabels(map[string]string{common.MemberOfKey: smcp.GetNamespace()})); err != nil {
		return pkgerrors.Wrap(err, "error listing member namespaces")
	}
	for _, member := range memberNamespaces.Items {
		meshNamespaces.Insert(member.GetName())
		// ca.istio.io/env label exists on any member namespaces
		if common.HasLabel(&member.ObjectMeta, "ca.istio.io/env") {
			allErrors = append(allErrors, fmt.Errorf("ca.istio.io/env label on namespace %s is not supported in older version", member.GetName()))
		}
		// ca.isio.io/override label exists on any member namespaces
		if common.HasLabel(&member.ObjectMeta, "ca.istio.io/override") {
			allErrors = append(allErrors, fmt.Errorf("ca.istio.io/override label on namespace %s is not supported in older version", member.GetName()))
		}
	}

	// Any VirtualService http entries use mirrorPercent attribute
	virtualServices := &networkingv1alpha3.VirtualServiceList{}
	// XXX: do we list all in the cluster, or list for each member namespace?
	if err := cl.List(ctx, virtualServices); err != nil {
		if !meta.IsNoMatchError(err) && !errors.IsNotFound(err) {
			return pkgerrors.Wrapf(err, "error listing %T resources", virtualServices)
		}
	}
	for _, vs := range virtualServices.Items {
		// we only care about resources in this mesh, which aren't being managed by the operator directly
		if meshNamespaces.Has(vs.GetNamespace()) && !metav1.IsControlledBy(&vs, smcp) {
			if routes, ok, _ := unstructured.NestedSlice(vs.Spec, "http"); ok {
				for _, route := range routes {
					if routeStruct, ok := route.(map[string]interface{}); ok {
						if _, ok, _ := unstructured.NestedFieldNoCopy(routeStruct, "mirrorPercent"); ok {
							allErrors = append(allErrors, fmt.Errorf("http.mirrorPercent on VirtualService %s/%s is not supported on older version", vs.GetNamespace(), vs.GetName()))
							break
						}
					}
				}
			}
		}
	}

	// return error if any new resources are being used
	for _, list := range unsupportedNewResourcesV1_0 {
		list = list.DeepCopyObject()
		// XXX: do we list all in the cluster, or list for each member namespace?
		if err := cl.List(ctx, list); err != nil {
			if !meta.IsNoMatchError(err) && !errors.IsNotFound(err) {
				return pkgerrors.Wrapf(err, "error listing %T resources", list)
			}
		}
		meta.EachListItem(list, func(obj runtime.Object) error {
			metaObj, err := meta.Accessor(obj)
			if err != nil {
				allErrors = append(allErrors, pkgerrors.Wrapf(err, "error accessing object metadata for %s resource", obj.GetObjectKind().GroupVersionKind().String()))
			}
			// we only care about resources in this mesh, which aren't being managed by the operator directly
			if meshNamespaces.Has(metaObj.GetNamespace()) && !metav1.IsControlledBy(metaObj, smcp) {
				allErrors = append(allErrors, fmt.Errorf("%s/%s of type %s is not supported in older version", metaObj.GetNamespace(), metaObj.GetName(), obj.GetObjectKind().GroupVersionKind().String()))
			}
			return nil
		})
	}

	// we don't do any validation for 1.0 control planes, but the validation
	// logic for it is associated with v1.0, so invoke it now.
	if err := V1_0.Strategy().Validate(ctx, cl, smcp); err != nil {
		allErrors = append(allErrors, err)
	}

	return utilerrors.NewAggregate(allErrors)
}

var (
	// These are unsupported in v1.1
	unsupportedOldResourcesV1_1 = []runtime.Object{
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
)

func (v *versionStrategyV1_1) ValidateUpgrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
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
		meta.EachListItem(list, func(obj runtime.Object) error {
			metaObj, err := meta.Accessor(obj)
			if err != nil {
				allErrors = append(allErrors, pkgerrors.Wrapf(err, "error accessing object metadata for %s resource", list.GetObjectKind().GroupVersionKind().String()))
			}
			// we only care about resources in this mesh, which aren't being managed by the operator directly
			if meshNamespaces.Has(metaObj.GetNamespace()) && !metav1.IsControlledBy(metaObj, smcp) {
				allErrors = append(allErrors, fmt.Errorf("%s/%s of type %s is not supported in newer version", metaObj.GetNamespace(), metaObj.GetName(), list.GetObjectKind().GroupVersionKind().String()))
			}
			return nil
		})
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
					allErrors = append(allErrors, fmt.Errorf("Port 443 is not allowed for http/http2 protocols on Service %s/%s", service.Namespace, service.Name))
				}
			}
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

func (v *versionStrategyV1_1) GetChartInstallOrder() [][]string {
	return v1_1ChartOrder
}

func (v *versionStrategyV1_1) Render(ctx context.Context, cr *common.ControllerResources, smcp *v2.ServiceMeshControlPlane) (map[string][]manifest.Manifest, error) {
	return v.renderImpl.render(ctx, v.version, cr, smcp)
}
