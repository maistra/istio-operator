package conversion

import (
	"fmt"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

const (
	searchSuffixGlobal                  = "global"
	searchSuffixNamespaceGlobalTemplate = "{{ valueOrDefault .DeploymentMeta.Namespace \"%s\" }}.global"

	clusterDomainDefault = "cluster.local"
)

// populateClusterValues popluates values.yaml specific to clustering.  this
// function will also update fields in other settings that are related to
// clustering, e.g. MeshExpansionPorts on Ingress gateway and DNS search
// search suffixes for Proxy.
func populateClusterValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	// Cluster settings
	// non-configurable defaults
	// XXX: not sure if this is version specific, i.e. does it apply to istio 1.6?
	if err := setHelmBoolValue(values, "global.useMCP", true); err != nil {
		return err
	}

	cluster := in.Cluster
	if cluster == nil {
		cluster = &v2.ControlPlaneClusterConfig{}
	}

	// TODO: figure out how to get the install namespace
	var namespace string
	clusterDomain := clusterDomainDefault
	if in.Proxy != nil && in.Proxy.Networking.ClusterDomain != "" {
		clusterDomain = in.Proxy.Networking.ClusterDomain
	}
	hasClusterName := len(cluster.Name) > 0
	hasNetworkName := len(cluster.Network) > 0
	if hasClusterName {
		if err := setHelmStringValue(values, "global.multiCluster.clusterName", cluster.Name); err != nil {
			return err
		}
	}
	if hasNetworkName {
		if err := setHelmStringValue(values, "global.network", cluster.Network); err != nil {
			return err
		}
	}

	multiClusterEnabled := false
	if cluster.MultiCluster == nil {
		if err := setHelmBoolValue(values, "global.multiCluster.enabled", false); err != nil {
			return err
		}
	} else {
		// multi-cluster
		// meshExpansion is always enabled for multi-cluster
		multiClusterEnabled = true
		if err := setHelmBoolValue(values, "global.multiCluster.enabled", true); err != nil {
			return err
		}
		if hasClusterName && hasNetworkName {
			// Configure local mesh network, if not defined
			if cluster.MultiCluster.MeshNetworks == nil {
				cluster.MultiCluster.MeshNetworks = make(map[string]v2.MeshNetworkConfig)
			}
			if _, ok := cluster.MultiCluster.MeshNetworks[cluster.Network]; !ok {
				// XXX: do we need to make sure ingress gateways is configured and includes port 443?
				cluster.MultiCluster.MeshNetworks[cluster.Network] = v2.MeshNetworkConfig{
					Endpoints: []v2.MeshEndpointConfig{
						{
							FromRegistry: cluster.Name,
						},
					},
					Gateways: []v2.MeshGatewayConfig{
						{
							// XXX: should we check to see if ilb gateway is being used instead?
							// XXX: this should be the gateway namespace or the control plane namespace
							Service: getLocalNetworkService("istio-ingressgateway", namespace, clusterDomain),
							Port:    443,
						},
					},
				}
			}

			if meshNetworksValue, err := toValues(cluster.MultiCluster.MeshNetworks); err == nil {
				if len(meshNetworksValue) > 0 {
					if err := setHelmValue(values, "global.meshNetworks", meshNetworksValue); err != nil {
						return err
					}
				}
			} else {
				return err
			}
		}

		// XXX: ingress and egress gateways must be configured if multicluster is enabled
		if in.Gateways == nil {
			enabled := true
			in.Gateways = &v2.GatewaysConfig{
				ClusterEgress: &v2.EgressGatewayConfig{
					GatewayConfig: v2.GatewayConfig{
						Enablement: v2.Enablement{
							Enabled: &enabled,
						},
					},
				},
			}
		} else {
			if in.Gateways.ClusterEgress == nil {
				enabled := true
				in.Gateways.ClusterEgress = &v2.EgressGatewayConfig{
					GatewayConfig: v2.GatewayConfig{
						Enablement: v2.Enablement{
							Enabled: &enabled,
						},
					},
				}
			}
		}
		foundExternal := false
		for _, network := range in.Gateways.ClusterEgress.RequestedNetworkView {
			if network == "external" {
				foundExternal = true
				break
			}
		}
		if !foundExternal {
			in.Gateways.ClusterEgress.RequestedNetworkView = append(in.Gateways.ClusterEgress.RequestedNetworkView, "external")
		}
		// XXX: i think this should be moved to Gateways.ClusterIngress
		// XXX: how does this interact with IOR
		if cluster.MultiCluster.Ingress {
			if err := setHelmBoolValue(values, "global.k8sIngress.enabled", true); err != nil {
				return err
			}
			if in.Gateways != nil && in.Gateways.ClusterIngress != nil {
				hasHTTPS := false
				for _, port := range in.Gateways.ClusterIngress.Service.Ports {
					if port.Port == 443 {
						hasHTTPS = true
						break
					}
				}
				if err := setHelmBoolValue(values, "global.k8sIngress.enableHttps", hasHTTPS); err != nil {
					return err
				}
			}
		}
	}
	if multiClusterEnabled || cluster.MeshExpansion != nil {
		if err := setHelmBoolValue(values, "global.meshExpansion.enabled", true); err != nil {
			return err
		}
		if expansionPorts, err := expansionPortsForVersion(in.Version); err == nil {
			if cluster.MeshExpansion == nil || cluster.MeshExpansion.ILBGateway == nil ||
				cluster.MeshExpansion.ILBGateway.Enabled == nil || !*cluster.MeshExpansion.ILBGateway.Enabled {
				if in.Gateways.ClusterIngress == nil {
					enabled := true
					in.Gateways.ClusterIngress = &v2.ClusterIngressGatewayConfig{
						IngressGatewayConfig: v2.IngressGatewayConfig{
							GatewayConfig: v2.GatewayConfig{
								Enablement: v2.Enablement{
									Enabled: &enabled,
								},
							},
						},
					}
				}
				addExpansionPorts(&in.Gateways.ClusterIngress.MeshExpansionPorts, expansionPorts)
				if err := setHelmBoolValue(values, "gateways.istio-ilbgateway.enabled", false); err != nil {
					return err
				}
				if err := setHelmBoolValue(values, "global.meshExpansion.useILB", false); err != nil {
					return err
				}
			} else {
				if err := setHelmBoolValue(values, "global.meshExpansion.useILB", true); err != nil {
					return err
				}
				addExpansionPorts(&cluster.MeshExpansion.ILBGateway.Service.Ports, expansionPorts)
				if ilbGatewayValues, err := gatewayConfigToValues(cluster.MeshExpansion.ILBGateway); err == nil {
					if err := setHelmValue(values, "gateways.istio-ilbgateway", ilbGatewayValues); err != nil {
						return err
					}
				} else {
					return err
				}
			}
		} else {
			return err
		}
	} else {
		if err := setHelmBoolValue(values, "global.meshExpansion.enabled", false); err != nil {
			return err
		}
		if err := setHelmBoolValue(values, "global.meshExpansion.useILB", false); err != nil {
			return err
		}
		ilbDisabled := false
		cluster.MeshExpansion = &v2.MeshExpansionConfig{
			ILBGateway: &v2.GatewayConfig{
				Enablement: v2.Enablement{
					Enabled: &ilbDisabled,
				},
			},
		}
	}

	if multiClusterEnabled {
		// Configure DNS search suffixes for "global"
		globalIndex := -1
		deploymentMetadataIndex := -1
		if in.Proxy == nil {
			in.Proxy = &v2.ProxyConfig{}
		} else {
			for index, ss := range in.Proxy.Networking.DNS.SearchSuffixes {
				if ss == searchSuffixGlobal {
					globalIndex = index
				} else if strings.Index(ss, ".DeploymentMeta.Namespace") > 0 { // greater than works here because the template must be bracketed with {{ }}
					deploymentMetadataIndex = index
				}
			}
		}
		if deploymentMetadataIndex < 0 {
			namespaceSuffix := fmt.Sprintf(searchSuffixNamespaceGlobalTemplate, namespace)
			if globalIndex < 0 {
				in.Proxy.Networking.DNS.SearchSuffixes = append([]string{namespaceSuffix}, in.Proxy.Networking.DNS.SearchSuffixes...)
			} else {
				in.Proxy.Networking.DNS.SearchSuffixes = append(append(in.Proxy.Networking.DNS.SearchSuffixes[:globalIndex+1], namespaceSuffix), in.Proxy.Networking.DNS.SearchSuffixes[globalIndex+1:]...)
			}
		}
		if globalIndex < 0 {
			in.Proxy.Networking.DNS.SearchSuffixes = append([]string{searchSuffixGlobal}, in.Proxy.Networking.DNS.SearchSuffixes...)
		}
	}

	return nil
}

func getLocalNetworkService(gatewayService, namespace, clusterDomain string) string {
	return fmt.Sprintf("%s.%s.svc.%s", gatewayService, namespace, clusterDomain)
}

func populateClusterConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	clusterConfig := &v2.ControlPlaneClusterConfig{}
	setClusterConfig := false

	if clusterName, ok, err := in.GetString("global.multiCluster.clusterName"); ok {
		clusterConfig.Name = clusterName
		setClusterConfig = true
	} else if err != nil {
		return err
	}
	if network, ok, err := in.GetString("global.network"); ok {
		clusterConfig.Network = network
		setClusterConfig = true
	} else if err != nil {
		return err
	}

	// multi-cluster
	if multiClusterEnabled, ok, err := in.GetBool("global.multiCluster.enabled"); ok && multiClusterEnabled {
		clusterConfig.MultiCluster = &v2.MultiClusterConfig{}
		setClusterConfig = true
		if rawMeshNetworks, ok, err := in.GetMap("global.meshNetworks"); ok && len(rawMeshNetworks) > 0 {
			clusterConfig.MultiCluster.MeshNetworks = make(map[string]v2.MeshNetworkConfig)
			if err := fromValues(rawMeshNetworks, &clusterConfig.MultiCluster.MeshNetworks); err != nil {
				return err
			}
			// remove defaulted mesh network
			if localNetwork, ok := clusterConfig.MultiCluster.MeshNetworks[clusterConfig.Network]; ok {
				if len(localNetwork.Endpoints) == 1 && localNetwork.Endpoints[0].FromRegistry == clusterConfig.Name {
					// XXX: we need this
					namespace := ""
					// XXX: should this be ilb, if it's enabled?
					gatewayName := "istio-ingressgateway"
					clusterDomain, ok, err := in.GetString("global.proxy.clusterDomain")
					if err != nil {
						return err
					} else if !ok || clusterDomain == "" {
						clusterDomain = clusterDomainDefault
					}
					if len(localNetwork.Gateways) == 1 && localNetwork.Gateways[0].Port == 443 &&
						localNetwork.Gateways[0].Service == getLocalNetworkService(gatewayName, namespace, clusterDomain) &&
						localNetwork.Gateways[0].Address == "" {
						delete(clusterConfig.MultiCluster.MeshNetworks, clusterConfig.Network)
						if len(clusterConfig.MultiCluster.MeshNetworks) == 0 {
							clusterConfig.MultiCluster.MeshNetworks = nil
						}
					}
				}
			}
		} else if err != nil {
			return err
		}
		if ingressEnabled, ok, err := in.GetBool("global.k8sIngress.enabled"); ok {
			clusterConfig.MultiCluster.Ingress = ingressEnabled
		} else if err != nil {
			return err
		}
		if expansionEnabled, ok, err := in.GetBool("global.meshExpansion.enabled"); ok && expansionEnabled {
			meshExpansionConfig := &v2.MeshExpansionConfig{}
			setMeshExpansion := false
			if useILBGateway, ok, err := in.GetBool("global.meshExpansion.useILB"); ok && useILBGateway {
				setMeshExpansion = true
				meshExpansionConfig.ILBGateway = &v2.GatewayConfig{
					Enablement: v2.Enablement{
						Enabled: &useILBGateway,
					},
				}
				if ilbGatewayValues, ok, err := in.GetMap("gateways.istio-ilbgateway"); ok && len(ilbGatewayValues) > 0 {
					if err := gatewayValuesToConfig(v1.NewHelmValues(ilbGatewayValues), meshExpansionConfig.ILBGateway); err != nil {
						return err
					}
				} else if err != nil {
					return err
				}
			} else if err != nil {
				return nil
			}
			if !multiClusterEnabled || setMeshExpansion {
				clusterConfig.MeshExpansion = meshExpansionConfig
				setClusterConfig = true
			}
		} else if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if setClusterConfig {
		out.Cluster = clusterConfig
	}

	return nil
}
