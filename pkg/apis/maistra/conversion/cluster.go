package conversion

import (
	"fmt"
	"regexp"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

const (
	searchSuffixGlobal                  = "global"
	searchSuffixNamespaceGlobalTemplate = "{{ valueOrDefault .DeploymentMeta.Namespace \"%s\" }}.global"

	clusterDomainDefault = "cluster.local"
)

var externalRequestedNetworkRegex = regexp.MustCompile("(^|,)external(,|$)")

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
	if in.Proxy != nil && in.Proxy.Networking != nil && in.Proxy.Networking.ClusterDomain != "" {
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
	gatewaysOverrides := v1.NewHelmValues(make(map[string]interface{}))
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
				if err := setHelmStringValue(values, "global.multiCluster.addedLocalNetwork", cluster.Network); err != nil {
					return err
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
			in.Gateways = &v2.GatewaysConfig{}
		}
		if in.Gateways.ClusterEgress == nil {
			gatewaysOverrides.SetField("egressEnabled", nil)
			enabled := true
			in.Gateways.ClusterEgress = &v2.EgressGatewayConfig{
				GatewayConfig: v2.GatewayConfig{
					Enablement: v2.Enablement{
						Enabled: &enabled,
					},
				},
			}
		} else if in.Gateways.ClusterEgress.Enabled == nil {
			gatewaysOverrides.SetField("egressEnabled", nil)
			enabled := true
			in.Gateways.ClusterEgress.Enabled = &enabled
		} else if !*in.Gateways.ClusterEgress.Enabled {
			gatewaysOverrides.SetField("egressEnabled", *in.Gateways.ClusterEgress.Enabled)
			*in.Gateways.ClusterEgress.Enabled = true
		}
		if in.Gateways.Enabled == nil {
			gatewaysOverrides.SetField("enabled", nil)
			enabled := true
			in.Gateways.Enabled = &enabled
		} else if !*in.Gateways.Enabled {
			gatewaysOverrides.SetField("enabled", *in.Gateways.Enabled)
			*in.Gateways.Enabled = true
		}

		foundExternal := false
		for _, network := range in.Gateways.ClusterEgress.RequestedNetworkView {
			if network == "external" {
				foundExternal = true
				break
			}
		}
		if !foundExternal {
			gatewaysOverrides.SetField("addedExternal", true)
			in.Gateways.ClusterEgress.RequestedNetworkView = append(in.Gateways.ClusterEgress.RequestedNetworkView, "external")
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
					gatewaysOverrides.SetField("ingressEnabled", nil)
					gatewaysOverrides.SetField("k8sIngressEnabled", nil)
					enabled := true
					in.Gateways.ClusterIngress = &v2.ClusterIngressGatewayConfig{
						IngressGatewayConfig: v2.IngressGatewayConfig{
							GatewayConfig: v2.GatewayConfig{
								Enablement: v2.Enablement{
									Enabled: &enabled,
								},
							},
						},
						IngressEnabled: &enabled,
					}
				} else {
					if in.Gateways.ClusterIngress.Enabled == nil || !*in.Gateways.ClusterIngress.Enabled {
						enabled := true
						in.Gateways.ClusterIngress.Enabled = &enabled
						gatewaysOverrides.SetField("ingressEnabled", *in.Gateways.ClusterIngress.Enabled)
					}
					if in.Gateways.ClusterIngress.IngressEnabled == nil || !*in.Gateways.ClusterIngress.IngressEnabled {
						k8sIngressEnabled := true
						in.Gateways.ClusterIngress.IngressEnabled = &k8sIngressEnabled
						gatewaysOverrides.SetField("k8sIngressEnabled", *in.Gateways.ClusterIngress.Enabled)
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
		addedSearchSuffixes := make([]interface{}, 0, 2)
		dns := &v2.ProxyDNSConfig{}
		if in.Proxy == nil {
			in.Proxy = &v2.ProxyConfig{}
		} else if in.Proxy.Networking != nil && in.Proxy.Networking.DNS != nil {
			dns = in.Proxy.Networking.DNS
			for index, ss := range dns.SearchSuffixes {
				if ss == searchSuffixGlobal {
					globalIndex = index
				} else if strings.Index(ss, ".DeploymentMeta.Namespace") > 0 { // greater than works here because the template must be bracketed with {{ }}
					deploymentMetadataIndex = index
				}
			}
		}
		if deploymentMetadataIndex < 0 {
			namespaceSuffix := fmt.Sprintf(searchSuffixNamespaceGlobalTemplate, namespace)
			addedSearchSuffixes = append(addedSearchSuffixes, namespaceSuffix)
			if globalIndex < 0 {
				dns.SearchSuffixes = append([]string{namespaceSuffix}, dns.SearchSuffixes...)
			} else {
				dns.SearchSuffixes = append(append(dns.SearchSuffixes[:globalIndex+1], namespaceSuffix), dns.SearchSuffixes[globalIndex+1:]...)
			}
		}
		if globalIndex < 0 {
			addedSearchSuffixes = append(addedSearchSuffixes, searchSuffixGlobal)
			dns.SearchSuffixes = append([]string{searchSuffixGlobal}, dns.SearchSuffixes...)
		}
		if len(addedSearchSuffixes) > 0 {
			if in.Proxy.Networking == nil {
				in.Proxy.Networking = &v2.ProxyNetworkingConfig{}
			}
			if in.Proxy.Networking.DNS == nil {
				in.Proxy.Networking.DNS = dns
			}
			if err := setHelmValue(values, "global.multiCluster.addedSearchSuffixes", addedSearchSuffixes); err != nil {
				return err
			}
		}
	}

	if len(gatewaysOverrides.GetContent()) > 0 {
		if err := setHelmValue(values, "global.multiCluster.gatewaysOverrides", gatewaysOverrides.GetContent()); err != nil {
			return err
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

	// patchup gateways
	if rawGatewaysOverrides, ok, err := in.GetMap("global.multiCluster.gatewaysOverrides"); ok && len(rawGatewaysOverrides) > 0 {
		gatewaysOverrides := v1.NewHelmValues(rawGatewaysOverrides)
		if gateways, ok, err := in.GetMap("gateways"); ok && len(gateways) > 0 {
			updateGateways := false
			if enabled, ok, err := gatewaysOverrides.GetFieldNoCopy("enabled"); ok {
				if enabled == nil {
					delete(gateways, "enabled")
				} else {
					gateways["enabled"] = enabled
				}
				updateGateways = true
			} else if err != nil {
				return err
			}
			if ingressEnabled, ok, err := gatewaysOverrides.GetFieldNoCopy("ingressEnabled"); ok {
				if ingressGateway, ok, err := v1.NewHelmValues(gateways).GetMap("istio-ingressgateway"); ok && len(ingressGateway) > 0 {
					if ingressEnabled == nil {
						delete(ingressGateway, "enabled")
					} else {
						ingressGateway["enabled"] = ingressEnabled
					}
					if len(ingressGateway) == 1 {
						// only element should be name
						delete(gateways, "istio-ingressgateway")
					} else {
						gateways["istio-ingressgateway"] = ingressGateway
					}
					updateGateways = true
				} else if err != nil {
					return nil
				}
			} else if err != nil {
				return nil
			}
			if egressGateway, ok, err := v1.NewHelmValues(gateways).GetMap("istio-egressgateway"); ok && len(egressGateway) > 0 {
				updateEgress := false
				if egressEnabled, ok, err := gatewaysOverrides.GetFieldNoCopy("egressEnabled"); ok {
					if egressEnabled == nil {
						delete(egressGateway, "enabled")
					} else {
						egressGateway["enabled"] = egressEnabled
					}
					updateEgress = true
				} else if err != nil {
					return nil
				}
				if addedExternal, ok, err := gatewaysOverrides.GetBool("addedExternal"); ok && addedExternal {
					if requestedNetworkView, ok, err := v1.NewHelmValues(egressGateway).GetString("env.ISTIO_META_REQUESTED_NETWORK_VIEW"); ok {
						newRequestedNetworkView := externalRequestedNetworkRegex.ReplaceAllString(requestedNetworkView, "$1")
						if newRequestedNetworkView != requestedNetworkView {
							updateEgress = true
							if err := setHelmStringValue(egressGateway, "env.ISTIO_META_REQUESTED_NETWORK_VIEW", newRequestedNetworkView); err != nil {
								return err
							}
						}
					} else if err != nil {
						return err
					}
				} else if err != nil {
					return err
				}
				if updateEgress {
					if len(egressGateway) == 1 {
						// only element should be name
						delete(gateways, "istio-egressgateway")
					} else {
						gateways["istio-egressgateway"] = egressGateway
					}
					updateGateways = true
				}
			} else if err != nil {
				return nil
			}
			if updateGateways {
				if len(gateways) == 0 {
					delete(in.GetContent(), "gateways")
				} else {
					in.SetField("gateways", gateways)
				}
			}
			if k8sIngressEnabled, ok, err := gatewaysOverrides.GetFieldNoCopy("k8sIngressEnabled"); ok {
				if k8sIngressValues, ok, err := in.GetMap("global.k8sIngress"); ok && len(k8sIngressValues) > 0 {
					if k8sIngressEnabled == nil {
						delete(k8sIngressValues, "enabled")
					} else {
						k8sIngressValues["enabled"] = k8sIngressEnabled
					}
					in.SetField("global.k8sIngress", k8sIngressValues)
				} else if err != nil {
					return nil
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
			if addedLocalNetwork, ok, err := in.GetString("global.multiCluster.addedLocalNetwork"); ok {
				delete(clusterConfig.MultiCluster.MeshNetworks, addedLocalNetwork)
			} else if err != nil {
				return nil
			}
		} else if err != nil {
			return nil
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
