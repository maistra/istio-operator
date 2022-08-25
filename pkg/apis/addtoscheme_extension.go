package apis

// FIXME: Remove this hack as part of https://issues.redhat.com/browse/MAISTRA-2331

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiv1 "maistra.io/api/core/v1"
	apiv1alpha1 "maistra.io/api/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	schemeGroupVersionAPIV1 = schema.GroupVersion{Group: "maistra.io", Version: "v1"}
	schemeBuilderAPIV1      = &scheme.Builder{GroupVersion: schemeGroupVersionAPIV1}

	schemeGroupVersionAPIV1alpha1 = schema.GroupVersion{Group: "maistra.io", Version: "v1alpha1"}
	schemeBuilderAPIV1alpha1      = &scheme.Builder{GroupVersion: schemeGroupVersionAPIV1alpha1}
)

func init() {
	schemeBuilderAPIV1.Register(&apiv1.ServiceMeshExtension{}, &apiv1.ServiceMeshExtensionList{})
	schemeBuilderAPIV1alpha1.Register(&apiv1alpha1.ServiceMeshExtension{}, &apiv1alpha1.ServiceMeshExtensionList{})

	AddToSchemes = append(AddToSchemes,
		schemeBuilderAPIV1.AddToScheme,
		schemeBuilderAPIV1alpha1.AddToScheme,
	)
}
