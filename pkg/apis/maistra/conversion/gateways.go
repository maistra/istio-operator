package conversion

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

const (
	imageNameSDS = "node-agent-k8s"
)

var (
	meshExpansionPortsV11 = []corev1.ServicePort{
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

func populateGatewaysValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	if in.Gateways == nil {
		return nil
	}

	if err := setHelmBoolValue(values, "gateways.enabled", true); err != nil {
		return err
	}

	gateways := in.Gateways

	if gateways.ClusterIngress != nil {
		if gatewayValues, err := gatewayIngressConfigToValues(&gateways.ClusterIngress.IngressGatewayConfig); err == nil {
			if len(gateways.ClusterIngress.MeshExpansionPorts) > 0 {
				untypedSlice := make([]interface{}, len(gateways.ClusterIngress.MeshExpansionPorts))
				for index, port := range gateways.ClusterIngress.MeshExpansionPorts {
					untypedSlice[index] = port
				}
				if portsValue, err := sliceToValues(untypedSlice); err == nil {
					if len(portsValue) > 0 {
						if err := setHelmValue(gatewayValues, "meshExpansionPorts", portsValue); err != nil {
							return err
						}
					}
				} else {
					return err
				}
			}
			if len(gatewayValues) > 0 {
				if err := setHelmValue(gatewayValues, "name", "istio-ingressgateway"); err != nil {
					return err
				}
				if err := setHelmValue(values, "gateways.istio-ingressgateway", gatewayValues); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}

	if gateways.ClusterEgress != nil {
		if gatewayValues, err := gatewayEgressConfigToValues(gateways.ClusterEgress); err == nil {
			if len(gatewayValues) > 0 {
				if err := setHelmValue(gatewayValues, "name", "istio-egressgateway"); err != nil {
					return err
				}
				if err := setHelmValue(values, "gateways.istio-egressgateway", gatewayValues); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}

	for name, gateway := range gateways.IngressGateways {
		if gatewayValues, err := gatewayIngressConfigToValues(&gateway); err == nil {
			if len(gatewayValues) > 0 {
				if err := setHelmValue(gatewayValues, "name", name); err != nil {
					return err
				}
				if err := setHelmValue(values, "gateways."+name, gatewayValues); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}

	for name, gateway := range gateways.EgressGateways {
		if gatewayValues, err := gatewayEgressConfigToValues(&gateway); err == nil {
			if len(gatewayValues) > 0 {
				if err := setHelmValue(gatewayValues, "name", name); err != nil {
					return err
				}
				if err := setHelmValue(values, "gateways."+name, gatewayValues); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	return nil
}

// converts v2.GatewayConfig to values.yaml
func gatewayConfigToValues(in *v2.GatewayConfig) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	if in.Enabled != nil {
		if err := setHelmBoolValue(values, "enabled", *in.Enabled); err != nil {
			return nil, err
		}
	}

	if in.Namespace != "" {
		if err := setHelmStringValue(values, "namespace", in.Namespace); err != nil {
			return nil, err
		}
	}
	if in.RouterMode == "" {
		if err := setHelmStringValue(values, "env.ISTIO_META_ROUTER_MODE", string(v2.RouterModeTypeSNIDNAT)); err != nil {
			return nil, err
		}
	} else {
		if err := setHelmStringValue(values, "env.ISTIO_META_ROUTER_MODE", string(in.RouterMode)); err != nil {
			return nil, err
		}
	}

	// Service specific settings
	if in.Service.LoadBalancerIP != "" {
		if err := setHelmStringValue(values, "loadBalancerIP", in.Service.LoadBalancerIP); err != nil {
			return nil, err
		}
	}
	if len(in.Service.LoadBalancerSourceRanges) > 0 {
		if err := setHelmSliceValue(values, "loadBalancerSourceRanges", in.Service.LoadBalancerSourceRanges); err != nil {
			return nil, err
		}
	}
	if in.Service.ExternalTrafficPolicy != "" {
		if err := setHelmStringValue(values, "externalTrafficPolicy", string(in.Service.ExternalTrafficPolicy)); err != nil {
			return nil, err
		}
	}
	if len(in.Service.ExternalIPs) > 0 {
		if err := setHelmSliceValue(values, "externalIPs", in.Service.ExternalIPs); err != nil {
			return nil, err
		}
	}
	if in.Service.Type != "" {
		if err := setHelmStringValue(values, "type", string(in.Service.Type)); err != nil {
			return nil, err
		}
	}
	if len(in.Service.Metadata.Labels) > 0 {
		if err := setHelmMapValue(values, "labels", in.Service.Metadata.Labels); err != nil {
			return nil, err
		}
	}
	if len(in.Service.Ports) > 0 {
		untypedSlice := make([]interface{}, len(in.Service.Ports))
		for index, port := range in.Service.Ports {
			untypedSlice[index] = port
		}
		if portsValue, err := sliceToValues(untypedSlice); err == nil {
			if len(portsValue) > 0 {
				if err := setHelmValue(values, "ports", portsValue); err != nil {
					return nil, err
				}
			}
		} else {
			return nil, err
		}
	}

	// Deployment specific settings
	if in.Runtime != nil {
		runtime := in.Runtime
		if err := populateRuntimeValues(runtime, values); err != nil {
			return nil, err
		}

		if runtime.Pod.Containers != nil {
			// Proxy container specific config
			if proxyContainer, ok := runtime.Pod.Containers["istio-proxy"]; ok {
				if proxyContainer.Resources != nil {
					if resourcesValues, err := toValues(proxyContainer.Resources); err == nil {
						if len(resourcesValues) > 0 {
							if err := setHelmValue(values, "resources", resourcesValues); err != nil {
								return nil, err
							}
						}
					} else {
						return nil, err
					}
				}
			}
		}
	}

	// Additional volumes
	if len(in.Volumes) > 0 {
		configVolumes := make([]map[string]string, 0)
		secretVolumes := make([]map[string]string, 0)
		for _, volume := range in.Volumes {
			if volume.Volume.ConfigMap != nil {
				configVolumes = append(configVolumes, map[string]string{
					"name":          volume.Mount.Name,
					"configMapName": volume.Volume.Name,
					"mountPath":     volume.Mount.MountPath,
				})
			} else if volume.Volume.Secret != nil {
				secretVolumes = append(secretVolumes, map[string]string{
					"name":       volume.Mount.Name,
					"secretName": volume.Volume.Name,
					"mountPath":  volume.Mount.MountPath,
				})
			} else {
				// XXX: ignore misconfigured volumes?
			}
		}
		if len(configVolumes) > 0 {
			if err := setHelmValue(values, "configVolumes", configVolumes); err != nil {
				return nil, err
			}
		}
		if len(secretVolumes) > 0 {
			if err := setHelmValue(values, "secretVolumes", secretVolumes); err != nil {
				return nil, err
			}
		}
	}
	return values, nil
}

func gatewayEgressConfigToValues(in *v2.EgressGatewayConfig) (map[string]interface{}, error) {
	values, err := gatewayConfigToValues(&in.GatewayConfig)
	if err != nil {
		return nil, err
	}

	if len(in.RequestedNetworkView) > 0 {
		if err := setHelmStringValue(values, "env.ISTIO_META_REQUESTED_NETWORK_VIEW", fmt.Sprintf("\"%s\"", strings.Join(in.RequestedNetworkView, ","))); err != nil {
			return nil, err
		}
	}

	return values, nil
}

func gatewayIngressConfigToValues(in *v2.IngressGatewayConfig) (map[string]interface{}, error) {
	values, err := gatewayConfigToValues(&in.GatewayConfig)
	if err != nil {
		return nil, err
	}

	// gateway SDS
	if in.EnableSDS != nil {
		if err := setHelmBoolValue(values, "sds.enable", *in.EnableSDS); err != nil {
			return nil, err
		}
	}

	if in.Runtime != nil {
		runtime := in.Runtime

		if runtime.Pod.Containers != nil {
			// SDS container specific config
			if sdsContainer, ok := runtime.Pod.Containers["ingress-sds"]; ok {
				if sdsContainer.Image != "" {
					if err := setHelmStringValue(values, "sds.image", sdsContainer.Image); err != nil {
						return nil, err
					}
				}
				if sdsContainer.Resources != nil {
					if resourcesValues, err := toValues(sdsContainer.Resources); err == nil {
						if len(resourcesValues) > 0 {
							if err := setHelmValue(values, "sds.resources", resourcesValues); err != nil {
								return nil, err
							}
						}
					} else {
						return nil, err
					}
				}
			}
		}
	}

	return values, nil
}

func expansionPortsForVersion(version string) ([]corev1.ServicePort, error) {
	switch version {
	case "":
		fallthrough
	case versions.V1_0.String():
		fallthrough
	case versions.V1_1.String():
		return meshExpansionPortsV11, nil
	case versions.V2_0.String():
		return meshExpansionPortsV20, nil
	default:
		return nil, fmt.Errorf("cannot convert for unknown version: %s", version)
	}
}
func addExpansionPorts(in *[]corev1.ServicePort, ports []corev1.ServicePort) {
	portCount := len(*in)
PortsLoop:
	for _, port := range ports {
		for index := 0; index < portCount; index++ {
			if port.Port == (*in)[index].Port {
				continue PortsLoop
			}
			*in = append(*in, port)
		}
	}
}
