// NOTE: Boilerplate only.  Ignore this file.

// Package v1 contains API Schema definitions for the maistra v1 API group
// +k8s:deepcopy-gen=package,register
// +groupName=maistra.io
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
)

const (
	APIGroup   = "maistra.io"
	APIVersion = "v1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: APIGroup, Version: APIVersion}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)
