package conversion

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
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
				if err := setHelmBoolValue(values, "global.defaultPodDisruptionBudget.enabled", true); err != nil {
					return err
				}
				if deployment.Disruption.MinAvailable != nil {
					if err := setHelmStringValue(values, "global.defaultPodDisruptionBudget.minAvailable", deployment.Disruption.MinAvailable.String()); err != nil {
						return err
					}
				}
				if deployment.Disruption.MaxUnavailable != nil {
					if err := setHelmStringValue(values, "global.defaultPodDisruptionBudget.maxUnavailable", deployment.Disruption.MaxUnavailable.String()); err != nil {
						return err
					}
				}
			}
		}
		pod := runtime.Defaults.Pod
		if pod != nil {
			if len(pod.NodeSelector) > 0 {
				if err := setHelmStringMapValue(values, "global.defaultNodeSelector", pod.NodeSelector); err != nil {
					return err
				}
			}
			if len(pod.Tolerations) > 0 {
				untypedSlice := make([]interface{}, len(pod.Tolerations))
				for index, toleration := range pod.Tolerations {
					untypedSlice[index] = toleration
				}
				if tolerations, err := sliceToValues(untypedSlice); err == nil {
					if len(tolerations) > 0 {
						if err := setHelmValue(values, "global.defaultTolerations", tolerations); err != nil {
							return err
						}
					}
				} else {
					return err
				}
			}
			if pod.PriorityClassName != "" {
				if err := setHelmStringValue(values, "global.priorityClassName", pod.PriorityClassName); err != nil {
					return err
				}
			}
		}
		container := runtime.Defaults.Container
		if container != nil {
			if container.ImagePullPolicy != "" {
				if err := setHelmStringValue(values, "global.imagePullPolicy", string(container.ImagePullPolicy)); err != nil {
					return err
				}
			}
			if len(container.ImagePullSecrets) > 0 {
				pullSecretsValues := make([]string, 0)
				for _, secret := range container.ImagePullSecrets {
					pullSecretsValues = append(pullSecretsValues, secret.Name)
				}
				if err := setHelmStringSliceValue(values, "global.imagePullSecrets", pullSecretsValues); err != nil {
					return err
				}
			}
			if container.ImageRegistry != "" {
				if err := setHelmStringValue(values, "global.hub", container.ImageRegistry); err != nil {
					return err
				}
			}
			if container.ImageTag != "" {
				if err := setHelmStringValue(values, "global.tag", container.ImageTag); err != nil {
					return err
				}
			}
			if container.Resources != nil {
				if resourcesValues, err := toValues(container.Resources); err == nil {
					if len(resourcesValues) > 0 {
						if err := setHelmValue(values, "global.defaultResources", resourcesValues); err != nil {
							return err
						}
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
		if err := setHelmIntValue(values, "replicaCount", 1); err != nil {
			return err
		}
	} else {
		if err := setHelmIntValue(values, "replicaCount", int64(*deployment.Replicas)); err != nil {
			return err
		}
	}
	// labels are populated from Service.Metadata.Labels
	if deployment.Strategy != nil && deployment.Strategy.RollingUpdate != nil {
		if deployment.Strategy.RollingUpdate.MaxSurge != nil {
			if err := setHelmStringValue(values, "rollingMaxSurge", deployment.Strategy.RollingUpdate.MaxSurge.String()); err != nil {
				return err
			}
		}
		if deployment.Strategy.RollingUpdate.MaxUnavailable != nil {
			if err := setHelmStringValue(values, "rollingMaxUnavailable", deployment.Strategy.RollingUpdate.MaxUnavailable.String()); err != nil {
				return err
			}
		}
	}
	return nil
}

func populatePodHelmValues(pod *v2.PodRuntimeConfig, values map[string]interface{}) error {
	if len(pod.Metadata.Annotations) > 0 {
		if err := setHelmStringMapValue(values, "podAnnotations", pod.Metadata.Annotations); err != nil {
			return err
		}
	}
	if pod.PriorityClassName != "" {
		// XXX: this is only available with global.priorityClassName
		if err := setHelmStringValue(values, "priorityClassName", pod.PriorityClassName); err != nil {
			return err
		}
	}

	// Scheduling
	if len(pod.NodeSelector) > 0 {
		if err := setHelmStringMapValue(values, "nodeSelector", pod.NodeSelector); err != nil {
			return err
		}
	}
	if pod.Affinity != nil {
		// NodeAffinity is not supported, only NodeSelector may be used.
		// PodAffinity is not supported.
		if len(pod.Affinity.PodAntiAffinity.RequiredDuringScheduling) > 0 {
			podAntiAffinityLabelSelector := convertAntiAffinityTermsToHelmValues(pod.Affinity.PodAntiAffinity.RequiredDuringScheduling)
			if len(podAntiAffinityLabelSelector) > 0 {
				if err := setHelmValue(values, "podAntiAffinityLabelSelector", podAntiAffinityLabelSelector); err != nil {
					return err
				}
			}
		}
		if len(pod.Affinity.PodAntiAffinity.PreferredDuringScheduling) > 0 {
			podAntiAffinityTermLabelSelector := convertAntiAffinityTermsToHelmValues(pod.Affinity.PodAntiAffinity.PreferredDuringScheduling)
			if len(podAntiAffinityTermLabelSelector) > 0 {
				if err := setHelmValue(values, "podAntiAffinityTermLabelSelector", podAntiAffinityTermLabelSelector); err != nil {
					return err
				}
			}
		}
	}
	if len(pod.Tolerations) > 0 {
		untypedSlice := make([]interface{}, len(pod.Tolerations))
		for index, toleration := range pod.Tolerations {
			untypedSlice[index] = toleration
		}
		if tolerations, err := sliceToValues(untypedSlice); err == nil {
			if len(tolerations) > 0 {
				if err := setHelmValue(values, "tolerations", tolerations); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	return nil
}

func convertAntiAffinityTermsToHelmValues(terms []v2.PodAntiAffinityTerm) []interface{} {
	termsValues := make([]interface{}, 0)
	for _, term := range terms {
		termsValues = append(termsValues, map[string]interface{}{
			"key":         term.Key,
			"operator":    string(term.Operator),
			"values":      strings.Join(term.Values, ","),
			"topologyKey": term.TopologyKey,
		})
	}
	return termsValues
}

func populateAutoscalingHelmValues(autoScalerConfg *v2.AutoScalerConfig, values map[string]interface{}) error {
	if autoScalerConfg == nil {
		if err := setHelmBoolValue(values, "autoscaleEnabled", false); err != nil {
			return err
		}
	} else {
		if err := setHelmBoolValue(values, "autoscaleEnabled", true); err != nil {
			return err
		}
		if autoScalerConfg.MinReplicas != nil {
			if err := setHelmIntValue(values, "autoscaleMin", int64(*autoScalerConfg.MinReplicas)); err != nil {
				return err
			}
		}
		if autoScalerConfg.MaxReplicas != nil {
			if err := setHelmIntValue(values, "autoscaleMax", int64(*autoScalerConfg.MaxReplicas)); err != nil {
				return err
			}
		}
		if autoScalerConfg.TargetCPUUtilizationPercentage != nil {
			if err := setHelmIntValue(values, "cpu.targetAverageUtilization", int64(*autoScalerConfg.TargetCPUUtilizationPercentage)); err != nil {
				return err
			}
		}
	}
	return nil
}

func runtimeValuesToComponentRuntimeConfig(in *v1.HelmValues, out *v2.ComponentRuntimeConfig) error {
	if err := runtimeValuesToDeploymentRuntimeConfig(in, &out.Deployment); err != nil {
		return err
	}
	if err := runtimeValuesToPodRuntimeConfig(in, &out.Pod); err != nil {
		return err
	}
	if err := runtimeValuesToAutoscalingConfig(in, &out.Deployment); err != nil {
		return err
	}
	return nil
}

func runtimeValuesToDeploymentRuntimeConfig(in *v1.HelmValues, out *v2.DeploymentRuntimeConfig) error {
	if replicaCount, ok, err := in.GetInt64("replicaCount"); ok {
		replicas := int32(replicaCount)
		out.Replicas = &replicas
	} else if err != nil {
		return err
	}
	rollingUpdate := &appsv1.RollingUpdateDeployment{}
	setRollingUpdate := false
	if rollingMaxSurgeString, ok, err := in.GetString("rollingMaxSurge"); ok {
		maxSurge := intstr.FromString(rollingMaxSurgeString)
		rollingUpdate.MaxSurge = &maxSurge
		setRollingUpdate = true
	} else if err != nil {
		if rollingMaxSurgeInt, ok, err := in.GetInt64("rollingMaxSurge"); ok {
			maxSurge := intstr.FromInt(int(rollingMaxSurgeInt))
			rollingUpdate.MaxSurge = &maxSurge
			setRollingUpdate = true
		} else if err != nil {
			return err
		}
	}
	if rollingMaxUnavailableString, ok, err := in.GetString("rollingMaxUnavailable"); ok {
		maxUnavailable := intstr.FromString(rollingMaxUnavailableString)
		rollingUpdate.MaxUnavailable = &maxUnavailable
		setRollingUpdate = true
	} else if err != nil {
		if rollingMaxUnavailableInt, ok, err := in.GetInt64("rollingMaxUnavailable"); ok {
			maxUnavailable := intstr.FromInt(int(rollingMaxUnavailableInt))
			rollingUpdate.MaxUnavailable = &maxUnavailable
			setRollingUpdate = true
		} else if err != nil {
			return err
		}
	}
	if setRollingUpdate {
		out.Strategy = &appsv1.DeploymentStrategy{
			RollingUpdate: rollingUpdate,
		}
	}
	return nil
}

func runtimeValuesToPodRuntimeConfig(in *v1.HelmValues, out *v2.PodRuntimeConfig) error {
	if rawAnnotations, ok, err := in.GetMap("podAnnotations"); ok {
		out.Metadata.Annotations = make(map[string]string)
		for key, value := range rawAnnotations {
			if stringValue, ok := value.(string); ok {
				out.Metadata.Annotations[key] = stringValue
			} else {
				return fmt.Errorf("non string value in podAnnotations definition")
			}
		}
	} else if err != nil {
		return err
	}
	if priorityClassName, ok, err := in.GetString("priorityClassName"); ok {
		out.PriorityClassName = priorityClassName
	} else if err != nil {
		return err
	}

	// Scheduling
	if priorityClassName, ok, err := in.GetString("priorityClassName"); ok {
		out.PriorityClassName = priorityClassName
	} else if err != nil {
		return err
	}
	if rawSelector, ok, err := in.GetMap("nodeSelector"); ok {
		out.NodeSelector = make(map[string]string)
		for key, value := range rawSelector {
			if stringValue, ok := value.(string); ok {
				out.NodeSelector[key] = stringValue
			} else {
				return fmt.Errorf("non string value in nodeSelector definition")
			}
		}
	} else if err != nil {
		return err
	}
	if rawAntiAffinityLabelSelector, ok, err := in.GetSlice("podAntiAffinityLabelSelector"); ok {
		if out.Affinity == nil {
			out.Affinity = &v2.Affinity{}
		}
		for _, rawSelector := range rawAntiAffinityLabelSelector {
			if selectorValues, ok := rawSelector.(map[string]interface{}); ok {
				term := v2.PodAntiAffinityTerm{}
				if err := affinityTermValuesToAntiAffinityTerm(v1.NewHelmValues(selectorValues), &term); err != nil {
					return err
				}
				out.Affinity.PodAntiAffinity.RequiredDuringScheduling = append(out.Affinity.PodAntiAffinity.RequiredDuringScheduling, term)
			} else {
				return fmt.Errorf("could not cast podAntiAffinityLabelSelector value to map[string]interface{}")
			}
		}
	} else if err != nil {
		return err
	}
	if rawAntiAffinityLabelSelector, ok, err := in.GetSlice("podAntiAffinityTermLabelSelector"); ok {
		if out.Affinity == nil {
			out.Affinity = &v2.Affinity{}
		}
		for _, rawSelector := range rawAntiAffinityLabelSelector {
			if selectorValues, ok := rawSelector.(map[string]interface{}); ok {
				term := v2.PodAntiAffinityTerm{}
				if err := affinityTermValuesToAntiAffinityTerm(v1.NewHelmValues(selectorValues), &term); err != nil {
					return err
				}
				out.Affinity.PodAntiAffinity.RequiredDuringScheduling = append(out.Affinity.PodAntiAffinity.RequiredDuringScheduling, term)
			} else {
				return fmt.Errorf("could not cast podAntiAffinityTermLabelSelector value to map[string]interface{}")
			}
		}
	} else if err != nil {
		return err
	}
	if tolerationsValues, ok, err := in.GetSlice("tolerations"); ok {
		for _, tolerationValues := range tolerationsValues {
			toleration := corev1.Toleration{}
			if err := fromValues(tolerationValues, &toleration); err != nil {
				return err
			}
			out.Tolerations = append(out.Tolerations, toleration)
		}
	} else if err != nil {
		return err
	}
	return nil
}

func affinityTermValuesToAntiAffinityTerm(in *v1.HelmValues, out *v2.PodAntiAffinityTerm) error {
	term := v2.PodAntiAffinityTerm{}
	if key, ok, err := in.GetString("key"); ok {
		term.Key = key
	} else if err != nil {
		return err
	}
	if operator, ok, err := in.GetString("operator"); ok {
		term.Operator = metav1.LabelSelectorOperator(operator)
	} else if err != nil {
		return err
	}
	if topologyKey, ok, err := in.GetString("topologyKey"); ok {
		term.TopologyKey = topologyKey
	} else if err != nil {
		return err
	}
	if values, ok, err := in.GetString("values"); ok {
		term.Values = strings.Split(values, ",")
	} else if err != nil {
		return err
	}
	return nil
}

func runtimeValuesToAutoscalingConfig(in *v1.HelmValues, out *v2.DeploymentRuntimeConfig) error {
	if enabled, ok, err := in.GetBool("autoscaleEnabled"); ok {
		if !enabled {
			return nil
		}
	} else if err != nil {
		return err
	}
	autoScaling := v2.AutoScalerConfig{}
	if minReplicas64, ok, err := in.GetInt64("autoscaleMin"); ok {
		minReplicas := int32(minReplicas64)
		autoScaling.MinReplicas = &minReplicas
	} else if err != nil {
		return err
	}
	if maxReplicas64, ok, err := in.GetInt64("autoscaleMax"); ok {
		maxReplicas := int32(maxReplicas64)
		autoScaling.MaxReplicas = &maxReplicas
	} else if err != nil {
		return err
	}
	if cpuUtilization64, ok, err := in.GetInt64("cpu.targetAverageUtilization"); ok {
		cpuUtilization := int32(cpuUtilization64)
		autoScaling.TargetCPUUtilizationPercentage = &cpuUtilization
	} else if err != nil {
		return err
	}
	return nil
}
