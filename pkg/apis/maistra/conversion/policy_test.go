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

var policyTestCases = []conversionTestCase{
	{
		name: "nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
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
			Policy:  &v2.PolicyConfig{},
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
		name: "defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy:  &v2.PolicyConfig{},
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
		name: "none." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeNone,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"policy": map[string]interface{}{
					"enabled": false,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "None",
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
		name: "none." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeNone,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"policy": map[string]interface{}{
					"enabled": false,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "None",
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
		name: "istiod.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeIstiod,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": false,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Istiod",
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
		name: "mixer.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type:  v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					EnableChecks: &featureEnabled,
					FailOpen:     &featureDisabled,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"disablePolicyChecks": false,
				"policyCheckFailOpen": false,
			},
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.adapters.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					Adapters: &v2.MixerPolicyAdaptersConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.adapters." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					Adapters: &v2.MixerPolicyAdaptersConfig{
						KubernetesEnv:  &featureEnabled,
						UseAdapterCRDs: &featureDisabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled": true,
					"adapters": map[string]interface{}{
						"kubernetesenv": map[string]interface{}{
							"enabled": true,
						},
						"useAdapterCRDs": false,
					},
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.runtime.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					Runtime: &v2.ComponentRuntimeConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled":          true,
					"autoscaleEnabled": false,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.runtime.basic." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
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
								"mixer": {
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
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled":               true,
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
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.runtime.autoscale." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
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
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled":          true,
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
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.nil." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type:  v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.misc." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					EnableChecks: &featureEnabled,
					FailOpen:     &featureDisabled,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"disablePolicyChecks": false,
				"policyCheckFailOpen": false,
			},
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.adapters.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					Adapters: &v2.MixerPolicyAdaptersConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.adapters." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					Adapters: &v2.MixerPolicyAdaptersConfig{
						KubernetesEnv:  &featureEnabled,
						UseAdapterCRDs: &featureDisabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"adapters": map[string]interface{}{
					"kubernetesenv": map[string]interface{}{
						"enabled": true,
					},
					"useAdapterCRDs": false,
				},
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.runtime.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					Runtime: &v2.ComponentRuntimeConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled":          true,
					"autoscaleEnabled": false,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.runtime.basic." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
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
								"mixer": {
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
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"image":   "custom-mixer",
				"nodeSelector": map[string]interface{}{
					"node-label": "node-value",
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
				"policy": map[string]interface{}{
					"enabled":               true,
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
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "mixer.runtime.autoscale." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
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
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"policy": map[string]interface{}{
					"enabled":          true,
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
			"policy": map[string]interface{}{
				"implementation": "Mixer",
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
		name: "remote.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remotePolicyAddress":      "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Remote",
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
		name: "remote.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type:   v2.PolicyTypeRemote,
				Remote: &v2.RemotePolicyConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remotePolicyAddress":      "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Remote",
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
		name: "remote.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
				Remote: &v2.RemotePolicyConfig{
					Address:       "mixer-policy.some-namespace.svc.cluster.local",
					CreateService: true,
					EnableChecks:  &featureEnabled,
					FailOpen:      &featureDisabled,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": true,
				"remotePolicyAddress":      "mixer-policy.some-namespace.svc.cluster.local",
				"disablePolicyChecks":      false,
				"policyCheckFailOpen":      false,
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Remote",
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
		name: "remote.nil." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remotePolicyAddress":      "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Remote",
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
		name: "remote.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type:   v2.PolicyTypeRemote,
				Remote: &v2.RemotePolicyConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remotePolicyAddress":      "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Remote",
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
		name: "remote.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
				Remote: &v2.RemotePolicyConfig{
					Address:       "mixer-policy.some-namespace.svc.cluster.local",
					CreateService: true,
					EnableChecks:  &featureEnabled,
					FailOpen:      &featureDisabled,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": true,
				"remotePolicyAddress":      "mixer-policy.some-namespace.svc.cluster.local",
				"disablePolicyChecks":      false,
				"policyCheckFailOpen":      false,
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
				"implementation": "Remote",
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

func TestPolicyConversionFromV2(t *testing.T) {
	for _, tc := range policyTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populatePolicyValues(specCopy, helmValues.GetContent()); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			if !reflect.DeepEqual(tc.isolatedIstio.DeepCopy(), helmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.isolatedIstio.GetContent(), helmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), helmValues.GetContent())
			if version, err := versions.ParseVersion(tc.spec.Version); err == nil {
				if err := populatePolicyConfig(helmValues.DeepCopy(), specv2, version); err != nil {
					t.Fatalf("error converting from values: %s", err)
				}
			} else {
				t.Fatalf("error parsing version: %s", err)
			}
			assertEquals(t, tc.spec.Policy, specv2.Policy)
		})
	}
}
