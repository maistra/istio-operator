package versions

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

var meshExpansionPortsV11 = []corev1.ServicePort{
	{
		Name:       "tcp-pilot-grpc-tls",
		Port:       15011,
		TargetPort: intstr.FromInt(15011),
	},
	{
		Name:       "tcp-mixer-grpc-tls",
		Port:       15004,
		TargetPort: intstr.FromInt(15004),
	},
	{
		Name:       "tcp-citadel-grpc-tls",
		Port:       8060,
		TargetPort: intstr.FromInt(8060),
	},
	{
		Name:       "tcp-dns-tls",
		Port:       853,
		TargetPort: intstr.FromInt(8853),
	},
}

type v1xConversionStrategy struct{}

func (v *v1xConversionStrategy) GetExpansionPorts() []corev1.ServicePort {
	return meshExpansionPortsV11
}

func (v *v1xConversionStrategy) GetTelemetryType(
	_ *maistrav1.HelmValues, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled bool,
) maistrav2.TelemetryType {
	if remoteEnabled {
		// using remote telemetry, which takes precedence over mixer (in the charts, at least)
		return maistrav2.TelemetryTypeRemote
	} else if mixerTelemetryEnabled {
		// mixer telemetry explicitly enabled
		return maistrav2.TelemetryTypeMixer
	} else if mixerTelemetryEnabledSet {
		// mixer is explicitly disabled
		return maistrav2.TelemetryTypeNone
	} else {
		// don't set telemetry type, let the defaults do their thing
		return ""
	}
}

func (v *v1xConversionStrategy) GetPolicyType(in *maistrav1.HelmValues, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled bool) maistrav2.PolicyType {
	if remoteEnabled {
		// using remote policy, which takes precedence over mixer (in the charts, at least)
		return maistrav2.PolicyTypeRemote
	} else if mixerPolicyEnabled {
		// mixer policy explicitly enabled
		return maistrav2.PolicyTypeMixer
	} else if mixerPolicyEnabledSet {
		// mixer is explicitly disabled
		return maistrav2.PolicyTypeNone
	} else {
		// don't set policy type, let the defaults do their thing
		return ""
	}
}

func (v *v1xConversionStrategy) GetTrustDomainFieldPath() string {
	return "global.trustDomain"
}
