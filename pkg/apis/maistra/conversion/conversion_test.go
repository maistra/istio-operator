package conversion

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

var (
	featureEnabled   = true
	featureDisabled  = false
	replicaCount1    = int32(1)
	replicaCount2    = int32(2)
	replicaCount5    = int32(5)
	cpuUtilization80 = int32(80)
	intStrInt1       = intstr.FromInt(1)
	intStr25Percent  = intstr.FromString("25%")

	globalMultiClusterDefaults = map[string]interface{}{
		"enabled": false,
		"multiClusterOverrides": map[string]interface{}{
			"expansionEnabled":    nil,
			"multiClusterEnabled": nil,
		},
	}
	globalMeshExpansionDefaults = map[string]interface{}{
		"enabled": false,
		"useILB":  false,
	}

	roundTripTestCases = []struct {
		name         string
		smcpv1       v1.ControlPlaneSpec
		smcpv2       v2.ControlPlaneSpec
		roundTripped *v1.ControlPlaneSpec
		cruft        *v1.HelmValues // these are just the key paths that need to be removed
		skip         bool
	}{
		{
			name: "simple",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"proxy": map[string]interface{}{
							"image": "asd",
						},
						"some-unmapped-field": map[string]interface{}{
							"foo":   "bar",
							"fooey": true,
						},
					},
					"prometheus": map[string]interface{}{
						"enabled": true,
					},
					"tracing": map[string]interface{}{
						"enabled": true,
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v1.1",
				Proxy: &v2.ProxyConfig{
					Runtime: &v2.ProxyRuntimeConfig{
						Container: &v2.ContainerConfig{
							Image: "asd",
						},
					},
				},
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
					},
				},
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"some-unmapped-field": map[string]interface{}{
							"foo":   "bar",
							"fooey": true,
						},
					},
				}),
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// a result of enabling tracing
					"enableTracing": nil,
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
					// a result of enabling tracing, default provider is jaeger
					"proxy": map[string]interface{}{
						"tracer": nil,
					},
				},
				"meshConfig": map[string]interface{}{
					// a result of enabling tracing
					"enableTracing": nil,
				},
				// a result of enabling prometheus
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": nil,
						},
					},
				},
				// a result of enabling prometheus
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": nil,
						},
					},
				},
				// a result of enabling tracing, default provider is jaeger
				"tracing": map[string]interface{}{
					"provider": nil,
				},
			}),
		},
		{
			name: "MAISTRA-1902",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"disablePolicyChecks": false,
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v1.1",
				Policy: &v2.PolicyConfig{
					Mixer: &v2.MixerPolicyConfig{
						EnableChecks: &featureEnabled,
					},
				},
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
				},
			}),
		},
		{
			name: "cruft-check",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"galley": map[string]interface{}{
						"resources": map[string]interface{}{
							"requests": map[string]interface{}{
								"cpu":    "10m",
								"memory": "128Mi",
							},
						},
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v1.1",
				Runtime: &v2.ControlPlaneRuntimeConfig{
					Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
						v2.ControlPlaneComponentNameGalley: {
							Container: &v2.ContainerConfig{
								CommonContainerConfig: v2.CommonContainerConfig{
									Resources: &corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("10m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
				},
			}),
		},
		{
			name: "float-int-tags",
			skip: true,
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"galley": map[string]interface{}{
						"tag": 1.4,
					},
					"pilot": map[string]interface{}{
						"tag": 1,
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v1.1",
				Runtime: &v2.ControlPlaneRuntimeConfig{
					Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
						v2.ControlPlaneComponentNameGalley: {
							Container: &v2.ContainerConfig{
								CommonContainerConfig: v2.CommonContainerConfig{
									ImageTag: "1.4",
								},
							},
						},
						v2.ControlPlaneComponentNamePilot: {
							Container: &v2.ContainerConfig{
								CommonContainerConfig: v2.CommonContainerConfig{
									ImageTag: "1",
								},
							},
						},
					},
				},
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
				},
			}),
		},
		{
			name: "float-int-tags",
			skip: true,
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"mtls": map[string]interface{}{
							"enabled": true,
						},
						"auto": true,
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v1.1",
				Security: &v2.SecurityConfig{
					DataPlane: &v2.DataPlaneSecurityConfig{
						MTLS:     &featureEnabled,
						AutoMTLS: &featureEnabled,
					},
				},
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
				},
			}),
		},
		{
			name: "MAISTRA-1983.1",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"tracing": map[string]interface{}{
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"nodeCount":        int64(5),
								"redundancyPolicy": nil,
								"esIndexCleaner": map[string]interface{}{
									"enabled":      true,
									"numberOfDays": int64(60),
									"schedule":     "55 23 * * *",
								},
							},
						},
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v1.1",
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Elasticsearch: &v2.JaegerElasticsearchStorageConfig{
									NodeCount: &jaegerElasticsearchNodeCount,
								},
							},
						},
					},
				},
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"tracing": map[string]interface{}{
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"esIndexCleaner": map[string]interface{}{
									"enabled":      true,
									"numberOfDays": int64(60),
									"schedule":     "55 23 * * *",
								},
							},
						},
					},
				}),
			},
			roundTripped: &v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"tracing": map[string]interface{}{
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"nodeCount": int64(5),
								"esIndexCleaner": map[string]interface{}{
									"enabled":      true,
									"numberOfDays": int64(60),
									"schedule":     "55 23 * * *",
								},
							},
						},
					},
				}),
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
				},
			}),
		},
		{
			name: "MAISTRA-1983.2",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"tracing": map[string]interface{}{
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"nodeCount":        int64(5),
								"redundancyPolicy": nil,
							},
						},
						"esIndexCleaner": map[string]interface{}{
							"enabled":      true,
							"numberOfDays": int64(60),
							"schedule":     "55 23 * * *",
						},
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v1.1",
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Elasticsearch: &v2.JaegerElasticsearchStorageConfig{
									NodeCount: &jaegerElasticsearchNodeCount,
								},
							},
						},
					},
				},
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"tracing": map[string]interface{}{
						"esIndexCleaner": map[string]interface{}{
							"enabled":      true,
							"numberOfDays": int64(60),
							"schedule":     "55 23 * * *",
						},
					},
				}),
			},
			roundTripped: &v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"tracing": map[string]interface{}{
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"nodeCount": int64(5),
							},
						},
						"esIndexCleaner": map[string]interface{}{
							"enabled":      true,
							"numberOfDays": int64(60),
							"schedule":     "55 23 * * *",
						},
					},
				}),
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
				},
			}),
		},
		{
			name: "MAISTRA-1983.3",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"tracing": map[string]interface{}{
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"nodeCount":        int64(5),
								"redundancyPolicy": nil,
							},
							"esIndexCleaner": map[string]interface{}{
								"enabled":      true,
								"numberOfDays": int64(60),
								"schedule":     "55 23 * * *",
							},
						},
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v1.1",
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Elasticsearch: &v2.JaegerElasticsearchStorageConfig{
									NodeCount: &jaegerElasticsearchNodeCount,
									IndexCleaner: v1.NewHelmValues(map[string]interface{}{
										"enabled":      true,
										"numberOfDays": int64(60),
										"schedule":     "55 23 * * *",
									}),
								},
							},
						},
					},
				},
			},
			roundTripped: &v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"tracing": map[string]interface{}{
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"nodeCount": int64(5),
							},
							"esIndexCleaner": map[string]interface{}{
								"enabled":      true,
								"numberOfDays": float64(60),
								"schedule":     "55 23 * * *",
							},
						},
					},
				}),
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
				},
			}),
		},
		{
			name: "MAISTRA-1983.user",
			smcpv1: v1.ControlPlaneSpec{
				Version:  "v1.1",
				Template: "default",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"gateways": map[string]interface{}{
						"istio-ingressgateway": map[string]interface{}{
							"autoscaleEnabled":      false,
							"externalTrafficPolicy": "Local",
							"ior_enabled":           false,
							"ports": []interface{}{
								map[string]interface{}{
									"name":       "status-port",
									"nodePort":   30158,
									"port":       15020,
									"protocol":   "TCP",
									"targetPort": 15020,
								},
								map[string]interface{}{
									"name":       "http2",
									"nodePort":   32520,
									"port":       80,
									"protocol":   "TCP",
									"targetPort": 8082,
								},
								map[string]interface{}{
									"name":       "https",
									"nodePort":   32731,
									"port":       443,
									"protocol":   "TCP",
									"targetPort": 8443,
								},
								map[string]interface{}{
									"name":       "tls",
									"nodePort":   30462,
									"port":       15443,
									"protocol":   "TCP",
									"targetPort": 15443,
								},
							},
							"replicaCount": 5,
							"sds": map[string]interface{}{
								"enabled": true,
								"image":   "istio/node-agent-k8s:1.5.9",
							},
							"type": "NodePort",
						},
					},
					"global": map[string]interface{}{
						"controlPlaneSecurityEnabled": true,
						"disablePolicyChecks":         false,
						"logging": map[string]interface{}{
							"level": "default:info",
						},
						"mtls": map[string]interface{}{
							"enabled": true,
						},
						"outboundTrafficPolicy": map[string]interface{}{
							"mode": "REGISTRY_ONLY",
						},
						"policyCheckFailOpen": false,
						"proxy": map[string]interface{}{
							"accessLogFile": "/dev/stdout",
						},
						"tls": map[string]interface{}{
							"minProtocolVersion": "TLSv1_2",
						},
					},
					"grafana": map[string]interface{}{
						"enabled": true,
					},
					"kiali": map[string]interface{}{
						"enabled": true,
					},
					"mixer": map[string]interface{}{
						"policy": map[string]interface{}{
							"autoscaleEnabled": false,
						},
						"telemetry": map[string]interface{}{
							"autoscaleEnabled": false,
						},
					},
					"pilot": map[string]interface{}{
						"autoscaleEnabled": false,
						"traceSampling":    100,
					},
					"tracing": map[string]interface{}{
						"enabled": true,
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"esIndexCleaner": map[string]interface{}{
									"enabled":      true,
									"numberOfDays": 60,
									"schedule":     "55 23 * * *",
								},
								"nodeCount":        3,
								"redundancyPolicy": nil,
							},
							"template": "production-elasticsearch",
						},
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version:  "v1.1",
				Profiles: []string{"default"},
				General: &v2.GeneralConfig{
					Logging: &v2.LoggingConfig{
						ComponentLevels: v2.ComponentLogLevels{
							"default": "info",
						},
					},
				},
				Policy: &v2.PolicyConfig{
					Mixer: &v2.MixerPolicyConfig{
						EnableChecks: &featureEnabled,
						FailOpen:     &featureDisabled,
					},
				},
				Proxy: &v2.ProxyConfig{
					Networking: &v2.ProxyNetworkingConfig{
						TrafficControl: &v2.ProxyTrafficControlConfig{
							Outbound: v2.ProxyOutboundTrafficControlConfig{
								Policy: v2.ProxyOutboundTrafficPolicyRegistryOnly,
							},
						},
					},
					AccessLogging: &v2.ProxyAccessLoggingConfig{
						File: &v2.ProxyFileAccessLogConfig{
							Name: "/dev/stdout",
						},
					},
				},
				Security: &v2.SecurityConfig{
					ControlPlane: &v2.ControlPlaneSecurityConfig{
						MTLS: &featureEnabled,
						TLS: &v2.ControlPlaneTLSConfig{
							MinProtocolVersion: "TLSv1_2",
						},
					},
					DataPlane: &v2.DataPlaneSecurityConfig{
						MTLS: &featureEnabled,
					},
				},
				Tracing: &v2.TracingConfig{
					Type:     v2.TracerTypeJaeger,
					Sampling: &traceSamplingInt10000,
				},
				Gateways: &v2.GatewaysConfig{
					ClusterIngress: &v2.ClusterIngressGatewayConfig{
						IngressGatewayConfig: v2.IngressGatewayConfig{
							GatewayConfig: v2.GatewayConfig{
								Service: v2.GatewayServiceConfig{
									ServiceSpec: corev1.ServiceSpec{
										Type:                  "NodePort",
										ExternalTrafficPolicy: "Local",
										Ports: []corev1.ServicePort{
											{
												Name:       "status-port",
												Protocol:   "TCP",
												Port:       15020,
												TargetPort: intstr.FromInt(15020),
												NodePort:   30158,
											},
											{
												Name:       "http2",
												Protocol:   "TCP",
												Port:       80,
												TargetPort: intstr.FromInt(8082),
												NodePort:   32520,
											},
											{
												Name:       "https",
												Protocol:   "TCP",
												Port:       443,
												TargetPort: intstr.FromInt(8443),
												NodePort:   32731,
											},
											{
												Name:       "tls",
												Protocol:   "TCP",
												Port:       15443,
												TargetPort: intstr.FromInt(15443),
												NodePort:   30462,
											},
										},
									},
								},
								Runtime: &v2.ComponentRuntimeConfig{
									Deployment: &v2.DeploymentRuntimeConfig{
										Replicas: &replicaCount5,
										AutoScaling: &v2.AutoScalerConfig{
											Enablement: v2.Enablement{Enabled: &featureDisabled},
										},
									},
								},
							},
							SDS: &v2.SecretDiscoveryService{
								Enablement: v2.Enablement{Enabled: &featureEnabled},
								Runtime: &v2.ContainerConfig{
									Image: "istio/node-agent-k8s:1.5.9",
								},
							},
						},
					},
					OpenShiftRoute: &v2.OpenShiftRouteConfig{
						Enablement: v2.Enablement{Enabled: &featureDisabled},
					},
				},
				Runtime: &v2.ControlPlaneRuntimeConfig{
					Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
						v2.ControlPlaneComponentNameMixerPolicy: {
							Deployment: &v2.DeploymentRuntimeConfig{
								AutoScaling: &v2.AutoScalerConfig{
									Enablement: v2.Enablement{Enabled: &featureDisabled},
								},
							},
						},
						v2.ControlPlaneComponentNameMixerTelemetry: {
							Deployment: &v2.DeploymentRuntimeConfig{
								AutoScaling: &v2.AutoScalerConfig{
									Enablement: v2.Enablement{Enabled: &featureDisabled},
								},
							},
						},
						v2.ControlPlaneComponentNamePilot: {
							Deployment: &v2.DeploymentRuntimeConfig{
								AutoScaling: &v2.AutoScalerConfig{
									Enablement: v2.Enablement{Enabled: &featureDisabled},
								},
							},
						},
					},
				},
				Addons: &v2.AddonsConfig{
					Grafana: &v2.GrafanaAddonConfig{
						Enablement: v2.Enablement{Enabled: &featureEnabled},
					},
					Jaeger: &v2.JaegerAddonConfig{
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Type: v2.JaegerStorageTypeElasticsearch,
								Elasticsearch: &v2.JaegerElasticsearchStorageConfig{
									NodeCount: &jaegerElasticsearchNodeCount3,
								},
							},
						},
					},
					Kiali: &v2.KialiAddonConfig{
						Enablement: v2.Enablement{Enabled: &featureEnabled},
					},
				},
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"tracing": map[string]interface{}{
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"esIndexCleaner": map[string]interface{}{
									"enabled":      true,
									"numberOfDays": int64(60),
									"schedule":     "55 23 * * *",
								},
							},
						},
					},
				}),
			},
			roundTripped: &v1.ControlPlaneSpec{
				Version:  "v1.1",
				Template: "default",
				Profiles: []string{"default"},
				Istio: v1.NewHelmValues(map[string]interface{}{
					"gateways": map[string]interface{}{
						"istio-ingressgateway": map[string]interface{}{
							"autoscaleEnabled":      false,
							"externalTrafficPolicy": "Local",
							"ior_enabled":           false,
							"ports": []interface{}{
								map[string]interface{}{
									"name":       "status-port",
									"nodePort":   float64(30158),
									"port":       float64(15020),
									"protocol":   "TCP",
									"targetPort": float64(15020),
								},
								map[string]interface{}{
									"name":       "http2",
									"nodePort":   float64(32520),
									"port":       float64(80),
									"protocol":   "TCP",
									"targetPort": float64(8082),
								},
								map[string]interface{}{
									"name":       "https",
									"nodePort":   float64(32731),
									"port":       float64(443),
									"protocol":   "TCP",
									"targetPort": float64(8443),
								},
								map[string]interface{}{
									"name":       "tls",
									"nodePort":   float64(30462),
									"port":       float64(15443),
									"protocol":   "TCP",
									"targetPort": float64(15443),
								},
							},
							"replicaCount": int64(5),
							"sds": map[string]interface{}{
								"enabled": true,
								"image":   "istio/node-agent-k8s:1.5.9",
							},
							"type": "NodePort",
						},
					},
					"global": map[string]interface{}{
						"controlPlaneSecurityEnabled": true,
						"disablePolicyChecks":         false,
						"logging": map[string]interface{}{
							"level": "default:info",
						},
						"mtls": map[string]interface{}{
							"enabled": true,
						},
						"outboundTrafficPolicy": map[string]interface{}{
							"mode": "REGISTRY_ONLY",
						},
						"policyCheckFailOpen": false,
						"proxy": map[string]interface{}{
							"accessLogFile": "/dev/stdout",
						},
						"tls": map[string]interface{}{
							"minProtocolVersion": "TLSv1_2",
						},
					},
					"grafana": map[string]interface{}{
						"enabled": true,
					},
					"kiali": map[string]interface{}{
						"enabled": true,
					},
					"meshConfig": map[string]interface{}{
						"accessLogFile": "/dev/stdout",
						"outboundTrafficPolicy": map[string]interface{}{
							"mode": "REGISTRY_ONLY",
						},
					},
					"mixer": map[string]interface{}{
						"policy": map[string]interface{}{
							"autoscaleEnabled": false,
						},
						"telemetry": map[string]interface{}{
							"autoscaleEnabled": false,
						},
					},
					"pilot": map[string]interface{}{
						"autoscaleEnabled": false,
						"traceSampling":    float64(100),
					},
					"tracing": map[string]interface{}{
						"enabled": true,
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"esIndexCleaner": map[string]interface{}{
									"enabled":      true,
									"numberOfDays": int64(60),
									"schedule":     "55 23 * * *",
								},
								"nodeCount": int64(3),
							},
							"template": "production-elasticsearch",
						},
					},
				}),
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster":  globalMultiClusterDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"gateways": map[string]interface{}{
					"istio-ingressgateway": map[string]interface{}{
						"gatewayType": "ingress",
						"name":        "istio-ingressgateway",
					},
				},
				"meshConfig": map[string]interface{}{
					// a result of enabling tracing
					"enableTracing": nil,
				},
				"tracing": map[string]interface{}{
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "MAISTRA-1983.user.2",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				// specifying both Template and Profiles saves having to duplicate
				// everything in roundTripped, i.e. this will round trip exactly
				Template: "default",
				Profiles: []string{"default"},
				Istio: v1.NewHelmValues(map[string]interface{}{
					"gateways": map[string]interface{}{
						"istio-ingressgateway": map[string]interface{}{
							"autoscaleEnabled":      false,
							"externalTrafficPolicy": "Local",
							"ior_enabled":           false,
							"ports": []interface{}{
								map[string]interface{}{
									"name":       "status-port",
									"nodePort":   int64(30158),
									"port":       int64(15020),
									"protocol":   "TCP",
									"targetPort": int64(15020),
								},
								map[string]interface{}{
									"name":       "http2",
									"nodePort":   int64(32520),
									"port":       int64(80),
									"protocol":   "TCP",
									"targetPort": int64(8082),
								},
								map[string]interface{}{
									"name":       "https",
									"nodePort":   int64(32731),
									"port":       int64(443),
									"protocol":   "TCP",
									"targetPort": int64(8443),
								},
								map[string]interface{}{
									"name":       "tls",
									"nodePort":   int64(30462),
									"port":       int64(15443),
									"protocol":   "TCP",
									"targetPort": int64(15443),
								},
							},
							"replicaCount": int64(5),
							"sds": map[string]interface{}{
								"enabled": true,
								"image":   "istio/node-agent-k8s:1.5.9",
							},
							"type": "NodePort",
						},
					},
					"global": map[string]interface{}{
						"controlPlaneSecurityEnabled": true,
						"disablePolicyChecks":         false,
						"logging": map[string]interface{}{
							"level": "default:info",
						},
						"mtls": map[string]interface{}{
							"enabled": true,
						},
						"outboundTrafficPolicy": map[string]interface{}{
							"mode": "REGISTRY_ONLY",
						},
						"policyCheckFailOpen": false,
						"proxy": map[string]interface{}{
							"accessLogFile": "/dev/stdout",
						},
						"tls": map[string]interface{}{
							"minProtocolVersion": "TLSv1_2",
						},
					},
					"grafana": map[string]interface{}{
						"enabled": true,
					},
					"kiali": map[string]interface{}{
						"enabled": true,
					},
					"mixer": map[string]interface{}{
						"policy": map[string]interface{}{
							"autoscaleEnabled": false,
						},
						"telemetry": map[string]interface{}{
							"autoscaleEnabled": false,
						},
					},
					"pilot": map[string]interface{}{
						"autoscaleEnabled": false,
						"traceSampling":    int64(100),
					},
					"tracing": map[string]interface{}{
						"enabled": true,
						"jaeger": map[string]interface{}{
							"elasticsearch": map[string]interface{}{
								"esIndexCleaner": map[string]interface{}{
									"enabled":      true,
									"numberOfDays": int64(60),
									"schedule":     "55 23 * * *",
								},
								"nodeCount":        int64(3),
								"redundancyPolicy": nil,
							},
							"template": "production-bad",
						},
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version:  "v1.1",
				Profiles: []string{"default"},
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"errored": map[string]interface{}{
						"istio": map[string]interface{}{
							"gateways": map[string]interface{}{
								"istio-ingressgateway": map[string]interface{}{
									"autoscaleEnabled":      false,
									"externalTrafficPolicy": "Local",
									"ior_enabled":           false,
									"ports": []interface{}{
										map[string]interface{}{
											"name":       "status-port",
											"nodePort":   int64(30158),
											"port":       int64(15020),
											"protocol":   "TCP",
											"targetPort": int64(15020),
										},
										map[string]interface{}{
											"name":       "http2",
											"nodePort":   int64(32520),
											"port":       int64(80),
											"protocol":   "TCP",
											"targetPort": int64(8082),
										},
										map[string]interface{}{
											"name":       "https",
											"nodePort":   int64(32731),
											"port":       int64(443),
											"protocol":   "TCP",
											"targetPort": int64(8443),
										},
										map[string]interface{}{
											"name":       "tls",
											"nodePort":   int64(30462),
											"port":       int64(15443),
											"protocol":   "TCP",
											"targetPort": int64(15443),
										},
									},
									"replicaCount": int64(5),
									"sds": map[string]interface{}{
										"enabled": true,
										"image":   "istio/node-agent-k8s:1.5.9",
									},
									"type": "NodePort",
								},
							},
							"global": map[string]interface{}{
								"controlPlaneSecurityEnabled": true,
								"disablePolicyChecks":         false,
								"logging": map[string]interface{}{
									"level": "default:info",
								},
								"mtls": map[string]interface{}{
									"enabled": true,
								},
								"outboundTrafficPolicy": map[string]interface{}{
									"mode": "REGISTRY_ONLY",
								},
								"policyCheckFailOpen": false,
								"proxy": map[string]interface{}{
									"accessLogFile": "/dev/stdout",
								},
								"tls": map[string]interface{}{
									"minProtocolVersion": "TLSv1_2",
								},
							},
							"grafana": map[string]interface{}{
								"enabled": true,
							},
							"kiali": map[string]interface{}{
								"enabled": true,
							},
							"mixer": map[string]interface{}{
								"policy": map[string]interface{}{
									"autoscaleEnabled": false,
								},
								"telemetry": map[string]interface{}{
									"autoscaleEnabled": false,
								},
							},
							"pilot": map[string]interface{}{
								"autoscaleEnabled": false,
								"traceSampling":    int64(100),
							},
							"tracing": map[string]interface{}{
								"enabled": true,
								"jaeger": map[string]interface{}{
									"elasticsearch": map[string]interface{}{
										"esIndexCleaner": map[string]interface{}{
											"enabled":      true,
											"numberOfDays": int64(60),
											"schedule":     "55 23 * * *",
										},
										"nodeCount":        int64(3),
										"redundancyPolicy": nil,
									},
									"template": "production-bad",
								},
							},
						},
						"message": string("unknown jaeger.template: production-bad"),
					},
				}),
			},
		},
		{
			name: "MAISTRA-2014.name.jaeger",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"proxy": map[string]interface{}{
							"tracer": "zipkin",
						},
						"tracer": map[string]interface{}{
							"zipkin": map[string]interface{}{
								"address": "jaeger-collector.cp-namespace.svc.cluster.local:9411",
							},
						},
					},
					"tracing": map[string]interface{}{
						"enabled": false,
					},
					"kiali": map[string]interface{}{
						"jaegerInClusterURL": "jaeger-query.cp-namespace.svc.cluster.local",
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v1.1",
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Name: "jaeger",
					},
				},
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"tracer": map[string]interface{}{
							"zipkin": map[string]interface{}{
								"address": "jaeger-collector.cp-namespace.svc.cluster.local:9411",
							},
						},
					},
					"kiali": map[string]interface{}{
						"jaegerInClusterURL": "jaeger-query.cp-namespace.svc.cluster.local",
					},
				}),
			},
			roundTripped: &v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"enableTracing": true,
						"proxy": map[string]interface{}{
							"tracer": "zipkin",
						},
						"tracer": map[string]interface{}{
							"zipkin": map[string]interface{}{
								"address": "jaeger-collector.cp-namespace.svc.cluster.local:9411",
							},
						},
					},
					"kiali": map[string]interface{}{
						"jaegerInClusterURL": "jaeger-query.cp-namespace.svc.cluster.local",
					},
					"meshConfig": map[string]interface{}{
						// a result of enabling tracing
						"enableTracing": true,
					},
					"tracing": map[string]interface{}{
						"enabled": false,
					},
				}),
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
				},
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "jaeger",
					},
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "MAISTRA-2014.name.custom-jaeger",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"proxy": map[string]interface{}{
							"tracer": "zipkin",
						},
						"tracer": map[string]interface{}{
							"zipkin": map[string]interface{}{
								"address": "custom-jaeger-collector.cp-namespace.svc.cluster.local:9411",
							},
						},
					},
					"tracing": map[string]interface{}{
						"enabled": false,
					},
					"kiali": map[string]interface{}{
						"jaegerInClusterURL": "custom-jaeger-query.cp-namespace.svc.cluster.local",
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v1.1",
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Name: "custom-jaeger",
					},
				},
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"tracer": map[string]interface{}{
							"zipkin": map[string]interface{}{
								"address": "custom-jaeger-collector.cp-namespace.svc.cluster.local:9411",
							},
						},
					},
					"kiali": map[string]interface{}{
						"jaegerInClusterURL": "custom-jaeger-query.cp-namespace.svc.cluster.local",
					},
				}),
			},
			roundTripped: &v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"enableTracing": true,
						"proxy": map[string]interface{}{
							"tracer": "zipkin",
						},
						"tracer": map[string]interface{}{
							"zipkin": map[string]interface{}{
								"address": "custom-jaeger-collector.cp-namespace.svc.cluster.local:9411",
							},
						},
					},
					"kiali": map[string]interface{}{
						"jaegerInClusterURL": "custom-jaeger-query.cp-namespace.svc.cluster.local",
					},
					"meshConfig": map[string]interface{}{
						// a result of enabling tracing
						"enableTracing": true,
					},
					"tracing": map[string]interface{}{
						"enabled": false,
					},
				}),
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
				},
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "jaeger",
					},
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "cluster-scoped",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v2.3",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"clusterScoped": true,
					},
				}),
			},
			smcpv2: v2.ControlPlaneSpec{
				Version: "v2.3",
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					v2.TechPreviewControlPlaneModeKey: v2.TechPreviewControlPlaneModeValueClusterScoped,
				}),
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// mesh expansion is disabled by default
					"meshExpansion": globalMeshExpansionDefaults,
					// multicluster is disabled by default
					"multiCluster": globalMultiClusterDefaults,
				},
			}),
		},
	}
)

type conversionTestCase struct {
	name               string
	namespace          string
	spec               *v2.ControlPlaneSpec
	roundTripSpec      *v2.ControlPlaneSpec
	isolatedIstio      *v1.HelmValues
	isolatedThreeScale *v1.HelmValues
	completeIstio      *v1.HelmValues
	completeThreeScale *v1.HelmValues
}

func assertEquals(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if diff := cmp.Diff(expected, actual, cmp.AllowUnexported(v1.HelmValues{})); diff != "" {
		t.Logf("DeepEqual() failed, retrying after pruning empty/nil objects (-expected, +got):\n%s", diff)
		prunedExpected := pruneEmptyObjects(expected)
		prunedActual := pruneEmptyObjects(actual)
		if diff := cmp.Diff(prunedExpected, prunedActual, cmp.AllowUnexported(v1.HelmValues{})); diff != "" {
			t.Errorf("unexpected output converting values back to v2 (-expected, +got):\n%s", diff)
		}
	}
}

func pruneEmptyObjects(in interface{}) *v1.HelmValues {
	values, err := toValues(in)
	if err != nil {
		panic(fmt.Errorf("unexpected error converting value: %v", in))
	}
	pruneTree(values)
	return v1.NewHelmValues(values)
}

func pruneTree(in map[string]interface{}) {
	for restart := true; restart; {
		restart = false
		for key, rawValue := range in {
			switch value := rawValue.(type) {
			case []interface{}:
				if len(value) == 0 {
					delete(in, key)
				}
			case map[string]interface{}:
				pruneTree(value)
				if len(value) == 0 {
					delete(in, key)
				}
			}
		}
	}
}

func TestCompleteClusterConversionFromV2(t *testing.T) {
	runTestCasesFromV2(clusterTestCases, t)
}

func TestCompleteGatewaysConversionFromV2(t *testing.T) {
	runTestCasesFromV2(gatewaysTestCases, t)
}

func TestCompleteRuntimeConversionFromV2(t *testing.T) {
	runTestCasesFromV2(runtimeTestCases, t)
}

func TestCompleteProxyConversionFromV2(t *testing.T) {
	runTestCasesFromV2(proxyTestCases, t)
}

func TestCompleteLoggingConversionFromV2(t *testing.T) {
	runTestCasesFromV2(loggingTestCases, t)
}

func TestCompletePolicyConversionFromV2(t *testing.T) {
	runTestCasesFromV2(policyTestCases, t)
}

func TestCompleteTelemetryConversionFromV2(t *testing.T) {
	runTestCasesFromV2(telemetryTestCases, t)
}

func TestCompleteSecurityConversionFromV2(t *testing.T) {
	runTestCasesFromV2(securityTestCases, t)
}

func TestCompletePrometheusConversionFromV2(t *testing.T) {
	runTestCasesFromV2(prometheusTestCases, t)
}

func TestCompleteGrafanaConversionFromV2(t *testing.T) {
	runTestCasesFromV2(grafanaTestCases, t)
}

func TestCompleteKialiConversionFromV2(t *testing.T) {
	runTestCasesFromV2(kialiTestCases, t)
}

func TestCompleteJaegerConversionFromV2(t *testing.T) {
	runTestCasesFromV2(jaegerTestCases, t)
}

func TestCompleteThreeScaleConversionFromV2(t *testing.T) {
	runTestCasesFromV2(threeScaleTestCases, t)
}

func TestTechPreviewConversionFromV2(t *testing.T) {
	runTestCasesFromV2(techPreviewTestCases, t)
}

func TestRoundTripConversion(t *testing.T) {
	for _, tc := range roundTripTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skip()
			}
			smcpv1 := *tc.smcpv1.DeepCopy()
			smcpv2 := v2.ControlPlaneSpec{}
			err := Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(&smcpv1, &smcpv2, nil)
			if err != nil {
				t.Fatalf("error converting smcpv1 to smcpv2: %v", err)
			}
			if diff := cmp.Diff(tc.smcpv2, smcpv2, cmp.AllowUnexported(v1.HelmValues{})); diff != "" {
				t.Errorf("TestRoundTripConversion() case %s v1->v2 mismatch (-want +got):\n%s", tc.name, diff)
			}
			smcpv1 = v1.ControlPlaneSpec{}
			err = Convert_v2_ControlPlaneSpec_To_v1_ControlPlaneSpec(&smcpv2, &smcpv1, nil)
			if err != nil {
				t.Fatalf("error converting smcpv2 to smcpv1: %v", err)
			}
			//nolint:revive
			expected := &tc.smcpv1
			if tc.roundTripped != nil {
				expected = tc.roundTripped
			}
			if diff := cmp.Diff(expected, &smcpv1, cmp.AllowUnexported(v1.HelmValues{})); diff != "" {
				t.Logf("TestRoundTripConversion() case %s v2->v1 mismatch, will try again after removing cruft (-want +got):\n%s", tc.name, diff)
				removeHelmValues(smcpv1.Istio.GetContent(), tc.cruft.GetContent())
				if diff := cmp.Diff(expected, &smcpv1, cmp.AllowUnexported(v1.HelmValues{})); diff != "" {
					t.Errorf("TestRoundTripConversion() case %s v2->v1 mismatch (-want +got):\n%s", tc.name, diff)
				}
			}
		})
	}
}

func runTestCasesFromV2(testCases []conversionTestCase, t *testing.T) {
	scheme := runtime.NewScheme()
	v1.SchemeBuilder.AddToScheme(scheme)
	v2.SchemeBuilder.AddToScheme(scheme)
	localSchemeBuilder.AddToScheme(scheme)
	t.Helper()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			smcpv1 := &v1.ServiceMeshControlPlane{}
			smcpv2 := &v2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: tc.namespace,
				},
				Spec: *tc.spec.DeepCopy(),
			}

			if err := scheme.Convert(smcpv2, smcpv1, nil); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			istio := tc.isolatedIstio.DeepCopy().GetContent()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), istio)
			if diff := cmp.Diff(istio, smcpv1.Spec.Istio.DeepCopy().GetContent()); diff != "" {
				t.Errorf("unexpected output converting v2 to Istio values: %s", diff)
			}
			threeScale := tc.isolatedThreeScale.DeepCopy().GetContent()
			mergeMaps(tc.completeThreeScale.DeepCopy().GetContent(), threeScale)
			if diff := cmp.Diff(threeScale, smcpv1.Spec.ThreeScale.DeepCopy().GetContent()); diff != "" {
				t.Errorf("unexpected output converting v2 to ThreeScale values:%s", diff)
			}
			newsmcpv2 := &v2.ServiceMeshControlPlane{}
			// use expected data
			smcpv1.Spec.Istio = v1.NewHelmValues(istio).DeepCopy()
			smcpv1.Spec.ThreeScale = v1.NewHelmValues(threeScale).DeepCopy()
			if err := scheme.Convert(smcpv1, newsmcpv2, nil); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			if tc.roundTripSpec != nil {
				t.Logf("Substituting roundTripSpec for actual result, differences between expected and substituted are:\n%s",
					cmp.Diff(smcpv2.Spec, *tc.roundTripSpec, cmp.AllowUnexported(v1.HelmValues{})))
				smcpv2.Spec = v2.ControlPlaneSpec{}
				tc.roundTripSpec.DeepCopyInto(&smcpv2.Spec)
			}
			assertEquals(t, smcpv2, newsmcpv2)
		})
	}
}

type conversionTestCaseV2 struct {
	name               string
	spec               *v2.ControlPlaneSpec
	expectedHelmValues v1.HelmValues
}

func TestConversion(t *testing.T) {
	var testCases []conversionTestCaseV2
	testCases = append(testCases, jaegerConversionTestCasesV2...)
	testCases = append(testCases, securityConversionTestCasesV2...)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run(tc.name+"-v2_to_v1", func(t *testing.T) {
				var specV1 v1.ControlPlaneSpec
				if err := Convert_v2_ControlPlaneSpec_To_v1_ControlPlaneSpec(tc.spec.DeepCopy(), &specV1, nil); err != nil {
					t.Errorf("failed to convert SMCP v2 to v1: %s", err)
				}

				if !reflect.DeepEqual(tc.expectedHelmValues.DeepCopy(), specV1.Istio.DeepCopy()) {
					t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.expectedHelmValues.GetContent(), specV1.Istio.GetContent())
				}
			})

			t.Run(tc.name+"-v1_to_v2", func(t *testing.T) {
				specV1 := v1.ControlPlaneSpec{
					Istio: tc.expectedHelmValues.DeepCopy(),
				}
				specV2 := v2.ControlPlaneSpec{}
				if err := Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(&specV1, &specV2, nil); err != nil {
					t.Errorf("failed to convert SMCP v1 to v2: %s", err)
				}

				assertEquals(t, tc.spec.Security, specV2.Security)
			})
		})
	}
}
