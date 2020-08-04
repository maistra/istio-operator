package conversion

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	jaegerMaxTraces              = int64(15000)
	jaegerElasticsearchNodeCount = int32(5)
)

var jaegerTestCases = []conversionTestCase{
	{
		name: "none." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeNone,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"tracing": map[string]interface{}{
				"enabled":  false,
				"provider": "none",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type:   v2.TracerTypeJaeger,
					Jaeger: nil,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type:   v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install": false,
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "simple." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      false,
					"resourceName": "my-jaeger",
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name:    "my-jaeger",
						Install: &v2.JaegerInstallConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.storage.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Config: v2.JaegerConfig{
								Storage: &v2.JaegerStorageConfig{},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.storage.memory.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Config: v2.JaegerConfig{
								Storage: &v2.JaegerStorageConfig{
									Type: v2.JaegerStorageTypeMemory,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "all-in-one",
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.storage.memory.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Config: v2.JaegerConfig{
								Storage: &v2.JaegerStorageConfig{
									Type:   v2.JaegerStorageTypeMemory,
									Memory: &v2.JaegerMemoryStorageConfig{},
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "all-in-one",
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.storage.memory.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Config: v2.JaegerConfig{
								Storage: &v2.JaegerStorageConfig{
									Type: v2.JaegerStorageTypeMemory,
									Memory: &v2.JaegerMemoryStorageConfig{
										MaxTraces: &jaegerMaxTraces,
									},
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "all-in-one",
					"memory": map[string]interface{}{
						"max_traces": 15000,
					},
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.storage.elasticsearch.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Config: v2.JaegerConfig{
								Storage: &v2.JaegerStorageConfig{
									Type: v2.JaegerStorageTypeElasticsearch,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "production-elasticsearch",
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.storage.elasticsearch.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Config: v2.JaegerConfig{
								Storage: &v2.JaegerStorageConfig{
									Type:          v2.JaegerStorageTypeElasticsearch,
									Elasticsearch: &v2.JaegerElasticsearchStorageConfig{},
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "production-elasticsearch",
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.storage.elasticsearch.basic." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Config: v2.JaegerConfig{
								Storage: &v2.JaegerStorageConfig{
									Type: v2.JaegerStorageTypeElasticsearch,
									Elasticsearch: &v2.JaegerElasticsearchStorageConfig{
										NodeCount: &jaegerElasticsearchNodeCount,
										IndexCleaner: v1.NewHelmValues(map[string]interface{}{
											"enabled":      true,
											"numberOfDays": 7,
											"schedule":     "55 23 * * *",
										}).DeepCopy(),
										RedundancyPolicy: "ZeroRedundancy",
										Storage: v1.NewHelmValues(map[string]interface{}{
											"storageClassName": "gp2",
											"size":             "5Gi",
										}).DeepCopy(),
									},
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "production-elasticsearch",
					"elasticsearch": map[string]interface{}{
						"nodeCount": 5,
						"esIndexCleaner": map[string]interface{}{
							"enabled":      true,
							"numberOfDays": 7,
							"schedule":     "55 23 * * *"},
						"redundancyPolicy": "ZeroRedundancy",
						"storage": map[string]interface{}{
							"size":             "5Gi",
							"storageClassName": "gp2",
						},
					},
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.storage.elasticsearch.runtime.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Config: v2.JaegerConfig{
								Storage: &v2.JaegerStorageConfig{
									Type: v2.JaegerStorageTypeElasticsearch,
									Elasticsearch: &v2.JaegerElasticsearchStorageConfig{
										Runtime: &v2.PodRuntimeConfig{},
									},
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "production-elasticsearch",
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.storage.elasticsearch.runtime.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Config: v2.JaegerConfig{
								Storage: &v2.JaegerStorageConfig{
									Type: v2.JaegerStorageTypeElasticsearch,
									Elasticsearch: &v2.JaegerElasticsearchStorageConfig{
										Runtime: &v2.PodRuntimeConfig{
											CommonPodRuntimeConfig: v2.CommonPodRuntimeConfig{
												NodeSelector: map[string]string{
													"node-label": "node-value",
												},
												PriorityClassName: "normal",
												Tolerations: []corev1.Toleration{
													{
														Key:      "bad-node",
														Operator: corev1.TolerationOpExists,
														Effect:   corev1.TaintEffectNoExecute,
													},
													{
														Key:      "istio",
														Operator: corev1.TolerationOpEqual,
														Value:    "disabled",
														Effect:   corev1.TaintEffectNoSchedule,
													},
												},
											},
											Affinity: &v2.Affinity{
												PodAntiAffinity: v2.PodAntiAffinity{
													PreferredDuringScheduling: []v2.PodAntiAffinityTerm{
														{
															LabelSelectorRequirement: metav1.LabelSelectorRequirement{
																Key:      "istio",
																Operator: metav1.LabelSelectorOpIn,
																Values: []string{
																	"control-plane",
																},
															},
														},
													},
													RequiredDuringScheduling: []v2.PodAntiAffinityTerm{
														{
															LabelSelectorRequirement: metav1.LabelSelectorRequirement{
																Key:      "istio",
																Operator: metav1.LabelSelectorOpIn,
																Values: []string{
																	"ingressgateway",
																},
															},
														},
													},
												},
											},
											Metadata: v2.MetadataConfig{
												Annotations: map[string]string{
													"some-pod-annotation": "pod-annotation-value",
												},
												Labels: map[string]string{
													"some-pod-label": "pod-label-value",
												},
											},
											Containers: map[string]v2.ContainerConfig{
												"elasticsearch": {
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
													Image: "custom-elasticsearch",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "production-elasticsearch",
					"elasticsearch": map[string]interface{}{
						"nodeSelector": map[string]interface{}{
							"node-label": "node-value",
						},
						"priorityClassName": "normal",
						"podAntiAffinityTermLabelSelector": []interface{}{
							map[string]interface{}{
								"key":         "istio",
								"operator":    "In",
								"topologyKey": "",
								"values":      "control-plane",
							},
						},
						"podAntiAffinityLabelSelector": []interface{}{
							map[string]interface{}{
								"key":         "istio",
								"operator":    "In",
								"topologyKey": "",
								"values":      "ingressgateway",
							},
						},
						"tolerations": []interface{}{
							map[string]interface{}{
								"effect":   "NoExecute",
								"key":      "bad-node",
								"operator": "Exists",
							},
							map[string]interface{}{
								"effect":   "NoSchedule",
								"key":      "istio",
								"operator": "Equal",
								"value":    "disabled",
							},
						},
						"podAnnotations": map[string]interface{}{
							"some-pod-annotation": "pod-annotation-value",
						},
						"podLabels": map[string]interface{}{
							"some-pod-label": "pod-label-value",
						},
						"hub":             "custom-registry",
						"image":           "custom-elasticsearch",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.runtime.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Runtime: &v2.ComponentRuntimeConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
				},
				"autoscaleEnabled": false,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.runtime.basic." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Runtime: &v2.ComponentRuntimeConfig{
								Deployment: v2.DeploymentRuntimeConfig{
									Replicas: &replicaCount2,
									Strategy: &appsv1.DeploymentStrategy{
										RollingUpdate: &appsv1.RollingUpdateDeployment{
											MaxSurge:       &intStrInt1,
											MaxUnavailable: &intStr25Percent,
										},
									},
								},
								Pod: v2.PodRuntimeConfig{
									CommonPodRuntimeConfig: v2.CommonPodRuntimeConfig{
										NodeSelector: map[string]string{
											"node-label": "node-value",
										},
										PriorityClassName: "normal",
										Tolerations: []corev1.Toleration{
											{
												Key:      "bad-node",
												Operator: corev1.TolerationOpExists,
												Effect:   corev1.TaintEffectNoExecute,
											},
											{
												Key:      "istio",
												Operator: corev1.TolerationOpEqual,
												Value:    "disabled",
												Effect:   corev1.TaintEffectNoSchedule,
											},
										},
									},
									Affinity: &v2.Affinity{
										PodAntiAffinity: v2.PodAntiAffinity{
											PreferredDuringScheduling: []v2.PodAntiAffinityTerm{
												{
													LabelSelectorRequirement: metav1.LabelSelectorRequirement{
														Key:      "istio",
														Operator: metav1.LabelSelectorOpIn,
														Values: []string{
															"control-plane",
														},
													},
												},
											},
											RequiredDuringScheduling: []v2.PodAntiAffinityTerm{
												{
													LabelSelectorRequirement: metav1.LabelSelectorRequirement{
														Key:      "istio",
														Operator: metav1.LabelSelectorOpIn,
														Values: []string{
															"ingressgateway",
														},
													},
												},
											},
										},
									},
									Metadata: v2.MetadataConfig{
										Annotations: map[string]string{
											"some-pod-annotation": "pod-annotation-value",
										},
										Labels: map[string]string{
											"some-pod-label": "pod-label-value",
										},
									},
									Containers: map[string]v2.ContainerConfig{
										"default": {
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
											Image: "custom-jaeger",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"podAnnotations": map[string]interface{}{
						"some-pod-annotation": "pod-annotation-value",
					},
					"podLabels": map[string]interface{}{
						"some-pod-label": "pod-label-value",
					},
					"hub":             "custom-registry",
					"image":           "custom-jaeger",
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
				"autoscaleEnabled":      false,
				"replicaCount":          2,
				"rollingMaxSurge":       1,
				"rollingMaxUnavailable": "25%",
				"nodeSelector": map[string]interface{}{
					"node-label": "node-value",
				},
				"priorityClassName": "normal",
				"tolerations": []interface{}{
					map[string]interface{}{
						"effect":   "NoExecute",
						"key":      "bad-node",
						"operator": "Exists",
					},
					map[string]interface{}{
						"effect":   "NoSchedule",
						"key":      "istio",
						"operator": "Equal",
						"value":    "disabled",
					},
				},
				"podAntiAffinityTermLabelSelector": []interface{}{
					map[string]interface{}{
						"key":         "istio",
						"operator":    "In",
						"topologyKey": "",
						"values":      "control-plane",
					},
				},
				"podAntiAffinityLabelSelector": []interface{}{
					map[string]interface{}{
						"key":         "istio",
						"operator":    "In",
						"topologyKey": "",
						"values":      "ingressgateway",
					},
				},
				"podAnnotations": map[string]interface{}{
					"some-pod-annotation": "pod-annotation-value",
				},
				"podLabels": map[string]interface{}{
					"some-pod-label": "pod-label-value",
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.ingress.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Ingress: &v2.JaegerIngressConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "install.ingress.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Ingress: &v2.JaegerIngressConfig{
								Enablement: v2.Enablement{
									Enabled: &featureEnabled,
								},
								Metadata: v2.MetadataConfig{
									Annotations: map[string]string{
										"ingress-annotation": "ingress-annotation-value",
									},
									Labels: map[string]string{
										"ingress-label": "ingress-label-value",
									},
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
				},
				"ingress": map[string]interface{}{
					"enabled": true,
					"annotations": map[string]interface{}{
						"ingress-annotation": "ingress-annotation-value",
					},
					"labels": map[string]interface{}{
						"ingress-label": "ingress-label-value",
					},
				},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
}

func TestJaegerConversionFromV2(t *testing.T) {
	for _, tc := range jaegerTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateAddonsValues(specCopy, helmValues.GetContent()); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			if !reflect.DeepEqual(tc.isolatedIstio.DeepCopy(), helmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.isolatedIstio.GetContent(), helmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), helmValues.GetContent())
			if err := populateAddonsConfig(helmValues.DeepCopy(), specv2); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			if !reflect.DeepEqual(tc.spec.Addons, specv2.Addons) {
				expected, _ := yaml.Marshal(tc.spec.Addons)
				got, _ := yaml.Marshal(specv2.Addons)
				t.Errorf("unexpected output converting values back to v2:\n\texpected:\n%s\n\tgot:\n%s", string(expected), string(got))
			}
		})
	}
}
