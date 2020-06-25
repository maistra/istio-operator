package conversion

import (
	conversion "k8s.io/apimachinery/pkg/conversion"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

// Convert_v2_ControlPlaneSpec_To_v1_ControlPlaneSpec converts a v2 ControlPlaneSpec to an equivalent values.yaml.
// XXX: this requires the following additional details:
//      * namespace - the target namespace for the resource
func Convert_v2_ControlPlaneSpec_To_v1_ControlPlaneSpec(in *v2.ControlPlaneSpec, out *v1.ControlPlaneSpec, s conversion.Scope) error {
	if err := autoConvert_v2_ControlPlaneSpec_To_v1_ControlPlaneSpec(in, out, s); err != nil {
		return err
	}

	// Make a copy so we can modify fields as needed
	in = in.DeepCopy()

	// Initialize output
	if out.Istio == nil {
		out.Istio = make(map[string]interface{})
	}

	// Cluster settings
	if err := populateClusterValues(in, out.Istio); err != nil {
		return err
	}

	// Policy
	if err := populatePolicyValues(in, out.Istio); err != nil {
		return err
	}

	return nil
}
