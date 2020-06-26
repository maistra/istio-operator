package conversion

import (
	"fmt"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func populateTelemetryValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	istiod := !(in.Version == "" || in.Version == versions.V1_0.String() || in.Version == versions.V1_1.String())

	telemetry := in.Telemetry
	if telemetry == nil {
		if istiod {
			return setHelmValue(values, "telemetry.enabled", false)
		}
		return setHelmValue(values, "mixer.telemetry.enabled", false)
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
		return setHelmValue(values, "telemetry.enabled", false)
	}
	setHelmValue(values, "mixer.telemetry.enabled", false)
	return fmt.Errorf("Unknown telemetry type: %s", telemetry.Type)
}

func populateMixerTelemetryValues(in *v2.ControlPlaneSpec, istiod bool, values map[string]interface{}) error {
	mixer := in.Telemetry.Mixer
	if mixer == nil {
		mixer = &v2.MixerTelemetryConfig{}
	}

	// Make sure mixer is enabled
	if err := setHelmValue(values, "mixer.enabled", true); err != nil {
		return err
	}

	batchingValues := make(map[string]interface{})
	if err := populateTelemetryBatchingValues(&mixer.Batching, batchingValues); err != nil {
		return nil
	}

	v1TelemetryValues := make(map[string]interface{})
	if err := setHelmValue(v1TelemetryValues, "enabled", true); err != nil {
		return err
	}

	if err := setHelmValue(v1TelemetryValues, "sessionAffinityEnabled", mixer.SessionAffinity); err != nil {
		return err
	}

	if err := populateTelemetryBatchingValues(&mixer.Batching, v1TelemetryValues); err != nil {
		return nil
	}

	if mixer.Adapters != nil {
		adaptersValues := make(map[string]interface{})
		if err := setHelmValue(adaptersValues, "useAdapterCRDs", mixer.Adapters.UseAdapterCRDs); err != nil {
			return err
		}
		if err := setHelmValue(adaptersValues, "kubernetesenv.enabled", mixer.Adapters.KubernetesEnv); err != nil {
			return err
		}
		if mixer.Adapters.Stdio == nil {
			if err := setHelmValue(adaptersValues, "stdio.enabled", false); err != nil {
				return err
			}
		} else {
			if err := setHelmValue(adaptersValues, "stdio.enabled", true); err != nil {
				return err
			}
			if err := setHelmValue(adaptersValues, "stdio.outputAsJson", mixer.Adapters.Stdio.OutputAsJSON); err != nil {
				return err
			}
		}
		if mixer.Adapters.Prometheus == nil {
			if err := setHelmValue(adaptersValues, "prometheus.enabled", false); err != nil {
				return err
			}
		} else {
			if err := setHelmValue(adaptersValues, "prometheus.enabled", true); err != nil {
				return err
			}
			if mixer.Adapters.Prometheus.MetricsExpiryDuration != "" {
				if err := setHelmValue(adaptersValues, "prometheus.metricsExpiryDuration", mixer.Adapters.Prometheus.MetricsExpiryDuration); err != nil {
					return err
				}
			}
		}
		if mixer.Adapters.Stackdriver == nil {
			if err := setHelmValue(adaptersValues, "stackdriver.enabled", false); err != nil {
				return err
			}
		} else {
			stackdriver := mixer.Adapters.Stackdriver
			if err := setHelmValue(adaptersValues, "stackdriver.enabled", true); err != nil {
				return err
			}
			if err := setHelmValue(adaptersValues, "stackdriver.contextGraph.enabled", stackdriver.EnableContextGraph); err != nil {
				return err
			}
			if err := setHelmValue(adaptersValues, "stackdriver.logging.enabled", stackdriver.EnableLogging); err != nil {
				return err
			}
			if err := setHelmValue(adaptersValues, "stackdriver.metrics.enabled", stackdriver.EnableMetrics); err != nil {
				return err
			}
			if stackdriver.Auth != nil {
				auth := stackdriver.Auth
				if err := setHelmValue(adaptersValues, "stackdriver.auth.appCredentials", auth.AppCredentials); err != nil {
					return err
				}
				if err := setHelmValue(adaptersValues, "stackdriver.auth.apiKey", auth.APIKey); err != nil {
					return err
				}
				if err := setHelmValue(adaptersValues, "stackdriver.auth.serviceAccountPath", auth.ServiceAccountPath); err != nil {
					return err
				}
			}
			if stackdriver.Tracer != nil {
				tracer := mixer.Adapters.Stackdriver.Tracer
				if err := setHelmValue(adaptersValues, "stackdriver.tracer.sampleProbability", tracer.SampleProbability); err != nil {
					return err
				}
			}
		}
		if err := setHelmValue(values, "mixer.adapters", adaptersValues); err != nil {
			return err
		}
	}

	// Deployment specific settings
	runtime := mixer.Runtime
	if runtime == nil {
		runtime = &v2.ComponentRuntimeConfig{}
	}
	if err := populateRuntimeValues(runtime, v1TelemetryValues); err != nil {
		return err
	}

	// set image and resources
	if runtime.Pod.Containers != nil {
		// Mixer container specific config
		if mixerContainer, ok := runtime.Pod.Containers["mixer"]; ok {
			if mixerContainer.Image != "" {
				if istiod {
					if err := setHelmValue(v1TelemetryValues, "image", mixerContainer.Image); err != nil {
						return err
					}
				} else {
					// XXX: this applies to both policy and telemetry in pre 1.6
					if err := setHelmValue(values, "mixer.image", mixerContainer.Image); err != nil {
						return err
					}
				}
			}
			if mixerContainer.Resources != nil {
				if resourcesValues, err := toValues(mixerContainer.Resources); err == nil {
					if err := setHelmValue(v1TelemetryValues, "resources", resourcesValues); err != nil {
						return err
					}
				} else {
					return err
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
		if err := setHelmValue(v2TelemetryValues, "enabled", true); err != nil {
			return err
		}
		if err := setHelmValue(v2TelemetryValues, "v1.enabled", true); err != nil {
			return err
		}
		if err := setHelmValue(v2TelemetryValues, "v2.enabled", false); err != nil {
			return err
		}

		if err := setHelmValue(values, "telemetry", v2TelemetryValues); err != nil {
			return err
		}
		if err := setHelmValue(values, "mixer.telemetry", v1TelemetryValues); err != nil {
			return err
		}
	} else {
		if err := setHelmValue(values, "mixer.telemetry", v1TelemetryValues); err != nil {
			return err
		}
	}

	return nil
}

func populateTelemetryBatchingValues(in *v2.TelemetryBatchingConfig, values map[string]interface{}) error {
	if in.MaxTime != "" {
		if err := setHelmValue(values, "mixer.telemetry.reportBatchMaxTime", in.MaxTime); err != nil {
			return err
		}
	}
	return setHelmValue(values, "mixer.telemetry.reportBatchMaxEntries", in.MaxEntries)
}

func populateRemoteTelemetryValues(in *v2.ControlPlaneSpec, istiod bool, values map[string]interface{}) error {
	remote := in.Telemetry.Remote
	if remote == nil {
		remote = &v2.RemoteTelemetryConfig{}
	}

	// Make sure mixer is disabled
	if err := setHelmValue(values, "mixer.enabled", false); err != nil {
		return err
	}

	if err := setHelmValue(values, "global.remoteTelemetryAddress", remote.Address); err != nil {
		return err
	}
	// XXX: this applies to both policy and telemetry
	if err := setHelmValue(values, "global.createRemoteSvcEndpoints", remote.CreateService); err != nil {
		return err
	}

	v1TelemetryValues := make(map[string]interface{})
	if err := setHelmValue(v1TelemetryValues, "enabled", true); err != nil {
		return err
	}

	if err := populateTelemetryBatchingValues(&remote.Batching, v1TelemetryValues); err != nil {
		return nil
	}

	// set the telemetry values
	if istiod {
		v2TelemetryValues := make(map[string]interface{})
		if err := setHelmValue(v2TelemetryValues, "enabled", true); err != nil {
			return err
		}
		if err := setHelmValue(v2TelemetryValues, "v1.enabled", true); err != nil {
			return err
		}
		if err := setHelmValue(v2TelemetryValues, "v2.enabled", false); err != nil {
			return err
		}

		if err := setHelmValue(values, "telemetry", v2TelemetryValues); err != nil {
			return err
		}
		if err := setHelmValue(values, "mixer.telemetry", v1TelemetryValues); err != nil {
			return err
		}
	} else {
		if err := setHelmValue(values, "mixer.telemetry", v1TelemetryValues); err != nil {
			return err
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
	if err := setHelmValue(values, "mixer.enabled", false); err != nil {
		return err
	}

	telemetryValues := make(map[string]interface{})
	if err := setHelmValue(telemetryValues, "v1.enabled", false); err != nil {
		return err
	}
	if err := setHelmValue(telemetryValues, "v2.enabled", true); err != nil {
		return err
	}

	// Adapters
	if istiod.MetadataExchange != nil {
		me := istiod.MetadataExchange
		if err := setHelmValue(telemetryValues, "v2.metadataExchange.wasmEnabled", me.WASMEnabled); err != nil {
			return err
		}
	}

	if istiod.PrometheusFilter != nil {
		prometheus := istiod.PrometheusFilter
		if err := setHelmValue(telemetryValues, "v2.prometheus.enabled", true); err != nil {
			return err
		}
		if err := setHelmValue(telemetryValues, "v2.prometheus.wasmEnabled", prometheus.WASMEnabled); err != nil {
			return err
		}
		if err := setHelmValue(values, "meshConfig.enablePrometheusMerge", prometheus.Scrape); err != nil {
			return err
		}
	}

	if istiod.StackDriverFilter != nil {
		stackdriver := istiod.StackDriverFilter
		if err := setHelmValue(telemetryValues, "v2.stackdriver.enabled", true); err != nil {
			return err
		}
		if err := setHelmValue(telemetryValues, "v2.stackdriver.logging", stackdriver.Logging); err != nil {
			return err
		}
		if err := setHelmValue(telemetryValues, "v2.stackdriver.monitoring", stackdriver.Monitoring); err != nil {
			return err
		}
		if err := setHelmValue(telemetryValues, "v2.stackdriver.topology", stackdriver.Topology); err != nil {
			return err
		}
		if err := setHelmValue(telemetryValues, "v2.stackdriver.disableOutbound", stackdriver.DisableOutbound); err != nil {
			return err
		}
		if err := setHelmValue(telemetryValues, "v2.stackdriver.configOverride", stackdriver.ConfigOverride); err != nil {
			return err
		}
	}

	if istiod.AccessLogTelemetryFilter != nil {
		accessLog := istiod.AccessLogTelemetryFilter
		if err := setHelmValue(telemetryValues, "v2.accessLogPolicy.enabled", true); err != nil {
			return err
		}
		if err := setHelmValue(telemetryValues, "v2.accessLogPolicy.logWindowDuration", accessLog.LogWindoDuration); err != nil {
			return err
		}
	}

	// set the telemetry values
	if err := setHelmValue(values, "telemetry", telemetryValues); err != nil {
		return err
	}

	return nil
}
