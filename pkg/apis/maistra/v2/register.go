// NOTE: Boilerplate only.  Ignore this file.

// Package v2 contains API Schema definitions for the maistra v2 API group
// +k8s:deepcopy-gen=package,register
// +groupName=maistra.io
package v2

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
)

const (
	// APIGroup for maistr.io
	APIGroup = "maistra.io"
	// APIVersion for v2
	APIVersion = "v2"
)

var (
	// `SchemeGroupVersion` is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: APIGroup, Version: APIVersion}

	// `SchemeBuilder` is used to add Go types to the `GroupVersionKind` scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)
