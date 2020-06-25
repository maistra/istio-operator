package conversion

import (
	"strings"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateRuntimeValues(runtime *v2.ComponentRuntimeConfig, values map[string]interface{}) error {
	if runtime == nil {
		runtime = &v2.ComponentRuntimeConfig{}
	}
	if err := populateAutoscalingHelmValues(runtime.Deployment.AutoScaling, values); err != nil {
		return err
	}
	if runtime.Deployment.Replicas == nil {
		if err := setHelmValue(values, "replicaCount", 1); err != nil {
			return err
		}
	} else {
		if err := setHelmValue(values, "replicaCount", *runtime.Deployment.Replicas); err != nil {
			return err
		}
	}
	// labels are populated from Service.Metadata.Labels
	if runtime.Deployment.Strategy != nil && runtime.Deployment.Strategy.RollingUpdate != nil {
		if runtime.Deployment.Strategy.RollingUpdate.MaxSurge != nil {
			if err := setHelmValue(values, "rollingMaxSurge", *runtime.Deployment.Strategy.RollingUpdate.MaxSurge); err != nil {
				return err
			}
		}
		if runtime.Deployment.Strategy.RollingUpdate.MaxUnavailable != nil {
			if err := setHelmValue(values, "rollingMaxUnavailable", *runtime.Deployment.Strategy.RollingUpdate.MaxUnavailable); err != nil {
				return err
			}
		}
	}
	if len(runtime.Pod.Metadata.Annotations) > 0 {
		if err := setHelmValue(values, "podAnnotations", runtime.Pod.Metadata.Annotations); err != nil {
			return err
		}
	}
	if runtime.Pod.PriorityClassName != "" {
		// XXX: this is only available with global.priorityClassName
		if err := setHelmValue(values, "priorityClassName", runtime.Pod.PriorityClassName); err != nil {
			return err
		}
	}

	// Scheduling
	if len(runtime.Pod.NodeSelector) > 0 {
		if err := setHelmValue(values, "nodeSelector", runtime.Pod.NodeSelector); err != nil {
			return err
		}
	}
	if runtime.Pod.Affinity != nil {
		// NodeAffinity is not supported, only NodeSelector may be used.
		// PodAffinity is not supported.
		if runtime.Pod.Affinity.PodAntiAffinity != nil {
			if len(runtime.Pod.Affinity.PodAntiAffinity.RequiredDuringScheduling) > 0 {
				podAntiAffinityLabelSelector := make([]map[string]string, 0)
				for _, term := range runtime.Pod.Affinity.PodAntiAffinity.RequiredDuringScheduling {
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
			if len(runtime.Pod.Affinity.PodAntiAffinity.PreferredDuringScheduling) > 0 {
				podAntiAffinityTermLabelSelector := make([]map[string]string, 0)
				for _, term := range runtime.Pod.Affinity.PodAntiAffinity.PreferredDuringScheduling {
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
	if len(runtime.Pod.Tolerations) > 0 {
		if tolerations, err := toValues(runtime.Pod.Tolerations); err == nil {
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
		if autoScalerConfg.MinReplicas == nil {
			if err := setHelmValue(values, "autoscaleMin", 1); err != nil {
				return err
			}
		} else {
			if err := setHelmValue(values, "autoscaleMin", *autoScalerConfg.MinReplicas); err != nil {
				return err
			}
		}
		if err := setHelmValue(values, "autoscaleMax", autoScalerConfg.MaxReplicas); err != nil {
			return err
		}
		if autoScalerConfg.TargetCPUUtilizationPercentage != nil {
			if err := setHelmValue(values, "cpu.targetAverageUtilization", *autoScalerConfg.TargetCPUUtilizationPercentage); err != nil {
				return err
			}
		}
	}
	return nil
}
