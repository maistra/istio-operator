package conversion

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	kialiTestNodePort = int32(12345)
)

var kialiTestCases = []conversionTestCase{
	{
		name: "nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: nil,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
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
		name: "defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
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
		name: "simple." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
						Name: "my-kiali",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"kiali": map[string]interface{}{
				"enabled":      true,
				"resourceName": "my-kiali",
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
		name: "install.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{
						Name:    "my-kiali",
						Install: &v2.KialiInstallConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"kiali": map[string]interface{}{
				"resourceName": "my-kiali",
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
		name: "install.config.simple." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{
						Name: "my-kiali",
						Install: &v2.KialiInstallConfig{
							Config: v2.KialiConfig{
								Dashboard: v2.KialiDashboardConfig{
									EnableGrafana:    &featureEnabled,
									EnablePrometheus: &featureEnabled,
									EnableTracing:    &featureDisabled,
									ViewOnly:         &featureEnabled,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"kiali": map[string]interface{}{
				"resourceName": "my-kiali",
				"dashboard": map[string]interface{}{
					"enableGrafana":    true,
					"enablePrometheus": true,
					"enableTracing":    false,
					"viewOnlyMode":     true,
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
		}),
	},
	{
		name: "install.service.misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{
						Name: "my-kiali",
						Install: &v2.KialiInstallConfig{
							Service: v2.ComponentServiceConfig{
								Metadata: v2.MetadataConfig{
									Annotations: map[string]string{
										"some-service-annotation": "service-annotation-value",
									},
									Labels: map[string]string{
										"some-service-label": "service-label-value",
									},
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"kiali": map[string]interface{}{
				"resourceName": "my-kiali",
				"service": map[string]interface{}{
					"annotations": map[string]interface{}{
						"some-service-annotation": "service-annotation-value",
					},
					"labels": map[string]interface{}{
						"some-service-label": "service-label-value",
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
		}),
	},
	{
		name: "install.service.ingress.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{
						Name: "my-kiali",
						Install: &v2.KialiInstallConfig{
							Service: v2.ComponentServiceConfig{
								Ingress: &v2.ComponentIngressConfig{},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"kiali": map[string]interface{}{
				"resourceName": "my-kiali",
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
		name: "install.service.ingress.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{
						Name: "my-kiali",
						Install: &v2.KialiInstallConfig{
							Service: v2.ComponentServiceConfig{
								Ingress: &v2.ComponentIngressConfig{
									Enablement: v2.Enablement{
										Enabled: &featureEnabled,
									},
									ContextPath: "/kiali",
									Hosts: []string{
										"kiali.example.com",
									},
									Metadata: v2.MetadataConfig{
										Annotations: map[string]string{
											"ingress-annotation": "ingress-annotation-value",
										},
										Labels: map[string]string{
											"ingress-label": "ingress-label-value",
										},
									},
									TLS: v1.NewHelmValues(map[string]interface{}{
										"termination": "reencrypt",
									}),
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"kiali": map[string]interface{}{
				"resourceName": "my-kiali",
				"ingress": map[string]interface{}{
					"enabled":     true,
					"contextPath": "/kiali",
					"annotations": map[string]interface{}{
						"ingress-annotation": "ingress-annotation-value",
					},
					"labels": map[string]interface{}{
						"ingress-label": "ingress-label-value",
					},
					"hosts": []interface{}{
						"kiali.example.com",
					},
					"tls": map[string]interface{}{
						"termination": "reencrypt",
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
		}),
	},
	{
		name: "install.service.nodeport." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{
						Name: "my-kiali",
						Install: &v2.KialiInstallConfig{
							Service: v2.ComponentServiceConfig{
								NodePort: &kialiTestNodePort,
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"kiali": map[string]interface{}{
				"resourceName": "my-kiali",
				"service": map[string]interface{}{
					"nodePort": map[string]interface{}{
						"enabled": true,
						"port":    12345,
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
		}),
	},
	{
		name: "install.runtime.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{
						Name: "my-kiali",
						Install: &v2.KialiInstallConfig{
							Runtime: &v2.ComponentRuntimeConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"kiali": map[string]interface{}{
				"resourceName":     "my-kiali",
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
		}),
	},
	{
		name: "install.runtime.basic." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{
						Install: &v2.KialiInstallConfig{
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
										"kiali": {
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
											Image: "custom-kiali",
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
			"kiali": map[string]interface{}{
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
				"hub":             "custom-registry",
				"image":           "custom-kiali",
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
		name: "install.runtime.autoscale." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Visualization: v2.VisualizationAddonsConfig{
					Kiali: &v2.KialiAddonConfig{
						Install: &v2.KialiInstallConfig{
							Runtime: &v2.ComponentRuntimeConfig{
								Deployment: v2.DeploymentRuntimeConfig{
									Replicas: &replicaCount2,
									AutoScaling: &v2.AutoScalerConfig{
										MaxReplicas:                    &replicaCount5,
										MinReplicas:                    &replicaCount1,
										TargetCPUUtilizationPercentage: &cpuUtilization80,
									},
									Strategy: &appsv1.DeploymentStrategy{
										RollingUpdate: &appsv1.RollingUpdateDeployment{
											MaxSurge:       &intStr25Percent,
											MaxUnavailable: &intStrInt1,
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
			"kiali": map[string]interface{}{
				"autoscaleEnabled": true,
				"autoscaleMax":     5,
				"autoscaleMin":     1,
				"cpu": map[string]interface{}{
					"targetAverageUtilization": 80,
				},
				"replicaCount":          2,
				"rollingMaxSurge":       "25%",
				"rollingMaxUnavailable": 1,
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

func TestKialiConversionFromV2(t *testing.T) {
	for _, tc := range kialiTestCases {
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
			assertEquals(t, tc.spec.Addons, specv2.Addons)
		})
	}
}
