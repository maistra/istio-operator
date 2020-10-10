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

var runtimeTestCases = []conversionTestCase{
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
		// XXX: round-trip fails
		name: "defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{},
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
		name: "defaults.poddisruption.1." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Defaults: &v2.DefaultRuntimeConfig{
					Deployment: &v2.CommonDeploymentRuntimeConfig{
						PodDisruption: &v2.PodDisruptionBudget{
							MaxUnavailable: &intStr25Percent,
							MinAvailable:   &intStrInt1,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"defaultPodDisruptionBudget": map[string]interface{}{
					"enabled":        true,
					"maxUnavailable": "25%",
					"minAvailable":   1,
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
		name: "defaults.poddisruption.2." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Defaults: &v2.DefaultRuntimeConfig{
					Deployment: &v2.CommonDeploymentRuntimeConfig{
						PodDisruption: &v2.PodDisruptionBudget{
							MaxUnavailable: &intStrInt1,
							MinAvailable:   &intStr25Percent,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"defaultPodDisruptionBudget": map[string]interface{}{
					"enabled":        true,
					"maxUnavailable": 1,
					"minAvailable":   "25%",
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
		name: "defaults.scheduling." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Defaults: &v2.DefaultRuntimeConfig{
					Pod: &v2.CommonPodRuntimeConfig{
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
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"defaultNodeSelector": map[string]interface{}{
					"node-label": "node-value",
				},
				"defaultTolerations": []interface{}{
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
				"priorityClassName": "normal",
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
		name: "defaults.container." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Defaults: &v2.DefaultRuntimeConfig{
					Container: &v2.CommonContainerConfig{
						ImageRegistry:   "custom-registry",
						ImageTag:        "2.0",
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
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"hub":             "custom-registry",
				"tag":             "2.0",
				"imagePullPolicy": "Always",
				"imagePullSecrets": []interface{}{
					"pull-secret",
				},
				"defaultResources": map[string]interface{}{
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
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "citadel.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameSecurity: {},
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
		name: "citadel.pod." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameSecurity: {
						Pod: &v2.PodRuntimeConfig{
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
							Metadata: &v2.MetadataConfig{
								Annotations: map[string]string{
									"some-pod-annotation": "pod-annotation-value",
								},
								Labels: map[string]string{
									"some-pod-label": "pod-label-value",
								},
							},
						},
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
							Image: "custom-citadel",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"security": map[string]interface{}{
				"nodeSelector": map[string]interface{}{
					"node-label": "node-value",
				},
				"podAntiAffinityLabelSelector": []interface{}{
					map[string]interface{}{
						"key":         "istio",
						"operator":    "In",
						"topologyKey": "",
						"values":      "ingressgateway",
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
				"podAnnotations": map[string]interface{}{
					"some-pod-annotation": "pod-annotation-value",
				},
				"podLabels": map[string]interface{}{
					"some-pod-label": "pod-label-value",
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
				"hub":             "custom-registry",
				"image":           "custom-citadel",
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
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "citadel.deployment." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameSecurity: {
						Deployment: &v2.DeploymentRuntimeConfig{
							Replicas: &replicaCount2,
							Strategy: &appsv1.DeploymentStrategy{
								RollingUpdate: &appsv1.RollingUpdateDeployment{
									MaxSurge:       &intStrInt1,
									MaxUnavailable: &intStr25Percent,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"security": map[string]interface{}{
				"replicaCount":          2,
				"rollingMaxSurge":       1,
				"rollingMaxUnavailable": "25%",
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
		name: "citadel.deployment.autoscale." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameSecurity: {
						Deployment: &v2.DeploymentRuntimeConfig{
							AutoScaling: &v2.AutoScalerConfig{
								Enablement:                     v2.Enablement{Enabled: &featureEnabled},
								MaxReplicas:                    &replicaCount5,
								MinReplicas:                    &replicaCount1,
								TargetCPUUtilizationPercentage: &cpuUtilization80,
							},
							Replicas: &replicaCount2,
							Strategy: &appsv1.DeploymentStrategy{
								RollingUpdate: &appsv1.RollingUpdateDeployment{
									MaxSurge:       &intStrInt1,
									MaxUnavailable: &intStr25Percent,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"security": map[string]interface{}{
				"autoscaleEnabled": true,
				"autoscaleMax":     5,
				"autoscaleMin":     1,
				"cpu": map[string]interface{}{
					"targetAverageUtilization": 80,
				},
				"replicaCount":          2,
				"rollingMaxSurge":       1,
				"rollingMaxUnavailable": "25%",
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
		name: "galley.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameGalley: {},
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
		name: "galley.pod." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameGalley: {
						Pod: &v2.PodRuntimeConfig{
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
							Metadata: &v2.MetadataConfig{
								Annotations: map[string]string{
									"some-pod-annotation": "pod-annotation-value",
								},
								Labels: map[string]string{
									"some-pod-label": "pod-label-value",
								},
							},
						},
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
							Image: "custom-citadel",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"galley": map[string]interface{}{
				"nodeSelector": map[string]interface{}{
					"node-label": "node-value",
				},
				"podAntiAffinityLabelSelector": []interface{}{
					map[string]interface{}{
						"key":         "istio",
						"operator":    "In",
						"topologyKey": "",
						"values":      "ingressgateway",
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
				"podAnnotations": map[string]interface{}{
					"some-pod-annotation": "pod-annotation-value",
				},
				"podLabels": map[string]interface{}{
					"some-pod-label": "pod-label-value",
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
				"hub":             "custom-registry",
				"image":           "custom-citadel",
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
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "galley.deployment." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameGalley: {
						Deployment: &v2.DeploymentRuntimeConfig{
							Replicas: &replicaCount2,
							Strategy: &appsv1.DeploymentStrategy{
								RollingUpdate: &appsv1.RollingUpdateDeployment{
									MaxSurge:       &intStrInt1,
									MaxUnavailable: &intStr25Percent,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"galley": map[string]interface{}{
				"replicaCount":          2,
				"rollingMaxSurge":       1,
				"rollingMaxUnavailable": "25%",
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
		name: "galley.deployment.autoscale." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameGalley: {
						Deployment: &v2.DeploymentRuntimeConfig{
							AutoScaling: &v2.AutoScalerConfig{
								Enablement:                     v2.Enablement{Enabled: &featureEnabled},
								MaxReplicas:                    &replicaCount5,
								MinReplicas:                    &replicaCount1,
								TargetCPUUtilizationPercentage: &cpuUtilization80,
							},
							Replicas: &replicaCount2,
							Strategy: &appsv1.DeploymentStrategy{
								RollingUpdate: &appsv1.RollingUpdateDeployment{
									MaxSurge:       &intStrInt1,
									MaxUnavailable: &intStr25Percent,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"galley": map[string]interface{}{
				"autoscaleEnabled": true,
				"autoscaleMax":     5,
				"autoscaleMin":     1,
				"cpu": map[string]interface{}{
					"targetAverageUtilization": 80,
				},
				"replicaCount":          2,
				"rollingMaxSurge":       1,
				"rollingMaxUnavailable": "25%",
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
		name: "pilot.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNamePilot: {},
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
		name: "pilot.pod." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNamePilot: {
						Pod: &v2.PodRuntimeConfig{
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
							Metadata: &v2.MetadataConfig{
								Annotations: map[string]string{
									"some-pod-annotation": "pod-annotation-value",
								},
								Labels: map[string]string{
									"some-pod-label": "pod-label-value",
								},
							},
						},
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
							Image: "custom-citadel",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"nodeSelector": map[string]interface{}{
					"node-label": "node-value",
				},
				"podAntiAffinityLabelSelector": []interface{}{
					map[string]interface{}{
						"key":         "istio",
						"operator":    "In",
						"topologyKey": "",
						"values":      "ingressgateway",
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
				"podAnnotations": map[string]interface{}{
					"some-pod-annotation": "pod-annotation-value",
				},
				"podLabels": map[string]interface{}{
					"some-pod-label": "pod-label-value",
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
				"hub":             "custom-registry",
				"image":           "custom-citadel",
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
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "pilot.deployment." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNamePilot: {
						Deployment: &v2.DeploymentRuntimeConfig{
							Replicas: &replicaCount2,
							Strategy: &appsv1.DeploymentStrategy{
								RollingUpdate: &appsv1.RollingUpdateDeployment{
									MaxSurge:       &intStrInt1,
									MaxUnavailable: &intStr25Percent,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"replicaCount":          2,
				"rollingMaxSurge":       1,
				"rollingMaxUnavailable": "25%",
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
		name: "pilot.deployment.autoscale." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNamePilot: {
						Deployment: &v2.DeploymentRuntimeConfig{
							AutoScaling: &v2.AutoScalerConfig{
								Enablement:                     v2.Enablement{Enabled: &featureEnabled},
								MaxReplicas:                    &replicaCount5,
								MinReplicas:                    &replicaCount1,
								TargetCPUUtilizationPercentage: &cpuUtilization80,
							},
							Replicas: &replicaCount2,
							Strategy: &appsv1.DeploymentStrategy{
								RollingUpdate: &appsv1.RollingUpdateDeployment{
									MaxSurge:       &intStrInt1,
									MaxUnavailable: &intStr25Percent,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"autoscaleEnabled": true,
				"autoscaleMax":     5,
				"autoscaleMin":     1,
				"cpu": map[string]interface{}{
					"targetAverageUtilization": 80,
				},
				"replicaCount":          2,
				"rollingMaxSurge":       1,
				"rollingMaxUnavailable": "25%",
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
		name: "pilot.env." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNamePilot: {
						Container: &v2.ContainerConfig{
							Env: map[string]string{
								"PILOT_PUSH_THROTTLE": "100",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"env": map[string]interface{}{
					"PILOT_PUSH_THROTTLE": "100",
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
		name: "mixer.policy.runtime.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameMixerPolicy: {},
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
		name: "mixer.policy.runtime.basic." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameMixerPolicy: {
						Deployment: &v2.DeploymentRuntimeConfig{
							Replicas: &replicaCount2,
							Strategy: &appsv1.DeploymentStrategy{
								RollingUpdate: &appsv1.RollingUpdateDeployment{
									MaxSurge:       &intStrInt1,
									MaxUnavailable: &intStr25Percent,
								},
							},
						},
						Pod: &v2.PodRuntimeConfig{
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
							Metadata: &v2.MetadataConfig{
								Annotations: map[string]string{
									"some-pod-annotation": "pod-annotation-value",
								},
								Labels: map[string]string{
									"some-pod-label": "pod-label-value",
								},
							},
						},
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
							Image: "custom-mixer",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"policy": map[string]interface{}{
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
					"image":           "custom-mixer",
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
	{
		name: "mixer.policy.runtime.autoscale." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameMixerPolicy: {
						Deployment: &v2.DeploymentRuntimeConfig{
							Replicas: &replicaCount2,
							AutoScaling: &v2.AutoScalerConfig{
								Enablement:                     v2.Enablement{Enabled: &featureEnabled},
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
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"policy": map[string]interface{}{
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
		name: "mixer.runtime.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameMixerTelemetry: {},
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
		name: "mixer.runtime.basic." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameMixerTelemetry: {
						Deployment: &v2.DeploymentRuntimeConfig{
							Replicas: &replicaCount2,
							Strategy: &appsv1.DeploymentStrategy{
								RollingUpdate: &appsv1.RollingUpdateDeployment{
									MaxSurge:       &intStrInt1,
									MaxUnavailable: &intStr25Percent,
								},
							},
						},
						Pod: &v2.PodRuntimeConfig{
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
							Metadata: &v2.MetadataConfig{
								Annotations: map[string]string{
									"some-pod-annotation": "pod-annotation-value",
								},
								Labels: map[string]string{
									"some-pod-label": "pod-label-value",
								},
							},
						},
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
							Image: "custom-mixer",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
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
					"image":           "custom-mixer",
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
	{
		name: "mixer.runtime.autoscale." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameMixerTelemetry: {
						Deployment: &v2.DeploymentRuntimeConfig{
							Replicas: &replicaCount2,
							AutoScaling: &v2.AutoScalerConfig{
								Enablement:                     v2.Enablement{Enabled: &featureEnabled},
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
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
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
		name: "jaeger.runtime.basic." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameTracing: {
						Deployment: &v2.DeploymentRuntimeConfig{
							Replicas: &replicaCount2,
							Strategy: &appsv1.DeploymentStrategy{
								RollingUpdate: &appsv1.RollingUpdateDeployment{
									MaxSurge:       &intStrInt1,
									MaxUnavailable: &intStr25Percent,
								},
							},
						},
						Pod: &v2.PodRuntimeConfig{
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
							Metadata: &v2.MetadataConfig{
								Annotations: map[string]string{
									"some-pod-annotation": "pod-annotation-value",
								},
								Labels: map[string]string{
									"some-pod-label": "pod-label-value",
								},
							},
						},
					},
					v2.ControlPlaneComponentNameTracingJaeger: {
						Pod: &v2.PodRuntimeConfig{
							Metadata: &v2.MetadataConfig{
								Annotations: map[string]string{
									"some-pod-annotation": "pod-annotation-value",
								},
								Labels: map[string]string{
									"some-pod-label": "pod-label-value",
								},
							},
						},
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
							Image: "custom-jaeger",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"tracing": map[string]interface{}{
				"jaeger": map[string]interface{}{
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
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
			"tracing": map[string]interface{}{
				"jaeger": map[string]interface{}{
					"podAnnotations": nil,
					"annotations": map[string]interface{}{
						"some-pod-annotation": "pod-annotation-value",
					},
				},
			},
		}),
	},
	{
		name: "jaeger.runtime.images." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameTracingJaegerAgent: {
						Container: &v2.ContainerConfig{
							Image: "custom-agent",
						},
					},
					v2.ControlPlaneComponentNameTracingJaegerAllInOne: {
						Container: &v2.ContainerConfig{
							Image: "custom-all-in-one",
						},
					},
					v2.ControlPlaneComponentNameTracingJaegerCollector: {
						Container: &v2.ContainerConfig{
							Image: "custom-collector",
						},
					},
					v2.ControlPlaneComponentNameTracingJaegerQuery: {
						Container: &v2.ContainerConfig{
							Image: "custom-query",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"tracing": map[string]interface{}{
				"jaeger": map[string]interface{}{
					"agent": map[string]interface{}{
						"image": "custom-agent",
					},
					"allInOne": map[string]interface{}{
						"image": "custom-all-in-one",
					},
					"collector": map[string]interface{}{
						"image": "custom-collector",
					},
					"query": map[string]interface{}{
						"image": "custom-query",
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
			"tracing": map[string]interface{}{
				"jaeger": map[string]interface{}{
					"agent":          nil,
					"agentImage":     "custom-agent",
					"allInOne":       nil,
					"allInOneImage":  "custom-all-in-one",
					"collector":      nil,
					"collectorImage": "custom-collector",
					"query":          nil,
					"queryImage":     "custom-query",
				},
			},
		}),
	},
	{
		name: "jaeger.elasticsearch.runtime.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Runtime: &v2.ControlPlaneRuntimeConfig{
				Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
					v2.ControlPlaneComponentNameTracingJaegerElasticsearch: {
						Pod: &v2.PodRuntimeConfig{
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
							Metadata: &v2.MetadataConfig{
								Annotations: map[string]string{
									"some-pod-annotation": "pod-annotation-value",
								},
								Labels: map[string]string{
									"some-pod-label": "pod-label-value",
								},
							},
						},
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
							Image: "custom-elasticsearch",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"tracing": map[string]interface{}{
				"jaeger": map[string]interface{}{
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
				"useMCP":        true,
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
}

func TestRuntimeConversionFromV2(t *testing.T) {
	for _, tc := range runtimeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateControlPlaneRuntimeValues(specCopy.Runtime, helmValues.GetContent()); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			if !reflect.DeepEqual(tc.isolatedIstio.DeepCopy(), helmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.isolatedIstio.GetContent(), helmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			if _, err := populateControlPlaneRuntimeConfig(helmValues.DeepCopy(), specv2); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			assertEquals(t, tc.spec.Runtime, specv2.Runtime)
		})
	}
}
