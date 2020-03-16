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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/simple/networking/v1alpha3"
	securityv1beta1 "github.com/maistra/istio-operator/pkg/apis/istio/simple/security/v1beta1"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

var (
	// These are unsupported in v1.0
	unsupportedNewResourcesV1_0 = []runtime.Object{
		&securityv1beta1.AuthorizationPolicyList{},
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
	virtualServices := &networkingv1alpha3.VirtualServiceList{}
	// XXX: do we list all in the cluster, or list for each member namespace?
	if err := v.client.List(ctx, nil, virtualServices); err != nil {
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
		if err := v.client.List(ctx, nil, list); err != nil {
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

	return utilerrors.NewAggregate(allErrors)
}
