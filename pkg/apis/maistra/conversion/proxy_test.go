package conversion

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	proxyConcurrency4 = int32(4)
)

var proxyTestCases = []conversionTestCase{
	{
		name: "defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy:   &v2.ProxyConfig{},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "logging." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Logging: v2.ProxyLoggingConfig{
					Level: v2.LogLevelCritical,
					ComponentLevels: v2.ComponentLogLevels{
						v2.EnvoyComponentAdmin:  v2.LogLevelDebug,
						v2.EnvoyComponentClient: v2.LogLevelTrace,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"componentLogLevel": "admin:debug,client:trace",
					"logLevel":          "critical",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				AdminPort:   12345,
				Concurrency: &proxyConcurrency4,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"concurrency": 4,
					"adminPort":   12345,
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					ClusterDomain:     "example.com",
					ConnectionTimeout: "30s",
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"clusterDomain":     "example.com",
					"connectionTimeout": "30s",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.dns.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					DNS: v2.ProxyDNSConfig{
						SearchSuffixes: []string{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.dns." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					DNS: v2.ProxyDNSConfig{
						SearchSuffixes: []string{
							"example.com",
						},
						RefreshRate: "120s",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{
					"example.com",
				},
				"proxy": map[string]interface{}{
					"dnsRefreshRate": "120s",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.init.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					Initialization: v2.ProxyNetworkInitConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.init.cni." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					Initialization: v2.ProxyNetworkInitConfig{
						Type: v2.ProxyNetworkInitTypeCNI,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.init.init.default." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					Initialization: v2.ProxyNetworkInitConfig{
						Type: v2.ProxyNetworkInitTypeInitContainer,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"istio_cni": map[string]interface{}{
				"enabled": false,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.init.init.runtime." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					Initialization: v2.ProxyNetworkInitConfig{
						Type: v2.ProxyNetworkInitTypeInitContainer,
						InitContainer: &v2.ProxyInitContainerConfig{
							Runtime: &v2.ContainerConfig{
								CommonContainerConfig: v2.CommonContainerConfig{
									ImageRegistry:   "custom-registry",
									ImageTag:        "test",
									ImagePullPolicy: "Always",
									ImagePullSecrets: []corev1.LocalObjectReference{
										{
											Name: "pull-secret",
										},
									},
									Resources: &corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("100m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("10m"),
											corev1.ResourceMemory: resource.MustParse("64Mi"),
										},
									},
								},
								Image: "custom-init",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy_init": map[string]interface{}{
					"hub":             "custom-registry",
					"image":           "custom-init",
					"tag":             "test",
					"imagePullPolicy": "Always",
					"imagePullSecrets": []interface{}{
						"pull-secret",
					},
					"resources": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    "100m",
							"memory": "128Mi",
						},
						"requests": map[string]interface{}{
							"cpu":    "10m",
							"memory": "64Mi",
						},
					},
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": false,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.protocol.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					Protocol: v2.ProxyNetworkProtocolConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.protocol.misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					Protocol: v2.ProxyNetworkProtocolConfig{
						DetectionTimeout: "500ms",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"protocolDetectionTimeout": "500ms",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.protocol.debug.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					Protocol: v2.ProxyNetworkProtocolConfig{
						Debug: &v2.ProxyNetworkProtocolDebugConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"enableProtocolSniffingForInbound":  false,
				"enableProtocolSniffingForOutbound": false,
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.protocol.debug.set." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					Protocol: v2.ProxyNetworkProtocolConfig{
						Debug: &v2.ProxyNetworkProtocolDebugConfig{
							EnableInboundSniffing:  true,
							EnableOutboundSniffing: false,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"enableProtocolSniffingForInbound":  true,
				"enableProtocolSniffingForOutbound": false,
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.traffic.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					TrafficControl: v2.ProxyTrafficControlConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.traffic.inbound.redirect." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					TrafficControl: v2.ProxyTrafficControlConfig{
						Inbound: v2.ProxyInboundTrafficControlConfig{
							InterceptionMode: v2.ProxyNetworkInterceptionModeRedirect,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"interceptionMode": "REDIRECT",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.traffic.inbound.tproxy." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					TrafficControl: v2.ProxyTrafficControlConfig{
						Inbound: v2.ProxyInboundTrafficControlConfig{
							InterceptionMode: v2.ProxyNetworkInterceptionModeTProxy,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"interceptionMode": "TPROXY",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.traffic.inbound.ports." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					TrafficControl: v2.ProxyTrafficControlConfig{
						Inbound: v2.ProxyInboundTrafficControlConfig{
							ExcludedPorts: []int32{
								12345,
								12456,
							},
							IncludedPorts: []string{
								"*",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"excludeInboundPorts": "12345,12456",
					"includeInboundPorts": "*",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.traffic.inbound.ports.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					TrafficControl: v2.ProxyTrafficControlConfig{
						Inbound: v2.ProxyInboundTrafficControlConfig{
							ExcludedPorts: []int32{},
							IncludedPorts: []string{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"excludeInboundPorts": "",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.traffic.outbound.policy.any." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					TrafficControl: v2.ProxyTrafficControlConfig{
						Outbound: v2.ProxyOutboundTrafficControlConfig{
							Policy: v2.ProxyOutboundTrafficPolicyAllowAny,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"outboundTrafficPolicy": map[string]interface{}{
					"mode": "ALLOW_ANY",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.traffic.outbound.policy.registry." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					TrafficControl: v2.ProxyTrafficControlConfig{
						Outbound: v2.ProxyOutboundTrafficControlConfig{
							Policy: v2.ProxyOutboundTrafficPolicyRegistryOnly,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"outboundTrafficPolicy": map[string]interface{}{
					"mode": "REGISTRY_ONLY",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.traffic.outbound.ports." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					TrafficControl: v2.ProxyTrafficControlConfig{
						Outbound: v2.ProxyOutboundTrafficControlConfig{
							ExcludedIPRanges: []string{
								"10.30.1.0/24",
								"192.168.1.0/24",
							},
							ExcludedPorts: []int32{
								12345,
								23456,
							},
							IncludedIPRanges: []string{
								"*",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"excludeIPRanges":      "10.30.1.0/24,192.168.1.0/24",
					"excludeOutboundPorts": "12345,23456",
					"includeIPRanges":      "*",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "networking.traffic.outbound.ports.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: v2.ProxyNetworkingConfig{
					TrafficControl: v2.ProxyTrafficControlConfig{
						Outbound: v2.ProxyOutboundTrafficControlConfig{
							ExcludedIPRanges: []string{},
							ExcludedPorts:    []int32{},
							IncludedIPRanges: []string{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"excludeIPRanges":      "",
					"excludeOutboundPorts": "",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "runtime.readiness." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Runtime: v2.ProxyRuntimeConfig{
					Readiness: v2.ProxyReadinessConfig{
						RewriteApplicationProbes: featureEnabled,
						FailureThreshold:         3,
						InitialDelaySeconds:      10,
						PeriodSeconds:            60,
						StatusPort:               15020,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"readinessFailureThreshold":    3,
					"readinessInitialDelaySeconds": 10,
					"readinessPeriodSeconds":       60,
					"statusPort":                   15020,
				},
			},
			"sidecarInjectorWebhook": map[string]interface{}{
				"rewriteAppHTTPProbe": true,
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
	{
		name: "runtime.container." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Runtime: v2.ProxyRuntimeConfig{
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							ImageRegistry:   "custom-registry",
							ImageTag:        "test",
							ImagePullPolicy: "Always",
							ImagePullSecrets: []corev1.LocalObjectReference{
								{
									Name: "pull-secret",
								},
							},
							Resources: &corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
						},
						Image: "custom-proxy",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"hub":             "custom-registry",
					"image":           "custom-proxy",
					"tag":             "test",
					"imagePullPolicy": "Always",
					"imagePullSecrets": []interface{}{
						"pull-secret",
					},
					"resources": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    "100m",
							"memory": "128Mi",
						},
						"requests": map[string]interface{}{
							"cpu":    "10m",
							"memory": "64Mi",
						},
					},
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP": true,
				"multiCluster": map[string]interface{}{
					"enabled": false,
				},
				"meshExpansion": map[string]interface{}{
					"enabled": false,
					"useILB":  false,
				},
			},
		}),
	},
}

func TestProxyConversionFromV2(t *testing.T) {
	for _, tc := range proxyTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateProxyValues(specCopy, helmValues.GetContent()); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			if !reflect.DeepEqual(tc.isolatedIstio.DeepCopy(), helmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.isolatedIstio.GetContent(), helmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), helmValues.GetContent())
			if err := populateProxyConfig(helmValues.DeepCopy(), specv2); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			if !reflect.DeepEqual(tc.spec.Proxy, specv2.Proxy) {
				expected, _ := yaml.Marshal(tc.spec.Proxy)
				got, _ := yaml.Marshal(specv2.Proxy)
				t.Errorf("unexpected output converting values back to v2:\n\texpected:\n%s\n\tgot:\n%s", string(expected), string(got))
			}
		})
	}
}
