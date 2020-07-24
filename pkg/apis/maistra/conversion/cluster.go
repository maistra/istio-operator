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

	if in.Cluster == nil {
		return nil
	}

	// TODO: figure out how to get the install namespace
	var namespace string
	clusterDomain := clusterDomainDefault
	if in.Proxy.Networking.ClusterDomain != "" {
		clusterDomain = in.Proxy.Networking.ClusterDomain
	}
	hasClusterName := len(in.Cluster.Name) > 0
	hasNetworkName := len(in.Cluster.Network) > 0
	if hasClusterName {
		if err := setHelmStringValue(values, "global.multiCluster.clusterName", in.Cluster.Name); err != nil {
			return err
		}
	}
	if hasNetworkName {
		if err := setHelmStringValue(values, "global.network", in.Cluster.Network); err != nil {
			return err
		}
	}
	if in.Cluster.MultiCluster == nil {
		if err := setHelmBoolValue(values, "global.multiCluster.enabled", false); err != nil {
			return err
		}
	} else {
		if err := setHelmBoolValue(values, "global.multiCluster.enabled", true); err != nil {
			return err
		}
		if hasClusterName && hasNetworkName {
			// Configure local mesh network, if not defined
			if in.Cluster.MultiCluster.MeshNetworks == nil {
				in.Cluster.MultiCluster.MeshNetworks = make(map[string]v2.MeshNetworkConfig)
			}
			if _, ok := in.Cluster.MultiCluster.MeshNetworks[in.Cluster.Network]; !ok {
				// XXX: do we need to make sure ingress gateways is configured and includes port 443?
				in.Cluster.MultiCluster.MeshNetworks[in.Cluster.Network] = v2.MeshNetworkConfig{
					Endpoints: []v2.MeshEndpointConfig{
						{
							FromRegistry: in.Cluster.Name,
						},
					},
					Gateways: []v2.MeshGatewayConfig{
						{
							// XXX: should we check to see if ilb gateway is being used instead?
							// XXX: this should be the gateway namespace or the control plane namespace
							Service: fmt.Sprintf("istio-ingressgateway.%s.svc.%s", namespace, clusterDomain),
							Port:    443,
						},
					},
				}
			}

			if meshNetworksValue, err := toValues(in.Cluster.MultiCluster.MeshNetworks); err == nil {
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
		if in.Gateways != nil {
			if in.Gateways.ClusterEgress != nil {
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
			}
			if in.Gateways.ClusterIngress != nil {
				if in.Cluster.MultiCluster.Ingress {
					if err := setHelmBoolValue(values, "global.k8sIngress.enabled", true); err != nil {
						return err
					}
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
				// meshExpansion is always enabled for multi-cluster
				if err := setHelmBoolValue(values, "global.meshExpansion.enabled", true); err != nil {
					return err
				}
				if expansionPorts, err := expansionPortsForVersion(in.Version); err != nil {
					if in.Cluster.MeshExpansion == nil || in.Cluster.MeshExpansion.ILBGateway == nil ||
						in.Cluster.MeshExpansion.ILBGateway.Enabled == nil || !*in.Cluster.MeshExpansion.ILBGateway.Enabled {
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
						addExpansionPorts(&in.Cluster.MeshExpansion.ILBGateway.Service.Ports, expansionPorts)
						if ilbGatewayValues, err := gatewayConfigToValues(in.Cluster.MeshExpansion.ILBGateway); err == nil {
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
			}
		}
		// Configure DNS search suffixes for "global"
		if in.Proxy != nil {
			foundGlobal := false
			foundDeploymentMetadata := false
			for _, ss := range in.Proxy.Networking.DNS.SearchSuffixes {
				if ss == searchSuffixGlobal {
					foundGlobal = true
				} else if strings.Index(ss, ".DeploymentMeta.Namespace") > 0 { // greater than works here because the template must be bracketed with {{ }}
					foundDeploymentMetadata = true
				}
			}
			if !foundGlobal {
				in.Proxy.Networking.DNS.SearchSuffixes = append(in.Proxy.Networking.DNS.SearchSuffixes, searchSuffixGlobal)
			}
			if !foundDeploymentMetadata {
				in.Proxy.Networking.DNS.SearchSuffixes = append(in.Proxy.Networking.DNS.SearchSuffixes, fmt.Sprintf(searchSuffixNamespaceGlobalTemplate, namespace))
			}
		}
	}

	return nil
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
	if enabled, ok, err := in.GetBool("global.multiCluster.enabled"); ok && enabled {
		clusterConfig.MultiCluster = &v2.MultiClusterConfig{}
		setClusterConfig = true
		if rawMeshNetworks, ok, err := in.GetMap("global.meshNetworks"); ok && len(rawMeshNetworks) > 0 {
			clusterConfig.MultiCluster.MeshNetworks = make(map[string]v2.MeshNetworkConfig)
			if err := fromValues(rawMeshNetworks, &clusterConfig.MultiCluster.MeshNetworks); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if ingressEnabled, ok, err := in.GetBool("global.k8sIngress.enabled"); ok {
			clusterConfig.MultiCluster.Ingress = ingressEnabled
		} else if err != nil {
			return err
		}
		if expansionEnabled, ok, err := in.GetBool("global.k8sIngress.enabled"); ok && expansionEnabled {
			clusterConfig.MeshExpansion = &v2.MeshExpansionConfig{}
			if useILBGateway, ok, err := in.GetBool("global.meshExpansion.useILB"); ok && useILBGateway {
				if ilbGatewayValues, ok, err := in.GetMap("gateways.istio-ilbgateway"); ok && len(ilbGatewayValues) > 0 {
					ilbGateway := &v2.GatewayConfig{}
					if err := gatewayValuesToConfig(v1.NewHelmValues(ilbGatewayValues), ilbGateway); err != nil {
						return err
					}
					clusterConfig.MeshExpansion.ILBGateway = ilbGateway
				} else if err != nil {
					return err
				}
			} else if err != nil {
				return nil
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
