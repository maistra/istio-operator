package conversion

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	proxyConcurrency4 = int32(4)
)

var proxyTestCases = []conversionTestCase{
	{
		name: "nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy:   &v2.ProxyConfig{},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "logging." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Logging: &v2.ProxyLoggingConfig{
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					ClusterDomain:     "example.com",
					ConnectionTimeout: "30s",
					MaxConnectionAge:  "30m",
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
			"pilot": map[string]interface{}{
				"keepaliveMaxServerConnectionAge": "30m",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.dns.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					DNS: &v2.ProxyDNSConfig{
						SearchSuffixes: []string{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"podDNSSearchNamespaces": []interface{}{},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.dns." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					DNS: &v2.ProxyDNSConfig{
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.init.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					Initialization: &v2.ProxyNetworkInitConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.init.cni." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					Initialization: &v2.ProxyNetworkInitConfig{
						Type: v2.ProxyNetworkInitTypeCNI,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"initType": "CNI",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.init.init.default." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					Initialization: &v2.ProxyNetworkInitConfig{
						Type: v2.ProxyNetworkInitTypeInitContainer,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"initType": "InitContainer",
				},
			},
			"istio_cni": map[string]interface{}{
				"enabled": false,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.init.init.runtime." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					Initialization: &v2.ProxyNetworkInitConfig{
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
				"proxy": map[string]interface{}{
					"initType": "InitContainer",
				},
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
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.protocol.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					Protocol: &v2.ProxyNetworkProtocolConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.protocol.auto.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					Protocol: &v2.ProxyNetworkProtocolConfig{
						AutoDetect: &v2.ProxyNetworkAutoProtocolDetectionConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.protocol.auto.all." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					Protocol: &v2.ProxyNetworkProtocolConfig{
						AutoDetect: &v2.ProxyNetworkAutoProtocolDetectionConfig{
							Timeout:  "500ms",
							Inbound:  &featureEnabled,
							Outbound: &featureDisabled,
						},
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
			"pilot": map[string]interface{}{
				"enableProtocolSniffingForInbound":  true,
				"enableProtocolSniffingForOutbound": false,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.traffic.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					TrafficControl: &v2.ProxyTrafficControlConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.traffic.inbound.redirect." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					TrafficControl: &v2.ProxyTrafficControlConfig{
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.traffic.inbound.tproxy." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					TrafficControl: &v2.ProxyTrafficControlConfig{
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.traffic.inbound.ports." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					TrafficControl: &v2.ProxyTrafficControlConfig{
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.traffic.inbound.ports.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					TrafficControl: &v2.ProxyTrafficControlConfig{
						Inbound: v2.ProxyInboundTrafficControlConfig{
							ExcludedPorts: []int32{},
							IncludedPorts: []string{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.traffic.outbound.policy.any." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					TrafficControl: &v2.ProxyTrafficControlConfig{
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.traffic.outbound.policy.registry." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					TrafficControl: &v2.ProxyTrafficControlConfig{
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.traffic.outbound.ports." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					TrafficControl: &v2.ProxyTrafficControlConfig{
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "networking.traffic.outbound.ports.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Networking: &v2.ProxyNetworkingConfig{
					TrafficControl: &v2.ProxyTrafficControlConfig{
						Outbound: v2.ProxyOutboundTrafficControlConfig{
							ExcludedIPRanges: []string{},
							ExcludedPorts:    []int32{},
							IncludedIPRanges: []string{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "injection.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Injection: &v2.ProxyInjectionConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "injection.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Injection: &v2.ProxyInjectionConfig{
					AutoInject: &featureEnabled,
					AlwaysInjectSelector: []metav1.LabelSelector{
						{
							MatchLabels: map[string]string{
								"some-label": "some-value",
							},
						},
					},
					NeverInjectSelector: []metav1.LabelSelector{
						{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "some-label",
									Operator: metav1.LabelSelectorOpNotIn,
									Values: []string{
										"some-value",
									},
								},
							},
						},
					},
					InjectedAnnotations: map[string]string{
						"some-annotation": "some-annotation-value",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"autoInject": "enabled",
				},
			},
			"sidecarInjectorWebhook": map[string]interface{}{
				"enableNamespacesByDefault": true,
				"alwaysInjectSelector": []interface{}{
					map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"some-label": "some-value",
						},
					},
				},
				"neverInjectSelector": []interface{}{
					map[string]interface{}{
						"matchExpressions": []interface{}{
							map[string]interface{}{
								"key":      "some-label",
								"operator": "NotIn",
								"values": []interface{}{
									"some-value",
								},
							},
						},
					},
				},
				"injectedAnnotations": map[string]interface{}{
					"some-annotation": "some-annotation-value",
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "accessLog.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				AccessLogging: &v2.ProxyAccessLoggingConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "accessLog.file." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				AccessLogging: &v2.ProxyAccessLoggingConfig{
					File: &v2.ProxyFileAccessLogConfig{
						Name:     "/dev/stdout",
						Encoding: "JSON",
						Format:   "[%START_TIME%] %REQ(:METHOD)%",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"accessLogFile":     "/dev/stdout",
					"accessLogEncoding": "JSON",
					"accessLogFormat":   "[%START_TIME%] %REQ(:METHOD)%",
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "accessLog.service." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				AccessLogging: &v2.ProxyAccessLoggingConfig{
					EnvoyService: &v2.ProxyEnvoyServiceConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
						Address: "some.host.com:8080",
						TCPKeepalive: &v2.EnvoyServiceTCPKeepalive{
							Probes:   3,
							Interval: "10s",
							Time:     "20s",
						},
						TLSSettings: &v2.EnvoyServiceClientTLSSettings{
							Mode:              "DISABLED",
							ClientCertificate: "/etc/istio/als/cert-chain.pem",
							PrivateKey:        "/etc/istio/als/key.pem",
							CACertificates:    "/etc/istio/als/root-cert.pem",
							SNIHost:           "als.somedomain",
							SubjectAltNames: []string{
								"als.somedomain",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"envoyAccessLogService": map[string]interface{}{
						"enabled": true,
						"host":    "some.host.com",
						"port":    "8080",
						"tcpKeepalive": map[string]interface{}{
							"interval": "10s",
							"probes":   3,
							"time":     "20s",
						},
						"tlsSettings": map[string]interface{}{
							"caCertificates":    "/etc/istio/als/root-cert.pem",
							"clientCertificate": "/etc/istio/als/cert-chain.pem",
							"mode":              "DISABLED",
							"privateKey":        "/etc/istio/als/key.pem",
							"sni":               "als.somedomain",
							"subjectAltNames": []interface{}{
								"als.somedomain",
							},
						},
					},
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "metricsService." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				EnvoyMetricsService: &v2.ProxyEnvoyServiceConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
					Address: "some.host.com:8080",
					TCPKeepalive: &v2.EnvoyServiceTCPKeepalive{
						Probes:   3,
						Interval: "10s",
						Time:     "20s",
					},
					TLSSettings: &v2.EnvoyServiceClientTLSSettings{
						Mode:              "DISABLED",
						ClientCertificate: "/etc/istio/als/cert-chain.pem",
						PrivateKey:        "/etc/istio/als/key.pem",
						CACertificates:    "/etc/istio/als/root-cert.pem",
						SNIHost:           "als.somedomain",
						SubjectAltNames: []string{
							"als.somedomain",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"envoyMetricsService": map[string]interface{}{
						"enabled": true,
						"host":    "some.host.com",
						"port":    "8080",
						"tcpKeepalive": map[string]interface{}{
							"interval": "10s",
							"probes":   3,
							"time":     "20s",
						},
						"tlsSettings": map[string]interface{}{
							"caCertificates":    "/etc/istio/als/root-cert.pem",
							"clientCertificate": "/etc/istio/als/cert-chain.pem",
							"mode":              "DISABLED",
							"privateKey":        "/etc/istio/als/key.pem",
							"sni":               "als.somedomain",
							"subjectAltNames": []interface{}{
								"als.somedomain",
							},
						},
					},
				},
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "runtime.readiness." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Runtime: &v2.ProxyRuntimeConfig{
					Readiness: &v2.ProxyReadinessConfig{
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "runtime.container." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Proxy: &v2.ProxyConfig{
				Runtime: &v2.ProxyRuntimeConfig{
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
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
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
			assertEquals(t, tc.spec.Proxy, specv2.Proxy)
		})
	}
}
