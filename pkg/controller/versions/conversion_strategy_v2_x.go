package versions

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	meshExpansionPortsV20 = []corev1.ServicePort{
		{
			Name:       "tcp-istiod",
			Port:       15012,
			TargetPort: intstr.FromInt(15012),
		},
		{
			Name:       "tcp-dns-tls",
			Port:       853,
			TargetPort: intstr.FromInt(8853),
		},
	}
)

type v2xConversionStrategy struct{}

func (v *v2xConversionStrategy) GetExpansionPorts() []corev1.ServicePort {
	return meshExpansionPortsV20
}

func (v *v2xConversionStrategy) GetTelemetryType(in *v1.HelmValues, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled bool) v2.TelemetryType {
	remoteTelemetryAddress, _, _ := in.GetString("global.remoteTelemetryAddress")
	if remoteEnabled || remoteTelemetryAddress != "" {
		// special case if copying over an old v1 resource and bumping the version to v2.0
		return v2.TelemetryTypeRemote
	} else {
		// leave the defaults
		return ""
	}
}

func (v *v2xConversionStrategy) GetPolicyType(in *v1.HelmValues, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled bool) v2.PolicyType {
	remotePolicyAddress, _, _ := in.GetString("global.remotePolicyAddress")
	if remoteEnabled || remotePolicyAddress != "" {
		// special case if copying over an old v1 resource an bumping the version to v2.0
		return v2.PolicyTypeRemote
	} else {
		// leave the defaults
		return ""
	}
}
