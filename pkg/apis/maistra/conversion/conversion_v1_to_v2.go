package conversion

import (
	conversion "k8s.io/apimachinery/pkg/conversion"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

// Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec converts a v1 ControlPlaneSpec to its v2 equivalent.
func Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(in *v1.ControlPlaneSpec, out *v2.ControlPlaneSpec, s conversion.Scope) error {
	values := in.Istio.GetContent()
	if err := populateGatewaysConfig(values, out.Gateways); err != nil {
		return err
	}

	if err := populateAddonsConfig(values, out.Addons); err != nil {
		return err
	}

	return nil
}

func Convert_v1_ControlPlaneStatus_To_v2_ControlPlaneStatus(in *v1.ControlPlaneStatus, out *v2.ControlPlaneStatus, s conversion.Scope) error {
	in.DeepCopyInto(&out.ControlPlaneStatus)
	return nil
}
