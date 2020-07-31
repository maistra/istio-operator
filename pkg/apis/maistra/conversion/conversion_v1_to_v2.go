package conversion

import (
	conversion "k8s.io/apimachinery/pkg/conversion"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

// Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec converts a v1 ControlPlaneSpec to its v2 equivalent.
func Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(in *v1.ControlPlaneSpec, out *v2.ControlPlaneSpec, s conversion.Scope) error {

	version, versionErr := versions.ParseVersion(in.Version)
	if versionErr != nil {
		return versionErr
	}
	if err := autoConvert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(in, out, s); err != nil {
		return err
	}

	// Cluster settings
	if err := populateClusterConfig(in.Istio, out); err != nil {
		return err
	}

	// Logging
	if err := populateControlPlaneLoggingConfig(in.Istio, out); err != nil {
		return err
	}

	// Policy
	if err := populatePolicyConfig(in.Istio, out, version); err != nil {
		return err
	}

	// Proxy
	if err := populateProxyConfig(in.Istio, out); err != nil {
		return err
	}

	// Security
	if err := populateSecurityConfig(in.Istio, out); err != nil {
		return err
	}

	// Telemetry
	if err := populateTelemetryConfig(in.Istio, out); err != nil {
		return err
	}

	// Gateways
	if err := populateGatewaysConfig(in.Istio, out); err != nil {
		return err
	}

	// Runtime
	if _, err := populateControlPlaneRuntimeConfig(in.Istio, out); err != nil {
		return err
	}

	// Addons
	if err := populateAddonsConfig(in.Istio, out); err != nil {
		return err
	}

	return nil
}

func Convert_v1_ControlPlaneStatus_To_v2_ControlPlaneStatus(in *v1.ControlPlaneStatus, out *v2.ControlPlaneStatus, s conversion.Scope) error {
	in.DeepCopyInto(&out.ControlPlaneStatus)
	return nil
}
