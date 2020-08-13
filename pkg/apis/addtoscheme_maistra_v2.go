package apis

import (
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"

	imagev1 "github.com/openshift/api/image/v1"
	networkv1 "github.com/openshift/api/network/v1"
	routev1 "github.com/openshift/api/route/v1"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	authorizationv1 "k8s.io/api/authorization/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		v2.SchemeBuilder.AddToScheme,
		imagev1.AddToScheme,
		networkv1.Install,
		routev1.Install,
		authorizationv1.AddToScheme,
		autoscalingv2beta1.AddToScheme,
		policyv1beta1.AddToScheme,
		appsv1beta1.AddToScheme,
		extensionsv1beta1.AddToScheme,
		corev1.AddToScheme,
		rbacv1.AddToScheme,
		apiextensionsv1.AddToScheme,
		apiextensionsv1beta1.AddToScheme)
}
