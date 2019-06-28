package apis

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	originappsv1 "github.com/openshift/api/apps/v1"
	networkv1 "github.com/openshift/api/network/v1"
	routev1 "github.com/openshift/api/route/v1"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	authorizationv1 "k8s.io/api/authorization/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		v1.SchemeBuilder.AddToScheme,
		originappsv1.AddToScheme,
		networkv1.AddToScheme,
		routev1.AddToScheme,
		authorizationv1.AddToScheme,
		autoscalingv2beta1.AddToScheme,
		policyv1beta1.AddToScheme,
		appsv1beta1.AddToScheme,
		extensionsv1beta1.AddToScheme,
		batchv1.AddToScheme,
		corev1.AddToScheme,
		rbacv1.AddToScheme)
}
