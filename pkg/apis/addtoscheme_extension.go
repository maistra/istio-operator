package apis

// FIXME: Remove this hack as part of https://issues.redhat.com/browse/MAISTRA-2331

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	apiv1 "maistra.io/api/core/v1"
	apiv1alpha1 "maistra.io/api/core/v1alpha1"
)

var (
	schemeGroupVersionApiV1 = schema.GroupVersion{Group: "maistra.io", Version: "v1"}
	schemeBuilderApiV1      = &scheme.Builder{GroupVersion: schemeGroupVersionApiV1}

	schemeGroupVersionApiV1alpha1 = schema.GroupVersion{Group: "maistra.io", Version: "v1alpha1"}
	schemeBuilderApiV1alpha1      = &scheme.Builder{GroupVersion: schemeGroupVersionApiV1alpha1}
)

func init() {
	schemeBuilderApiV1.Register(&apiv1.ServiceMeshExtension{}, &apiv1.ServiceMeshExtensionList{})
	schemeBuilderApiV1alpha1.Register(&apiv1alpha1.ServiceMeshExtension{}, &apiv1alpha1.ServiceMeshExtensionList{})

	AddToSchemes = append(AddToSchemes,
		schemeBuilderApiV1.AddToScheme,
		schemeBuilderApiV1alpha1.AddToScheme,
	)
}
