package apis

import (
	"github.com/maistra/istio-operator/pkg/apis/istio/v1alpha1"

	securityv1 "github.com/openshift/api/security/v1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, v1alpha1.SchemeBuilder.AddToScheme, securityv1.AddToScheme, corev1.AddToScheme, rbacv1.AddToScheme, batchv1.AddToScheme)
}
