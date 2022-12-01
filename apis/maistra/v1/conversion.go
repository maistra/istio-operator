package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

var _ conversion.Convertible = (*ServiceMeshControlPlane)(nil)

// ConverterV1V2 and ConverterV2V1 functions are really hacky, but allow us to avoid circular imports.
// The alternative is to move the entire conversion package into here.
var (
	ConverterV1V2 func(src, dst runtime.Object) error
	ConverterV2V1 func(src, dst runtime.Object) error
)

// ConvertTo v2 SMCP resource
func (smcp *ServiceMeshControlPlane) ConvertTo(dst conversion.Hub) error {
	return ConverterV1V2(smcp, dst)
}

// ConvertFrom v2 SMCP resource
func (smcp *ServiceMeshControlPlane) ConvertFrom(src conversion.Hub) error {
	return ConverterV2V1(src, smcp)
}
