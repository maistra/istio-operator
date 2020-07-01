package conversion

import (
	"strings"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateControlPlaneRuntimeValues(runtime *v2.ControlPlaneRuntimeConfig, values map[string]interface{}) error {
	if runtime == nil {
		return nil
	}

	if runtime.Defaults != nil {
		// defaultNodeSelector, defaultTolerations, defaultPodDisruptionBudget, priorityClassName
		deployment := runtime.Defaults.Deployment
		if deployment != nil {
			if deployment.Disruption != nil {
				if err := setHelmValue(values, "global.defaultPodDisruptionBudget.enabled", true); err != nil {
					return err
				}
				if deployment.Disruption.MinAvailable != nil {
					if err := setHelmValue(values, "global.defaultPodDisruptionBudget.minAvailable", deployment.Disruption.MinAvailable); err != nil {
						return err
					}
				}
				if deployment.Disruption.MaxUnavailable != nil {
					if err := setHelmValue(values, "global.defaultPodDisruptionBudget.maxUnavailable", deployment.Disruption.MaxUnavailable); err != nil {
						return err
					}
				}
			}
		}
		pod := runtime.Defaults.Pod
		if pod != nil {
			if len(pod.NodeSelector) > 0 {
				if err := setHelmValue(values, "global.defaultNodeSelector", pod.NodeSelector); err != nil {
					return err
				}
			}
			if len(pod.Tolerations) > 0 {
				if tolerations, err := toValues(pod.Tolerations); err == nil {
					if err := setHelmValue(values, "global.defaultTolerations", tolerations); err != nil {
						return err
					}
				} else {
					return err
				}
			}
			if pod.PriorityClassName != "" {
				if err := setHelmValue(values, "global.priorityClassName", pod.PriorityClassName); err != nil {
					return err
				}
			}
		}
		container := runtime.Defaults.Container
		if container != nil {
			if container.ImagePullPolicy != "" {
				if err := setHelmValue(values, "global.imagePullPolicy", container.ImagePullPolicy); err != nil {
					return err
				}
			}
			if len(container.ImagePullSecrets) > 0 {
				if err := setHelmValue(values, "global.imagePullSecrets", container.ImagePullSecrets); err != nil {
					return err
				}
			}
			if container.ImageRegistry != "" {
				if err := setHelmValue(values, "global.hub", container.ImageRegistry); err != nil {
					return err
				}
			}
			if container.ImageTag != "" {
				if err := setHelmValue(values, "global.tag", container.ImageTag); err != nil {
					return err
				}
			}
			if container.Resources != nil {
				if resourcesValues, err := toValues(container.Resources); err == nil {
					if err := setHelmValue(values, "global.defaultResources", resourcesValues); err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}
	}

	if runtime.Citadel != nil {
		citadelValues := make(map[string]interface{})
		if err := populateRuntimeValues(runtime.Citadel, citadelValues); err == nil {
			for key, value := range citadelValues {
				if err := setHelmValue(values, "security."+key, value); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	if runtime.Galley != nil {
		galleyValues := make(map[string]interface{})
		if err := populateRuntimeValues(runtime.Galley, galleyValues); err == nil {
			for key, value := range galleyValues {
				if err := setHelmValue(values, "galley."+key, value); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	if runtime.Pilot != nil {
		pilotValues := make(map[string]interface{})
		if err := populateRuntimeValues(runtime.Pilot, pilotValues); err == nil {
			for key, value := range pilotValues {
				if err := setHelmValue(values, "pilot."+key, value); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	return nil
}

func populateRuntimeValues(runtime *v2.ComponentRuntimeConfig, values map[string]interface{}) error {
	if runtime == nil {
		return nil
	}
	if err := populateDeploymentHelmValues(&runtime.Deployment, values); err != nil {
		return err
	}
	if err := populatePodHelmValues(&runtime.Pod, values); err != nil {
		return err
	}
	if err := populateAutoscalingHelmValues(runtime.Deployment.AutoScaling, values); err != nil {
		return err
	}

	return nil
}

func populateDeploymentHelmValues(deployment *v2.DeploymentRuntimeConfig, values map[string]interface{}) error {
	if deployment == nil {
		return nil
	}
	if deployment.Replicas == nil {
		if err := setHelmValue(values, "replicaCount", 1); err != nil {
			return err
		}
	} else {
		if err := setHelmValue(values, "replicaCount", *deployment.Replicas); err != nil {
			return err
		}
	}
	// labels are populated from Service.Metadata.Labels
	if deployment.Strategy != nil && deployment.Strategy.RollingUpdate != nil {
		if deployment.Strategy.RollingUpdate.MaxSurge != nil {
			if err := setHelmValue(values, "rollingMaxSurge", *deployment.Strategy.RollingUpdate.MaxSurge); err != nil {
				return err
			}
		}
		if deployment.Strategy.RollingUpdate.MaxUnavailable != nil {
			if err := setHelmValue(values, "rollingMaxUnavailable", *deployment.Strategy.RollingUpdate.MaxUnavailable); err != nil {
				return err
			}
		}
	}
	return nil
}

func populatePodHelmValues(pod *v2.PodRuntimeConfig, values map[string]interface{}) error {
	if len(pod.Metadata.Annotations) > 0 {
		if err := setHelmValue(values, "podAnnotations", pod.Metadata.Annotations); err != nil {
			return err
		}
	}
	if pod.PriorityClassName != "" {
		// XXX: this is only available with global.priorityClassName
		if err := setHelmValue(values, "priorityClassName", pod.PriorityClassName); err != nil {
			return err
		}
	}

	// Scheduling
	if len(pod.NodeSelector) > 0 {
		if err := setHelmValue(values, "nodeSelector", pod.NodeSelector); err != nil {
			return err
		}
	}
	if pod.Affinity != nil {
		// NodeAffinity is not supported, only NodeSelector may be used.
		// PodAffinity is not supported.
		if pod.Affinity.PodAntiAffinity != nil {
			if len(pod.Affinity.PodAntiAffinity.RequiredDuringScheduling) > 0 {
				podAntiAffinityLabelSelector := make([]map[string]string, 0)
				for _, term := range pod.Affinity.PodAntiAffinity.RequiredDuringScheduling {
					podAntiAffinityLabelSelector = append(podAntiAffinityLabelSelector, map[string]string{
						"key":         term.Key,
						"operator":    string(term.Operator),
						"values":      strings.Join(term.Values, ","),
						"topologyKey": term.TopologyKey,
					})
				}
				if err := setHelmValue(values, "podAntiAffinityLabelSelector", podAntiAffinityLabelSelector); err != nil {
					return err
				}
			}
			if len(pod.Affinity.PodAntiAffinity.PreferredDuringScheduling) > 0 {
				podAntiAffinityTermLabelSelector := make([]map[string]string, 0)
				for _, term := range pod.Affinity.PodAntiAffinity.PreferredDuringScheduling {
					podAntiAffinityTermLabelSelector = append(podAntiAffinityTermLabelSelector, map[string]string{
						"key":         term.Key,
						"operator":    string(term.Operator),
						"values":      strings.Join(term.Values, ","),
						"topologyKey": term.TopologyKey,
					})
				}
				if err := setHelmValue(values, "podAntiAffinityTermLabelSelector", podAntiAffinityTermLabelSelector); err != nil {
					return err
				}
			}
		}
	}
	if len(pod.Tolerations) > 0 {
		if tolerations, err := toValues(pod.Tolerations); err == nil {
			if err := setHelmValue(values, "tolerations", tolerations); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func populateAutoscalingHelmValues(autoScalerConfg *v2.AutoScalerConfig, values map[string]interface{}) error {
	if autoScalerConfg == nil {
		if err := setHelmValue(values, "autoscaleEnabled", false); err != nil {
			return err
		}
	} else {
		if err := setHelmValue(values, "autoscaleEnabled", true); err != nil {
			return err
		}
		if autoScalerConfg.MinReplicas != nil {
			if err := setHelmValue(values, "autoscaleMin", *autoScalerConfg.MinReplicas); err != nil {
				return err
			}
		}
		if autoScalerConfg.MaxReplicas != nil {
			if err := setHelmValue(values, "autoscaleMax", *autoScalerConfg.MaxReplicas); err != nil {
				return err
			}
		}
		if autoScalerConfg.TargetCPUUtilizationPercentage != nil {
			if err := setHelmValue(values, "cpu.targetAverageUtilization", *autoScalerConfg.TargetCPUUtilizationPercentage); err != nil {
				return err
			}
		}
	}
	return nil
}
