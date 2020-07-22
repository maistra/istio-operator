package conversion

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
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
		if err := setHelmStringSliceValue(values, "loadBalancerSourceRanges", in.Service.LoadBalancerSourceRanges); err != nil {
			return nil, err
		}
	}
	if in.Service.ExternalTrafficPolicy != "" {
		if err := setHelmStringValue(values, "externalTrafficPolicy", string(in.Service.ExternalTrafficPolicy)); err != nil {
			return nil, err
		}
	}
	if len(in.Service.ExternalIPs) > 0 {
		if err := setHelmStringSliceValue(values, "externalIPs", in.Service.ExternalIPs); err != nil {
			return nil, err
		}
	}
	if in.Service.Type != "" {
		if err := setHelmStringValue(values, "type", string(in.Service.Type)); err != nil {
			return nil, err
		}
	}
	if len(in.Service.Metadata.Labels) > 0 {
		if err := setHelmStringMapValue(values, "labels", in.Service.Metadata.Labels); err != nil {
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
			if volume.Volume.Secret != nil {
				secretVolumes = append(secretVolumes, map[string]string{
					"name":       volume.Mount.Name,
					"secretName": volume.Volume.Name,
					"mountPath":  volume.Mount.MountPath,
				})
			} else if volume.Volume.ConfigMap != nil {
				configVolumes = append(configVolumes, map[string]string{
					"name":          volume.Mount.Name,
					"configMapName": volume.Volume.Name,
					"mountPath":     volume.Mount.MountPath,
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
		if err := setHelmStringValue(values, "env.ISTIO_META_REQUESTED_NETWORK_VIEW", strings.Join(in.RequestedNetworkView, ",")); err != nil {
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

func populateGatewaysConfig(in map[string]interface{}, out *v2.GatewaysConfig) error {
	gateways, ok := in["gateways"].(map[string]interface{})
	if ok {
		for name, gateway := range gateways {
			gc := v2.GatewayConfig{}
			gatewayMap, ok := gateway.(map[string]interface{})
			if !ok {
				return fmt.Errorf("Failed to parse gateway.%s: cannot cast to map[string]interface{}", name)
			}
			gatewayValues := v1.NewHelmValues(gatewayMap)
			if err := gatewayValuesToConfig(gatewayValues, &gc); err != nil {
				return err
			}

			// egress only
			if networkView, ok, err := gatewayValues.GetString("env.ISTIO_META_REQUESTED_NETWORK_VIEW"); ok {
				egressGateway := v2.EgressGatewayConfig{
					GatewayConfig:        gc,
					RequestedNetworkView: strings.Split(networkView, ","),
				}
				if name == "istio-egressgateway" {
					out.ClusterEgress = &egressGateway
				} else {
					out.EgressGateways[name] = egressGateway
				}
			} else if err != nil {
				return err
			} else {
				// assume ingress gateway
				ingressGateway := v2.IngressGatewayConfig{
					GatewayConfig: gc,
				}
				if enableSDS, ok, err := gatewayValues.GetBool("sds.enabled"); ok {
					ingressGateway.EnableSDS = &enableSDS
				} else if err != nil {
					return err
				}
				sdsContainerConfig := v2.ContainerConfig{}
				setSDSContainerConfig := false
				if sdsImage, ok, err := gatewayValues.GetString("sds.image"); ok {
					sdsContainerConfig.Image = sdsImage
					setSDSContainerConfig = true
				} else if err != nil {
					return err
				}
				if resourcesValues, ok, err := gatewayValues.GetMap("sds.resources"); ok {
					resources := &corev1.ResourceRequirements{}
					if err := fromValues(resourcesValues, resources); err != nil {
						return err
					}
					sdsContainerConfig.Resources = resources
					setSDSContainerConfig = true
				} else if err != nil {
					return err
				}
				if setSDSContainerConfig {
					ingressGateway.Runtime.Pod.Containers["ingress-sds"] = sdsContainerConfig
				}
				if name == "istio-ingressgateway" {
					clusterIngress := v2.ClusterIngressGatewayConfig{
						IngressGatewayConfig: ingressGateway,
					}
					out.ClusterIngress = &clusterIngress
				} else {
					out.IngressGateways[name] = ingressGateway
				}
			}
		}
	}
	return nil
}

func gatewayValuesToConfig(in *v1.HelmValues, out *v2.GatewayConfig) error {
	gatewayConfig := v2.GatewayConfig{}
	if enabled, ok, err := in.GetBool("enabled"); ok {
		gatewayConfig.Enabled = &enabled
	} else if err != nil {
		return err
	}
	if namespace, ok, err := in.GetString("namespace"); ok {
		gatewayConfig.Namespace = namespace
	} else if err != nil {
		return err
	}
	// env.ISTIO_META_ROUTER_MODE
	if routerMode, ok, err := in.GetString("env.ISTIO_META_ROUTER_MODE"); ok {
		gatewayConfig.RouterMode = v2.RouterModeType(routerMode)
	} else if err != nil {
		return err
	}

	// Service-specific config
	gatewayConfig.Service = v2.GatewayServiceConfig{}
	if err := fromValues(in.GetContent(), &gatewayConfig.Service); err != nil {
		return err
	}

	if rawLabels, ok, err := in.GetMap("labels"); ok {
		labels := make(map[string]string)
		for key, value := range rawLabels {
			if stringValue, ok := value.(string); ok {
				labels[key] = stringValue
			} else {
				return fmt.Errorf("non string value in labels definition")
			}
		}
		gatewayConfig.Service.Metadata.Labels = labels
	} else if err != nil {
		return err
	}

	// volumes
	if secretVolumes, ok, err := in.GetSlice("secretVolumes"); ok {
		for _, rawSecretVolume := range secretVolumes {
			if secretVolume, ok := rawSecretVolume.(map[string]interface{}); ok {
				volumeValues := v1.NewHelmValues(secretVolume)
				volume := v2.VolumeConfig{
					Volume: v2.GatewayVolume{
						Secret: &corev1.SecretVolumeSource{},
					},
				}
				if name, ok, err := volumeValues.GetString("name"); ok {
					volume.Mount.Name = name
				} else if err != nil {
					return err
				}
				if secretName, ok, err := volumeValues.GetString("secretName"); ok {
					volume.Volume.Name = secretName
				} else if err != nil {
					return err
				}
				if mountPath, ok, err := volumeValues.GetString("mountPath"); ok {
					volume.Mount.MountPath = mountPath
				} else if err != nil {
					return err
				}
				gatewayConfig.Volumes = append(out.Volumes, volume)
			} else {
				return fmt.Errorf("could not cast secretVolume entry to map[string]interface{}")
			}
		}
	} else if err != nil {
		return err
	}
	if configVolumes, ok, err := in.GetSlice("configVolumes"); ok {
		for _, rawConfigVolume := range configVolumes {
			if configVolume, ok := rawConfigVolume.(map[string]interface{}); ok {
				volumeValues := v1.NewHelmValues(configVolume)
				volume := v2.VolumeConfig{
					Volume: v2.GatewayVolume{
						ConfigMap: &corev1.ConfigMapVolumeSource{},
					},
				}
				if name, ok, err := volumeValues.GetString("name"); ok {
					volume.Mount.Name = name
				} else if err != nil {
					return err
				}
				if configMapName, ok, err := volumeValues.GetString("configMapName"); ok {
					volume.Volume.Name = configMapName
				} else if err != nil {
					return err
				}
				if mountPath, ok, err := volumeValues.GetString("mountPath"); ok {
					volume.Mount.MountPath = mountPath
				} else if err != nil {
					return err
				}
				gatewayConfig.Volumes = append(out.Volumes, volume)
			} else {
				return fmt.Errorf("could not cast secretVolume entry to map[string]interface{}")
			}
		}
	} else if err != nil {
		return err
	}

	// runtime
	gatewayConfig.Runtime = &v2.ComponentRuntimeConfig{}
	if err := runtimeValuesToComponentRuntimeConfig(in, gatewayConfig.Runtime); err != nil {
		return err
	}

	// container settings
	if resourcesValues, ok, err := in.GetMap("resources"); ok {
		resources := &corev1.ResourceRequirements{}
		if err := fromValues(resourcesValues, resources); err != nil {
			return err
		}
		gatewayConfig.Runtime.Pod.Containers["istio-proxy"] = v2.ContainerConfig{
			CommonContainerConfig: v2.CommonContainerConfig{
				Resources: resources,
			},
		}
	} else if err != nil {
		return err
	}

	return nil
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
