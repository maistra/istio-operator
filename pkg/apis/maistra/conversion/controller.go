package conversion

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func init() {
	v1.ConverterV1V2 = func(src, dst runtime.Object) error {
		srcSMCP, ok := src.(*v1.ServiceMeshControlPlane)
		if !ok {
			return fmt.Errorf("src is not a v1.ServiceMeshControlPlane: %T", src)
		}
		dstSMCP, ok := dst.(*v2.ServiceMeshControlPlane)
		if !ok {
			return fmt.Errorf("dst is not a v2.ServiceMeshControlPlane: %T", dst)
		}
		return Convert_v1_ServiceMeshControlPlane_To_v2_ServiceMeshControlPlane(srcSMCP, dstSMCP, nil)
	}

	v1.ConverterV2V1 = func(src, dst runtime.Object) error {
		srcSMCP, ok := src.(*v2.ServiceMeshControlPlane)
		if !ok {
			return fmt.Errorf("src is not a v2.ServiceMeshControlPlane: %T", src)
		}
		dstSMCP, ok := dst.(*v1.ServiceMeshControlPlane)
		if !ok {
			return fmt.Errorf("dst is not a v1.ServiceMeshControlPlane: %T", dst)
		}
		return Convert_v2_ServiceMeshControlPlane_To_v1_ServiceMeshControlPlane(srcSMCP, dstSMCP, nil)
	}
}
