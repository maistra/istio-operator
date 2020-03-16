package validation

import (
	"context"
	"fmt"

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
	"github.com/maistra/istio-operator/pkg/controller/common"
)

var (
	// These are unsupported in v1.0
	unsupportedNewResourcesV1_0 = []schema.GroupVersionKind{
		schema.GroupVersionKind{Group: "security.istio.io", Version: "v1beta1", Kind: "AuthorizationPolicy"},
	}
)

func (v *ControlPlaneValidator) validateDowngradeFromV1_1(ctx context.Context, smcp *maistrav1.ServiceMeshControlPlane) error {
	var allErrors []error
	meshNamespaces := sets.NewString(smcp.GetNamespace())

	memberNamespaces := &corev1.NamespaceList{}
	if err := v.client.List(ctx, client.MatchingLabels(map[string]string{common.MemberOfKey: smcp.GetNamespace()}), memberNamespaces); err != nil {
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
	vsgvk := schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1alpha3", Kind: "VirtualService"}
	virtualServices := &unstructured.UnstructuredList{}
	virtualServices.SetGroupVersionKind(vsgvk)
	// XXX: do we list all in the cluster, or list for each member namespace?
	if err := v.client.List(ctx, nil, virtualServices); err != nil {
		if !meta.IsNoMatchError(err) && !errors.IsNotFound(err) {
			return pkgerrors.Wrapf(err, "error listing %s resources", vsgvk.String())
		}
	}
	for _, vs := range virtualServices.Items {
		// we only care about resources in this mesh, which aren't being managed by the operator directly
		if meshNamespaces.Has(vs.GetNamespace()) && !metav1.IsControlledBy(&vs, smcp) {
			if routes, ok, _ := unstructured.NestedSlice(vs.UnstructuredContent(), "spec", "http"); ok {
				for _, route := range routes {
					if routeStruct, ok := route.(map[string]interface{}); ok {
						if _, ok, _ := unstructured.NestedFieldNoCopy(routeStruct, "mirrorPercent"); ok {
							allErrors = append(allErrors, fmt.Errorf("http.mirrorPercent on VirtualService %s/%s is not supported on older version", vs.GetNamespace(), vs.GetName()))
							break;
						}
					}
				}
			}
		}
	}

	// return error if any new resources are being used
	for _, gvk := range unsupportedNewResourcesV1_0 {
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
				allErrors = append(allErrors, fmt.Errorf("%s/%s of type %s is not supported in older version", metaObj.GetNamespace(), metaObj.GetName(), gvk.String()))
			}
			return nil
		})
	}

	return utilerrors.NewAggregate(allErrors)
}
