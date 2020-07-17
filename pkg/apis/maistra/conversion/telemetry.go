package conversion

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func populateTelemetryValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	istiod := !(in.Version == "" || in.Version == versions.V1_0.String() || in.Version == versions.V1_1.String())

	telemetry := in.Telemetry
	if telemetry == nil {
		return nil
	}
	if telemetry.Type == v2.TelemetryTypeNone {
		if istiod {
			return setHelmBoolValue(values, "telemetry.enabled", false)
		}
		return setHelmBoolValue(values, "mixer.telemetry.enabled", false)
	}

	if telemetry.Type == "" {
		if istiod {
			telemetry.Type = v2.TelemetryTypeIstiod
		} else {
			telemetry.Type = v2.TelemetryTypeMixer
		}
	}
	switch telemetry.Type {
	case v2.TelemetryTypeMixer:
		return populateMixerTelemetryValues(in, istiod, values)
	case v2.TelemetryTypeRemote:
		return populateRemoteTelemetryValues(in, istiod, values)
	case v2.TelemetryTypeIstiod:
		return populateIstiodTelemetryValues(in, values)
	}

	if istiod {
		return setHelmBoolValue(values, "telemetry.enabled", false)
	}
	setHelmBoolValue(values, "mixer.telemetry.enabled", false)
	return fmt.Errorf("Unknown telemetry type: %s", telemetry.Type)
}

func populateMixerTelemetryValues(in *v2.ControlPlaneSpec, istiod bool, values map[string]interface{}) error {
	mixer := in.Telemetry.Mixer
	if mixer == nil {
		mixer = &v2.MixerTelemetryConfig{}
	}

	// Make sure mixer is enabled
	if err := setHelmBoolValue(values, "mixer.enabled", true); err != nil {
		return err
	}

	batchingValues := make(map[string]interface{})
	if err := populateTelemetryBatchingValues(&mixer.Batching, batchingValues); err != nil {
		return nil
	}

	v1TelemetryValues := make(map[string]interface{})
	if err := setHelmBoolValue(v1TelemetryValues, "enabled", true); err != nil {
		return err
	}

	if mixer.SessionAffinity != nil {
		if err := setHelmBoolValue(v1TelemetryValues, "sessionAffinityEnabled", *mixer.SessionAffinity); err != nil {
			return err
		}
	}

	if err := populateTelemetryBatchingValues(&mixer.Batching, v1TelemetryValues); err != nil {
		return nil
	}

	if mixer.Adapters != nil {
		adaptersValues := make(map[string]interface{})
		if mixer.Adapters.UseAdapterCRDs != nil {
			if err := setHelmBoolValue(adaptersValues, "useAdapterCRDs", *mixer.Adapters.UseAdapterCRDs); err != nil {
				return err
			}
		}
		if mixer.Adapters.KubernetesEnv != nil {
			if err := setHelmBoolValue(adaptersValues, "kubernetesenv.enabled", *mixer.Adapters.KubernetesEnv); err != nil {
				return err
			}
		}
		if mixer.Adapters.Stdio == nil {
			if err := setHelmBoolValue(adaptersValues, "stdio.enabled", false); err != nil {
				return err
			}
		} else {
			if err := setHelmBoolValue(adaptersValues, "stdio.enabled", true); err != nil {
				return err
			}
			if err := setHelmBoolValue(adaptersValues, "stdio.outputAsJson", mixer.Adapters.Stdio.OutputAsJSON); err != nil {
				return err
			}
		}
		if mixer.Adapters.Prometheus == nil {
			if err := setHelmBoolValue(adaptersValues, "prometheus.enabled", false); err != nil {
				return err
			}
		} else {
			if err := setHelmBoolValue(adaptersValues, "prometheus.enabled", true); err != nil {
				return err
			}
			if mixer.Adapters.Prometheus.MetricsExpiryDuration != "" {
				if err := setHelmStringValue(adaptersValues, "prometheus.metricsExpiryDuration", mixer.Adapters.Prometheus.MetricsExpiryDuration); err != nil {
					return err
				}
			}
		}
		if mixer.Adapters.Stackdriver == nil {
			if err := setHelmBoolValue(adaptersValues, "stackdriver.enabled", false); err != nil {
				return err
			}
		} else {
			stackdriver := mixer.Adapters.Stackdriver
			if err := setHelmBoolValue(adaptersValues, "stackdriver.enabled", true); err != nil {
				return err
			}
			if err := setHelmBoolValue(adaptersValues, "stackdriver.contextGraph.enabled", stackdriver.EnableContextGraph); err != nil {
				return err
			}
			if err := setHelmBoolValue(adaptersValues, "stackdriver.logging.enabled", stackdriver.EnableLogging); err != nil {
				return err
			}
			if err := setHelmBoolValue(adaptersValues, "stackdriver.metrics.enabled", stackdriver.EnableMetrics); err != nil {
				return err
			}
			if stackdriver.Auth != nil {
				auth := stackdriver.Auth
				if err := setHelmBoolValue(adaptersValues, "stackdriver.auth.appCredentials", auth.AppCredentials); err != nil {
					return err
				}
				if err := setHelmStringValue(adaptersValues, "stackdriver.auth.apiKey", auth.APIKey); err != nil {
					return err
				}
				if err := setHelmStringValue(adaptersValues, "stackdriver.auth.serviceAccountPath", auth.ServiceAccountPath); err != nil {
					return err
				}
			}
			if stackdriver.Tracer != nil {
				tracer := mixer.Adapters.Stackdriver.Tracer
				if err := setHelmIntValue(adaptersValues, "stackdriver.tracer.sampleProbability", int64(tracer.SampleProbability)); err != nil {
					return err
				}
			}
		}
		if len(adaptersValues) > 0 {
			if err := setHelmValue(values, "mixer.adapters", adaptersValues); err != nil {
				return err
			}
		}
	}

	// Deployment specific settings
	if mixer.Runtime != nil {
		runtime := mixer.Runtime
		if err := populateRuntimeValues(runtime, v1TelemetryValues); err != nil {
			return err
		}

		// set image and resources
		if runtime.Pod.Containers != nil {
			// Mixer container specific config
			if mixerContainer, ok := runtime.Pod.Containers["mixer"]; ok {
				if mixerContainer.Image != "" {
					if istiod {
						if err := setHelmStringValue(v1TelemetryValues, "image", mixerContainer.Image); err != nil {
							return err
						}
					} else {
						// XXX: this applies to both policy and telemetry in pre 1.6
						if err := setHelmStringValue(values, "mixer.image", mixerContainer.Image); err != nil {
							return err
						}
					}
				}
				if mixerContainer.Resources != nil {
					if resourcesValues, err := toValues(mixerContainer.Resources); err == nil {
						if len(resourcesValues) > 0 {
							if err := setHelmValue(v1TelemetryValues, "resources", resourcesValues); err != nil {
								return err
							}
						}
					} else {
						return err
					}
				}
			}
		}
	}

	if !istiod {
		// move podAnnotations, nodeSelector, podAntiAffinityLabelSelector, and
		// podAntiAffinityTermLabelSelector from mixer.telemetry to mixer for v1.0 and v1.1
		// Note, these may overwrite settings specified in policy
		if podAnnotations, found, _ := unstructured.NestedFieldCopy(v1TelemetryValues, "podAnnotations"); found {
			if err := setHelmValue(values, "mixer.podAnnotations", podAnnotations); err != nil {
				return err
			}
		}
		if nodeSelector, found, _ := unstructured.NestedFieldCopy(v1TelemetryValues, "nodeSelector"); found {
			if err := setHelmValue(values, "mixer.nodeSelector", nodeSelector); err != nil {
				return err
			}
		}
		if podAntiAffinityLabelSelector, found, _ := unstructured.NestedFieldCopy(v1TelemetryValues, "podAntiAffinityLabelSelector"); found {
			if err := setHelmValue(values, "mixer.podAntiAffinityLabelSelector", podAntiAffinityLabelSelector); err != nil {
				return err
			}
		}
		if podAntiAffinityTermLabelSelector, found, _ := unstructured.NestedFieldCopy(v1TelemetryValues, "podAntiAffinityTermLabelSelector"); found {
			if err := setHelmValue(values, "mixer.podAntiAffinityTermLabelSelector", podAntiAffinityTermLabelSelector); err != nil {
				return err
			}
		}
	}

	// set the telemetry values
	if istiod {
		v2TelemetryValues := make(map[string]interface{})
		if err := setHelmBoolValue(v2TelemetryValues, "enabled", true); err != nil {
			return err
		}
		if err := setHelmBoolValue(v2TelemetryValues, "v1.enabled", true); err != nil {
			return err
		}
		if err := setHelmBoolValue(v2TelemetryValues, "v2.enabled", false); err != nil {
			return err
		}

		if err := setHelmValue(values, "telemetry", v2TelemetryValues); err != nil {
			return err
		}
		if len(v1TelemetryValues) > 0 {
			if err := setHelmValue(values, "mixer.telemetry", v1TelemetryValues); err != nil {
				return err
			}
		}
	} else {
		if len(v1TelemetryValues) > 0 {
			if err := setHelmValue(values, "mixer.telemetry", v1TelemetryValues); err != nil {
				return err
			}
		}
	}

	return nil
}

func populateTelemetryBatchingValues(in *v2.TelemetryBatchingConfig, values map[string]interface{}) error {
	if in.MaxTime != "" {
		if err := setHelmStringValue(values, "mixer.telemetry.reportBatchMaxTime", in.MaxTime); err != nil {
			return err
		}
	}
	if in.MaxEntries != nil {
		return setHelmIntValue(values, "mixer.telemetry.reportBatchMaxEntries", int64(*in.MaxEntries))
	}
	return nil
}

func populateRemoteTelemetryValues(in *v2.ControlPlaneSpec, istiod bool, values map[string]interface{}) error {
	remote := in.Telemetry.Remote
	if remote == nil {
		remote = &v2.RemoteTelemetryConfig{}
	}

	// Make sure mixer is disabled
	if err := setHelmBoolValue(values, "mixer.enabled", false); err != nil {
		return err
	}

	if err := setHelmStringValue(values, "global.remoteTelemetryAddress", remote.Address); err != nil {
		return err
	}
	// XXX: this applies to both policy and telemetry
	if err := setHelmBoolValue(values, "global.createRemoteSvcEndpoints", remote.CreateService); err != nil {
		return err
	}

	v1TelemetryValues := make(map[string]interface{})
	if err := setHelmBoolValue(v1TelemetryValues, "enabled", true); err != nil {
		return err
	}

	if err := populateTelemetryBatchingValues(&remote.Batching, v1TelemetryValues); err != nil {
		return nil
	}

	// set the telemetry values
	if istiod {
		v2TelemetryValues := make(map[string]interface{})
		if err := setHelmBoolValue(v2TelemetryValues, "enabled", true); err != nil {
			return err
		}
		if err := setHelmBoolValue(v2TelemetryValues, "v1.enabled", true); err != nil {
			return err
		}
		if err := setHelmBoolValue(v2TelemetryValues, "v2.enabled", false); err != nil {
			return err
		}

		if err := setHelmValue(values, "telemetry", v2TelemetryValues); err != nil {
			return err
		}
		if len(v1TelemetryValues) > 0 {
			if err := setHelmValue(values, "mixer.telemetry", v1TelemetryValues); err != nil {
				return err
			}
		}
	} else {
		if len(v1TelemetryValues) > 0 {
			if err := setHelmValue(values, "mixer.telemetry", v1TelemetryValues); err != nil {
				return err
			}
		}
	}

	return nil
}

func populateIstiodTelemetryValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	istiod := in.Telemetry.Istiod
	if istiod == nil {
		istiod = &v2.IstiodTelemetryConfig{}
	}

	// Make sure mixer is disabled
	if err := setHelmBoolValue(values, "mixer.enabled", false); err != nil {
		return err
	}

	telemetryValues := make(map[string]interface{})
	if err := setHelmBoolValue(telemetryValues, "v1.enabled", false); err != nil {
		return err
	}
	if err := setHelmBoolValue(telemetryValues, "v2.enabled", true); err != nil {
		return err
	}

	// Adapters
	if istiod.MetadataExchange != nil {
		me := istiod.MetadataExchange
		if err := setHelmBoolValue(telemetryValues, "v2.metadataExchange.wasmEnabled", me.WASMEnabled); err != nil {
			return err
		}
	}

	if istiod.PrometheusFilter != nil {
		prometheus := istiod.PrometheusFilter
		if err := setHelmBoolValue(telemetryValues, "v2.prometheus.enabled", true); err != nil {
			return err
		}
		if err := setHelmBoolValue(telemetryValues, "v2.prometheus.wasmEnabled", prometheus.WASMEnabled); err != nil {
			return err
		}
		if err := setHelmBoolValue(values, "meshConfig.enablePrometheusMerge", prometheus.Scrape); err != nil {
			return err
		}
	}

	if istiod.StackDriverFilter != nil {
		stackdriver := istiod.StackDriverFilter
		if err := setHelmBoolValue(telemetryValues, "v2.stackdriver.enabled", true); err != nil {
			return err
		}
		if err := setHelmBoolValue(telemetryValues, "v2.stackdriver.logging", stackdriver.Logging); err != nil {
			return err
		}
		if err := setHelmBoolValue(telemetryValues, "v2.stackdriver.monitoring", stackdriver.Monitoring); err != nil {
			return err
		}
		if err := setHelmBoolValue(telemetryValues, "v2.stackdriver.topology", stackdriver.Topology); err != nil {
			return err
		}
		if err := setHelmBoolValue(telemetryValues, "v2.stackdriver.disableOutbound", stackdriver.DisableOutbound); err != nil {
			return err
		}
		if err := setHelmStringMapValue(telemetryValues, "v2.stackdriver.configOverride", stackdriver.ConfigOverride); err != nil {
			return err
		}
	}

	if istiod.AccessLogTelemetryFilter != nil {
		accessLog := istiod.AccessLogTelemetryFilter
		if err := setHelmBoolValue(telemetryValues, "v2.accessLogPolicy.enabled", true); err != nil {
			return err
		}
		if err := setHelmStringValue(telemetryValues, "v2.accessLogPolicy.logWindowDuration", accessLog.LogWindoDuration); err != nil {
			return err
		}
	}

	// set the telemetry values
	if len(telemetryValues) > 0 {
		if err := setHelmValue(values, "telemetry", telemetryValues); err != nil {
			return err
		}
	}

	return nil
}
