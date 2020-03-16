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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
)

var (

	// These are unsupported in v1.1
	unsupportedOldResourcesV1_1 = []schema.GroupVersionKind{
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "HTTPAPISpecBinding"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "HTTPAPISpec"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "QuotaSpecBinding"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "QuotaSpec"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "bypass"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "circonus"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "denier"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "fluentd"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "kubernetesenv"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "listchecker"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "memquota"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "noop"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "opa"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "prometheus"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "rbac"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "redisquota"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "signalfx"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "solarwinds"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "stackdriver"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "statsd"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "stdio"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "apikey"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "authorization"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "checknothing"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "kubernetes"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "listentry"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "logentry"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "edge"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "metric"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "quota"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "reportnothing"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "tracespan"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "cloudwatch"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "dogstatsd"},
		schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "zipkin"},
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
	for _, gvk := range unsupportedOldResourcesV1_1 {
		objects := &unstructured.UnstructuredList{}
		objects.SetGroupVersionKind(gvk)
		// XXX: do we list all in the cluster, or list for each member namespace?
		if err := v.client.List(ctx, nil, objects); err != nil {
			if !meta.IsNoMatchError(err) && !errors.IsNotFound(err) {
				return pkgerrors.Wrapf(err, "error listing %s resources", gvk.String())
			}
		}
		objects.EachListItem(func(obj runtime.Object) error {
			metaObj, err := meta.Accessor(obj)
			if err != nil {
				allErrors = append(allErrors, pkgerrors.Wrapf(err, "error accessing object metadata for %s resource", gvk.String()))
			}
			// we only care about resources in this mesh, which aren't being managed by the operator directly
			if meshNamespaces.Has(metaObj.GetNamespace()) && !metav1.IsControlledBy(metaObj, smcp) {
				allErrors = append(allErrors, fmt.Errorf("%s/%s of type %s is not supported in newer version", metaObj.GetNamespace(), metaObj.GetName(), gvk.String()))
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
