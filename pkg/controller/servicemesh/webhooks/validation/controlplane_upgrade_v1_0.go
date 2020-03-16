package validation

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha2 "github.com/maistra/istio-operator/pkg/apis/istio/simple/config/v1alpha2"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
)

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

func (v *ControlPlaneValidator) validateUpgradeFromV1_0(ctx context.Context, smcp *maistrav1.ServiceMeshControlPlane) error {
	var allErrors []error

	meshNamespaces := sets.NewString(smcp.GetNamespace())
	smmr, err := v.getSMMR(smcp)
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
		if err := v.client.List(ctx, nil, list); err != nil {
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
		if err := v.client.List(ctx, client.InNamespace(namespace), memberServices); err != nil {
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
