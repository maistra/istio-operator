package conversion

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

const clusterTestNamespace = "cp-namespace"

var clusterTestCases = []conversionTestCase{
	{
		name:      "nil." + versions.V1_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_0.String(),
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"enabled": false,
					"multiClusterOverrides": map[string]interface{}{
						"expansionEnabled":    nil,
						"multiClusterEnabled": nil,
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{}),
	},
	{
		name:      "nil." + versions.V1_1.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"enabled": false,
					"multiClusterOverrides": map[string]interface{}{
						"expansionEnabled":    nil,
						"multiClusterEnabled": nil,
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{}),
	},
	{
		name:      "nil." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"enabled": false,
					"multiClusterOverrides": map[string]interface{}{
						"expansionEnabled":    nil,
						"multiClusterEnabled": nil,
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{}),
	},
	{
		name:      "simple." + versions.V1_1.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName": "my-cluster",
					"enabled":     false,
					"multiClusterOverrides": map[string]interface{}{
						"expansionEnabled":    nil,
						"multiClusterEnabled": nil,
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
				"network": "my-network",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{}),
	},
	{
		name:      "simple." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName": "my-cluster",
					"enabled":     false,
					"multiClusterOverrides": map[string]interface{}{
						"expansionEnabled":    nil,
						"multiClusterEnabled": nil,
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
				"network": "my-network",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{}),
	},
	{
		name:      "multicluster.simple." + versions.V1_1.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     nil,
						"expansionEnabled":  nil,
						"gatewaysEnabled":   nil,
						"ilbEnabled":        nil,
						"ingressEnabled":    nil,
						"k8sIngressEnabled": nil,
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true,
					"env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.simple." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     nil,
						"expansionEnabled":  nil,
						"gatewaysEnabled":   nil,
						"ilbEnabled":        nil,
						"ingressEnabled":    nil,
						"k8sIngressEnabled": nil,
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.ilb." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
				MeshExpansion: &v2.MeshExpansionConfig{
					ILBGateway: &v2.GatewayConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":    true,
						"egressEnabled":    nil,
						"expansionEnabled": nil,
						"gatewaysEnabled":  nil,
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  true,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": true,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
			},
		}),
	},
	{
		name:      "multicluster.meshNetwork.override." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
					MeshNetworks: map[string]v2.MeshNetworkConfig{
						"my-network": {
							Endpoints: []v2.MeshEndpointConfig{
								{
									FromRegistry: "my-cluster",
								},
							},
							Gateways: []v2.MeshGatewayConfig{
								{
									Service: "istio-ingressgateway.my-ns.svc.cluster.local",
									Port:    9443,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName": "my-cluster",
					"enabled":     true,
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     interface{}(nil),
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    9443,
								"service": "istio-ingressgateway.my-ns.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.meshNetwork.additional." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
					MeshNetworks: map[string]v2.MeshNetworkConfig{
						"other-network": {
							Endpoints: []v2.MeshEndpointConfig{
								{
									FromRegistry: "other-cluster",
								},
							},
							Gateways: []v2.MeshGatewayConfig{
								{
									Service: "istio-ingressgateway.other-ns.svc.cluster.local",
									Port:    443,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     interface{}(nil),
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
					"other-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "other-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.other-ns.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.clusterDomain.override." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					ClusterDomain: "example.com",
				},
			},
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     interface{}(nil),
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.example.com",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"proxy": map[string]interface{}{
					"clusterDomain": "example.com",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.searchSuffix.global." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					DNS: &v2.ProxyDNSConfig{
						SearchSuffixes: []string{
							"global",
						},
					},
				},
			},
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     interface{}(nil),
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.searchSuffix.namespace." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					DNS: &v2.ProxyDNSConfig{
						SearchSuffixes: []string{
							"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						},
					},
				},
			},
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     interface{}(nil),
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.searchSuffix.all." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					DNS: &v2.ProxyDNSConfig{
						SearchSuffixes: []string{
							"global",
							"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						},
					},
				},
			},
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     interface{}(nil),
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.searchSuffix.custom." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					DNS: &v2.ProxyDNSConfig{
						SearchSuffixes: []string{
							"custom",
						},
					},
				},
			},
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     interface{}(nil),
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
					"custom",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.searchSuffix.custom.insert." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					DNS: &v2.ProxyDNSConfig{
						SearchSuffixes: []string{
							"custom",
							"global",
						},
					},
				},
			},
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     interface{}(nil),
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"custom",
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.gateways.egress.unconfigured" + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
			Gateways: &v2.GatewaysConfig{},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     interface{}(nil),
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.gateways.egress.enabled." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
			Gateways: &v2.GatewaysConfig{
				ClusterEgress: &v2.EgressGatewayConfig{
					GatewayConfig: v2.GatewayConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.gateways.egress.configured." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
			Gateways: &v2.GatewaysConfig{
				ClusterEgress: &v2.EgressGatewayConfig{
					GatewayConfig: v2.GatewayConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
					},
					RequestedNetworkView: []string{
						"external",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.ingress.http." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":     true,
						"egressEnabled":     interface{}(nil),
						"expansionEnabled":  nil,
						"gatewaysEnabled":   interface{}(nil),
						"ilbEnabled":        nil,
						"ingressEnabled":    interface{}(nil),
						"k8sIngressEnabled": interface{}(nil),
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": false,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
				},
			},
		}),
	},
	{
		name:      "multicluster.ingress.https." + versions.V2_0.String(),
		namespace: clusterTestNamespace,
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Cluster: &v2.ControlPlaneClusterConfig{
				Name:    "my-cluster",
				Network: "my-network",
				MultiCluster: &v2.MultiClusterConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
			Gateways: &v2.GatewaysConfig{
				ClusterIngress: &v2.ClusterIngressGatewayConfig{
					IngressGatewayConfig: v2.IngressGatewayConfig{
						GatewayConfig: v2.GatewayConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
							Service: v2.GatewayServiceConfig{
								ServiceSpec: corev1.ServiceSpec{
									Ports: []corev1.ServicePort{
										{
											Name:       "https",
											Port:       443,
											TargetPort: intstr.FromInt(8443),
										},
									},
								},
							},
						},
					},
					IngressEnabled: &featureEnabled,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster": map[string]interface{}{
					"clusterName":       "my-cluster",
					"enabled":           true,
					"addedLocalNetwork": "my-network",
					"addedSearchSuffixes": []interface{}{
						"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
						"global",
					},
					"multiClusterOverrides": map[string]interface{}{
						"addedExternal":    true,
						"egressEnabled":    interface{}(nil),
						"expansionEnabled": nil,
						"gatewaysEnabled":  interface{}(nil),
						"ilbEnabled":       nil,
					},
				},
				"meshExpansion": map[string]interface{}{
					"enabled": true,
					"useILB":  false,
				},
				"meshNetworks": map[string]interface{}{
					"my-network": map[string]interface{}{
						"endpoints": []interface{}{
							map[string]interface{}{
								"fromRegistry": "my-cluster",
							},
						}, "gateways": []interface{}{
							map[string]interface{}{
								"port":    443,
								"service": "istio-ingressgateway.cp-namespace.svc.cluster.local",
							},
						},
					},
				},
				"network": "my-network",
			},
			"gateways": map[string]interface{}{
				"istio-ilbgateway": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"global",
					"{{ valueOrDefault .DeploymentMeta.Namespace \"cp-namespace\" }}.global",
				},
				"k8sIngress": map[string]interface{}{
					"enabled":     true,
					"enableHttps": true,
					"gatewayName": "ingressgateway",
				},
			},
			"gateways": map[string]interface{}{
				"enabled": true,
				"istio-egressgateway": map[string]interface{}{
					"enabled": true, "env": map[string]interface{}{
						"ISTIO_META_REQUESTED_NETWORK_VIEW": "external",
					},
					"name":        "istio-egressgateway",
					"gatewayType": "egress",
				},
				"istio-ingressgateway": map[string]interface{}{
					"enabled":     true,
					"name":        "istio-ingressgateway",
					"gatewayType": "ingress",
					"ports": []interface{}{
						map[string]interface{}{
							"name":       "https",
							"port":       443,
							"targetPort": 8443,
						},
					},
				},
			},
		}),
	},
}

func TestClusterConversionFromV2(t *testing.T) {
	for _, tc := range clusterTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateClusterValues(specCopy, tc.namespace, helmValues.GetContent()); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			if !reflect.DeepEqual(tc.isolatedIstio.DeepCopy(), helmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.isolatedIstio.GetContent(), helmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), helmValues.GetContent())
			if err := populateClusterConfig(helmValues.DeepCopy(), specv2); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			assertEquals(t, tc.spec.Cluster, specv2.Cluster)
		})
	}
}
