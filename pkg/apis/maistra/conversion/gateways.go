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

	if in.Gateways.Enabled != nil {
		if err := setHelmBoolValue(values, "gateways.enabled", *in.Gateways.Enabled); err != nil {
			return err
		}
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
			if gateways.ClusterIngress.IngressEnabled != nil {
				if err := setHelmBoolValue(values, "global.k8sIngress.enabled", *gateways.ClusterIngress.IngressEnabled); err != nil {
					return err
				}
				hasHTTPS := false
				for _, port := range gateways.ClusterIngress.Service.Ports {
					if port.Port == 443 {
						hasHTTPS = true
						break
					}
				}
				if err := setHelmBoolValue(values, "global.k8sIngress.enableHttps", hasHTTPS); err != nil {
					return err
				}
				if err := setHelmStringValue(values, "global.k8sIngress.gatewayName", "ingressgateway"); err != nil {
					return err
				}
			}
			if err := setHelmValue(gatewayValues, "name", "istio-ingressgateway"); err != nil {
				return err
			}
			if err := setHelmValue(values, "gateways.istio-ingressgateway", gatewayValues); err != nil {
				return err
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

	if gateways.OpenShiftRoute != nil {
		if err := setHelmBoolValue(values, "gateways.istio-ingressgateway.ior_enabled", *gateways.OpenShiftRoute.Enabled); err != nil {
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
	if in.RouterMode != "" {
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
	if len(in.Service.Metadata.Annotations) > 0 {
		if err := setHelmStringMapValue(values, "annotations", in.Service.Metadata.Annotations); err != nil {
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

		if runtime.Container != nil {
			if err := populateContainerConfigValues(runtime.Container, values); err != nil {
				return nil, err
			}
		}
	}

	// Additional volumes
	if len(in.Volumes) > 0 {
		configVolumes := make([]interface{}, 0)
		secretVolumes := make([]interface{}, 0)
		for _, volume := range in.Volumes {
			if volume.Volume.Secret != nil {
				secretVolumes = append(secretVolumes, map[string]interface{}{
					"name":       volume.Mount.Name,
					"secretName": volume.Volume.Secret.SecretName,
					"mountPath":  volume.Mount.MountPath,
				})
			} else if volume.Volume.ConfigMap != nil {
				configVolumes = append(configVolumes, map[string]interface{}{
					"name":          volume.Mount.Name,
					"configMapName": volume.Volume.ConfigMap.Name,
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

	// Always set this to allow round-tripping
	if err := setHelmStringValue(values, "gatewayType", "egress"); err != nil {
		return nil, err
	}

	if in.RequestedNetworkView != nil {
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

	// Always set this to allow round-tripping
	if err := setHelmStringValue(values, "gatewayType", "ingress"); err != nil {
		return nil, err
	}

	// gateway SDS
	if in.SDS != nil {
		sdsValues := make(map[string]interface{})
		if in.SDS.Enabled != nil {
			if err := setHelmBoolValue(sdsValues, "enabled", *in.SDS.Enabled); err != nil {
				return nil, err
			}
		}
		if in.SDS.Runtime != nil {
			// SDS container specific config
			if err := populateContainerConfigValues(in.SDS.Runtime, sdsValues); err != nil {
				return nil, err
			}
		}
		if len(sdsValues) > 0 {
			setHelmValue(values, "sds", sdsValues)
		}
	}

	return values, nil
}

func populateGatewaysConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	gatewaysConfig := &v2.GatewaysConfig{
		EgressGateways:  make(map[string]v2.EgressGatewayConfig),
		IngressGateways: make(map[string]v2.IngressGatewayConfig),
	}
	setGatewaysConfig := false
	if gateways, ok, err := in.GetMap("gateways"); ok {
		for name, gateway := range gateways {
			if name == "enabled" {
				if enabled, ok := gateway.(bool); ok {
					gatewaysConfig.Enabled = &enabled
					setGatewaysConfig = true
				} else {
					return fmt.Errorf("invalid value for gateways.enabled: %v", gateway)
				}
				continue
			}
			gc := v2.GatewayConfig{}
			gatewayMap, ok := gateway.(map[string]interface{})
			if !ok {
				return fmt.Errorf("Failed to parse gateway.%s: cannot cast to map[string]interface{}", name)
			} else if len(gatewayMap) == 0 {
				continue
			}
			setGatewaysConfig = true
			gatewayValues := v1.NewHelmValues(gatewayMap)
			if err := gatewayValuesToConfig(gatewayValues, &gc); err != nil {
				return err
			}

			if isEgressGateway(gatewayValues) || name == "istio-egressgateway" {
				// egress only
				egressGateway := v2.EgressGatewayConfig{
					GatewayConfig: gc,
				}
				if rawNetworkView, ok, err := gatewayValues.GetString("env.ISTIO_META_REQUESTED_NETWORK_VIEW"); ok {
					if rawNetworkView == "" {
						egressGateway.RequestedNetworkView = make([]string, 0)
					} else {
						egressGateway.RequestedNetworkView = strings.Split(rawNetworkView, ",")
					}
				} else if err != nil {
					return err
				}
				if name == "istio-egressgateway" {
					gatewaysConfig.ClusterEgress = &egressGateway
				} else {
					gatewaysConfig.EgressGateways[name] = egressGateway
				}
			} else {
				// assume ingress gateway
				ingressGateway := v2.IngressGatewayConfig{
					GatewayConfig: gc,
				}
				if rawSDSValues, ok, err := gatewayValues.GetMap("sds"); ok && len(rawSDSValues) > 0 {
					sdsValues := v1.NewHelmValues(rawSDSValues)
					sds := &v2.SecretDiscoveryService{}
					if enableSDS, ok, err := sdsValues.GetBool("enabled"); ok {
						sds.Enabled = &enableSDS
						ingressGateway.SDS = sds
					} else if err != nil {
						return err
					}
					sdsContainerConfig := &v2.ContainerConfig{}
					if applied, err := populateContainerConfig(sdsValues, sdsContainerConfig); err != nil {
						return err
					} else if applied {
						sds.Runtime = sdsContainerConfig
						ingressGateway.SDS = sds
					}
				} else if err != nil {
					return err
				}
				if name == "istio-ingressgateway" {
					clusterIngress := v2.ClusterIngressGatewayConfig{
						IngressGatewayConfig: ingressGateway,
					}
					if k8sIngressEnabled, ok, err := in.GetBool("global.k8sIngress.enabled"); ok {
						clusterIngress.IngressEnabled = &k8sIngressEnabled
					} else if err != nil {
						return err
					}
					gatewaysConfig.ClusterIngress = &clusterIngress

					if iorEnabled, ok, err := gatewayValues.GetBool("ior_enabled"); ok {
						gatewaysConfig.OpenShiftRoute = &v2.OpenShiftRouteConfig{
							Enablement: v2.Enablement{
								Enabled: &iorEnabled,
							},
						}
					} else if err != nil {
						return err
					}
				} else if name != "istio-ilbgateway" {
					// ilb gateway is handled by cluster config
					gatewaysConfig.IngressGateways[name] = ingressGateway
				}
			}
		}
	} else if err != nil {
		return err
	}
	if setGatewaysConfig {
		if len(gatewaysConfig.EgressGateways) == 0 {
			gatewaysConfig.EgressGateways = nil
		}
		if len(gatewaysConfig.IngressGateways) == 0 {
			gatewaysConfig.IngressGateways = nil
		}
		out.Gateways = gatewaysConfig
	}
	return nil
}

func isEgressGateway(gatewayValues *v1.HelmValues) bool {
	if gatewayType, ok, _ := gatewayValues.GetString("gatewayType"); ok {
		if gatewayType == "egress" {
			return true
		}
		return false
	}
	_, ok, _ := gatewayValues.GetString("env.ISTIO_META_REQUESTED_NETWORK_VIEW")
	return ok
}

func gatewayValuesToConfig(in *v1.HelmValues, out *v2.GatewayConfig) error {
	if enabled, ok, err := in.GetBool("enabled"); ok {
		out.Enabled = &enabled
	} else if err != nil {
		return err
	}
	if namespace, ok, err := in.GetString("namespace"); ok {
		out.Namespace = namespace
	} else if err != nil {
		return err
	}
	// env.ISTIO_META_ROUTER_MODE
	if routerMode, ok, err := in.GetString("env.ISTIO_META_ROUTER_MODE"); ok {
		out.RouterMode = v2.RouterModeType(routerMode)
	} else if err != nil {
		return err
	}

	// Service-specific config
	out.Service = v2.GatewayServiceConfig{}
	if err := fromValues(in.GetContent(), &out.Service); err != nil {
		return err
	}

	if rawLabels, ok, err := in.GetMap("labels"); ok {
		if err := setMetadataLabels(rawLabels, &out.Service.Metadata); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if rawAnnotations, ok, err := in.GetMap("annotations"); ok {
		if err := setMetadataAnnotations(rawAnnotations, &out.Service.Metadata); err != nil {
			return err
		}
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
					volume.Volume.Secret.SecretName = secretName
				} else if err != nil {
					return err
				}
				if mountPath, ok, err := volumeValues.GetString("mountPath"); ok {
					volume.Mount.MountPath = mountPath
				} else if err != nil {
					return err
				}
				out.Volumes = append(out.Volumes, volume)
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
					volume.Volume.ConfigMap.Name = configMapName
				} else if err != nil {
					return err
				}
				if mountPath, ok, err := volumeValues.GetString("mountPath"); ok {
					volume.Mount.MountPath = mountPath
				} else if err != nil {
					return err
				}
				out.Volumes = append(out.Volumes, volume)
			} else {
				return fmt.Errorf("could not cast secretVolume entry to map[string]interface{}")
			}
		}
	} else if err != nil {
		return err
	}

	// runtime
	runtime := &v2.ComponentRuntimeConfig{}
	if applied, err := runtimeValuesToComponentRuntimeConfig(in, runtime); err != nil {
		return err
	} else if applied {
		out.Runtime = runtime
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
