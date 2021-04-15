package conversion

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	batchMaxEntries100 = int32(100)
	maxAnnotations     = int64(100)
	maxEvents          = int64(200)
	maxAttributes      = int64(300)
)

var telemetryTestCases []conversionTestCase

var telemetryTestCasesV1 = []conversionTestCase{
	{
		name: "defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version:   versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "none." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeNone,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": false,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "None",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "mixer.nil." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "mixer.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type:  v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "mixer.misc." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					SessionAffinity: &featureEnabled,
					Batching: &v2.TelemetryBatchingConfig{
						MaxEntries: &batchMaxEntries100,
						MaxTime:    "5",
					},
					Loadshedding: &v2.TelemetryLoadSheddingConfig{
						Mode:             "enforce",
						LatencyThreshold: "100ms",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled":                true,
					"reportBatchMaxEntries":  int64(100),
					"reportBatchMaxTime":     "5",
					"sessionAffinityEnabled": true,
					"loadshedding": map[string]interface{}{
						"mode":             "enforce",
						"latencyThreshold": "100ms",
					},
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "mixer.adapters.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "mixer.adapters.misc." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						KubernetesEnv:  &featureEnabled,
						UseAdapterCRDs: &featureDisabled,
					},
				},
			},
		},
		roundTripSpec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						KubernetesEnv:  &featureEnabled,
						UseAdapterCRDs: &featureDisabled,
					},
				},
			},
			Policy: &v2.PolicyConfig{
				Mixer: &v2.MixerPolicyConfig{
					Adapters: &v2.MixerPolicyAdaptersConfig{
						KubernetesEnv:  &featureEnabled,
						UseAdapterCRDs: &featureDisabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"kubernetesenv": map[string]interface{}{
						"enabled": true,
					},
					"useAdapterCRDs": false,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "mixer.adapters.prometheus.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
			Addons: &v2.AddonsConfig{
				Prometheus: &v2.PrometheusAddonConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
			"mixer": map[string]interface{}{
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": true,
					},
				},
			},
			"prometheus": map[string]interface{}{"enabled": true},
			"telemetry": map[string]interface{}{
				"v2": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": true,
					},
				},
			},
		}),
	},
	{
		name: "mixer.adapters.prometheus.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
			Addons: &v2.AddonsConfig{
				Prometheus: &v2.PrometheusAddonConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
					MetricsExpiryDuration: "10m",
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
			"mixer": map[string]interface{}{
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled":               true,
						"metricsExpiryDuration": "10m",
					},
				},
			},
			"prometheus": map[string]interface{}{"enabled": true},
			"telemetry": map[string]interface{}{
				"v2": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": true,
					},
				},
			},
		}),
	},
	{
		name: "mixer.adapters.stackdriver.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
			Addons: &v2.AddonsConfig{
				Stackdriver: &v2.StackdriverAddonConfig{
					Telemetry: &v2.StackdriverTelemetryConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
			"mixer": map[string]interface{}{
				"adapters": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"enabled": true,
					},
				},
			},
			"telemetry": map[string]interface{}{
				"v2": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"enabled": true,
					},
				},
			},
		}),
	},
	{
		name: "mixer.adapters.stackdriver.misc." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
			Addons: &v2.AddonsConfig{
				Stackdriver: &v2.StackdriverAddonConfig{
					Telemetry: &v2.StackdriverTelemetryConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
						EnableLogging:      &featureEnabled,
						EnableMetrics:      &featureEnabled,
						EnableContextGraph: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
			"mixer": map[string]interface{}{
				"adapters": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"enabled":      true,
						"logging":      map[string]interface{}{"enabled": true},
						"metrics":      map[string]interface{}{"enabled": true},
						"contextGraph": map[string]interface{}{"enabled": true},
					},
				},
			},
			"telemetry": map[string]interface{}{
				"v2": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"enabled":    true,
						"logging":    true,
						"monitoring": true,
						"topology":   true,
					},
				},
			},
		}),
	},
	{
		name: "mixer.adapters.stackdriver.auth.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
			Addons: &v2.AddonsConfig{
				Stackdriver: &v2.StackdriverAddonConfig{
					Telemetry: &v2.StackdriverTelemetryConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
						Auth: &v2.StackdriverAuthConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
			"mixer": map[string]interface{}{
				"adapters": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"enabled": true,
					},
				},
			},
			"telemetry": map[string]interface{}{
				"v2": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"enabled": true,
					},
				},
			},
		}),
	},
	{
		name: "mixer.adapters.stackdriver.auth.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
			Addons: &v2.AddonsConfig{
				Stackdriver: &v2.StackdriverAddonConfig{
					Telemetry: &v2.StackdriverTelemetryConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
						Auth: &v2.StackdriverAuthConfig{
							AppCredentials:     &featureEnabled,
							APIKey:             "mykey",
							ServiceAccountPath: "/path/to/sa",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
			"mixer": map[string]interface{}{
				"adapters": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"auth": map[string]interface{}{
							"apiKey":             "mykey",
							"appCredentials":     true,
							"serviceAccountPath": "/path/to/sa",
						},
					},
				},
			},
			"telemetry": map[string]interface{}{
				"v2": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"enabled": true,
					},
				},
			},
		}),
	},
	{
		name: "mixer.adapters.stackdriver.tracer.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
			Tracing: &v2.TracingConfig{
				Type: v2.TracerTypeStackdriver,
			},
			Addons: &v2.AddonsConfig{
				Stackdriver: &v2.StackdriverAddonConfig{
					Tracer: &v2.StackdriverTracerConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
				"enableTracing": true,
				"proxy": map[string]interface{}{
					"tracer": "stackdriver",
				},
			},
			"mixer": map[string]interface{}{
				"adapters": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"tracer": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "stackdriver",
			},
		}),
	},
	{
		name: "mixer.adapters.stackdriver.tracer.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
			Tracing: &v2.TracingConfig{
				Type:     v2.TracerTypeStackdriver,
				Sampling: &traceSampling,
			},
			Addons: &v2.AddonsConfig{
				Stackdriver: &v2.StackdriverAddonConfig{
					Tracer: &v2.StackdriverTracerConfig{
						Debug:                    &featureEnabled,
						MaxNumberOfAnnotations:   &maxAnnotations,
						MaxNumberOfAttributes:    &maxAttributes,
						MaxNumberOfMessageEvents: &maxEvents,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
				"enableTracing": true,
				"proxy": map[string]interface{}{
					"tracer": "stackdriver",
				},
				"tracer": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"debug":                    true,
						"maxNumberOfAttributes":    maxAttributes,
						"maxNumberOfAnnotations":   maxAnnotations,
						"maxNumberOfMessageEvents": maxEvents,
					},
				},
			},
			"mixer": map[string]interface{}{
				"adapters": map[string]interface{}{
					"stackdriver": map[string]interface{}{
						"tracer": map[string]interface{}{
							"enabled":           true,
							"sampleProbability": .01,
						},
					},
				},
			},
			"pilot": map[string]interface{}{
				"traceSampling": 0.01,
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "stackdriver",
			},
		}),
	},
	{
		name: "mixer.adapters.stdio.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stdio: &v2.MixerTelemetryStdioConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"stdio": map[string]interface{}{
						"enabled": true,
					},
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "mixer.adapters.stdio.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stdio: &v2.MixerTelemetryStdioConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
							OutputAsJSON: &featureEnabled,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": false,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"stdio": map[string]interface{}{
						"enabled":      true,
						"outputAsJson": true,
					},
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Mixer",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "remote.nil." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeRemote,
			},
		},
		roundTripSpec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeRemote,
			},
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": true,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": false,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Remote",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "remote.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type:   v2.TelemetryTypeRemote,
				Remote: &v2.RemoteTelemetryConfig{},
			},
		},
		roundTripSpec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeRemote,
			},
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote": true,
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled": false,
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Remote",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
	{
		name: "remote.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeRemote,
				Remote: &v2.RemoteTelemetryConfig{
					Address:       "mixer-telemetry.some-namespace.svc.cluster.local",
					CreateService: &featureEnabled,
					Batching: &v2.TelemetryBatchingConfig{
						MaxEntries: &batchMaxEntries100,
						MaxTime:    "5",
					},
				},
			},
		},
		roundTripSpec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeRemote,
				Remote: &v2.RemoteTelemetryConfig{
					Address:       "mixer-telemetry.some-namespace.svc.cluster.local",
					CreateService: &featureEnabled,
					Batching: &v2.TelemetryBatchingConfig{
						MaxEntries: &batchMaxEntries100,
						MaxTime:    "5",
					},
				},
			},
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
				Remote: &v2.RemotePolicyConfig{
					CreateService: &featureEnabled,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"istioRemote":              true,
				"createRemoteSvcEndpoints": true,
				"remoteTelemetryAddress":   "mixer-telemetry.some-namespace.svc.cluster.local",
			},
			"mixer": map[string]interface{}{
				"telemetry": map[string]interface{}{
					"enabled":               false,
					"reportBatchMaxEntries": int64(100),
					"reportBatchMaxTime":    "5",
				},
			},
			"telemetry": map[string]interface{}{
				"implementation": "Remote",
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
}

func telemetryTestCasesV2(version versions.Version) []conversionTestCase{
	ver := version.String()
	return []conversionTestCase{
		{
			name: "nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version:   ver,
				Telemetry: &v2.TelemetryConfig{},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "none." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeNone,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "None",
					"enabled":        false,
					"v1": map[string]interface{}{
						"enabled": false,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "istiod.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeIstiod,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Istiod",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": false,
					},
					"v2": map[string]interface{}{
						"enabled": true,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "istiod.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeIstiod,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Istiod",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": false,
					},
					"v2": map[string]interface{}{
						"enabled": true,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "istiod.prometheus.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeIstiod,
				},
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Istiod",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": false,
					},
					"v2": map[string]interface{}{
						"enabled": true,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"prometheus": map[string]interface{}{"enabled": true},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}),
		},
		{
			name: "istiod.prometheus.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeIstiod,
				},
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
						Scrape: &featureEnabled,
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Istiod",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": false,
					},
					"v2": map[string]interface{}{
						"enabled": true,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"meshConfig": map[string]interface{}{
					"enablePrometheusMerge": true,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"prometheus": map[string]interface{}{"enabled": true},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}),
		},
		{
			name: "istiod.accesslog.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeIstiod,
				},
				Addons: &v2.AddonsConfig{
					Stackdriver: &v2.StackdriverAddonConfig{
						Telemetry: &v2.StackdriverTelemetryConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
							AccessLogging: &v2.StackdriverAccessLogTelemetryConfig{},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Istiod",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": false,
					},
					"v2": map[string]interface{}{
						"enabled": true,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}),
		},
		{
			name: "istiod.accesslog.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeIstiod,
				},
				Addons: &v2.AddonsConfig{
					Stackdriver: &v2.StackdriverAddonConfig{
						Telemetry: &v2.StackdriverTelemetryConfig{
							AccessLogging: &v2.StackdriverAccessLogTelemetryConfig{
								Enablement: v2.Enablement{
									Enabled: &featureEnabled,
								},
								LogWindowDuration: "43200s",
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Istiod",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": false,
					},
					"v2": map[string]interface{}{
						"enabled": true,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"accessLogPolicy": map[string]interface{}{
							"enabled":           true,
							"logWindowDuration": "43200s",
						},
					},
				},
			}),
		},
		{
			name: "istiod.stackdriver.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeIstiod,
				},
				Addons: &v2.AddonsConfig{
					Stackdriver: &v2.StackdriverAddonConfig{
						Telemetry: &v2.StackdriverTelemetryConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Istiod",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": false,
					},
					"v2": map[string]interface{}{
						"enabled": true,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}),
		},
		{
			name: "istiod.stackdriver.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeIstiod,
				},
				Addons: &v2.AddonsConfig{
					Stackdriver: &v2.StackdriverAddonConfig{
						Telemetry: &v2.StackdriverTelemetryConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
							EnableLogging:      &featureEnabled,
							EnableMetrics:      &featureEnabled,
							EnableContextGraph: &featureEnabled,
							ConfigOverride: v1.NewHelmValues(map[string]interface{}{
								"overrides": map[string]interface{}{
									"some-key": "some-val",
									"some-struct": map[string]interface{}{
										"nested-key": "nested-val",
									},
								},
							}),
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Istiod",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": false,
					},
					"v2": map[string]interface{}{
						"enabled": true,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled":      true,
							"logging":      map[string]interface{}{"enabled": true},
							"metrics":      map[string]interface{}{"enabled": true},
							"contextGraph": map[string]interface{}{"enabled": true},
						},
					},
				},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
							"configOverride": map[string]interface{}{
								"overrides": map[string]interface{}{
									"some-key": "some-val",
									"some-struct": map[string]interface{}{
										"nested-key": "nested-val",
									},
								},
							},
							"logging":    true,
							"monitoring": true,
							"topology":   true,
						},
					},
				},
			}),
		},
		{
			name: "mixer.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeMixer,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "mixer.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type:  v2.TelemetryTypeMixer,
					Mixer: &v2.MixerTelemetryConfig{},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "mixer.misc." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeMixer,
					Mixer: &v2.MixerTelemetryConfig{
						SessionAffinity: &featureEnabled,
						Batching: &v2.TelemetryBatchingConfig{
							MaxEntries: &batchMaxEntries100,
							MaxTime:    "5",
						},
						Loadshedding: &v2.TelemetryLoadSheddingConfig{
							Mode:             "enforce",
							LatencyThreshold: "100ms",
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled":                true,
						"reportBatchMaxEntries":  int64(100),
						"reportBatchMaxTime":     "5",
						"sessionAffinityEnabled": true,
						"loadshedding": map[string]interface{}{
							"mode":             "enforce",
							"latencyThreshold": "100ms",
						},
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "mixer.adapters.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeMixer,
					Mixer: &v2.MixerTelemetryConfig{
						Adapters: &v2.MixerTelemetryAdaptersConfig{},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "mixer.adapters.misc." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeMixer,
					Mixer: &v2.MixerTelemetryConfig{
						Adapters: &v2.MixerTelemetryAdaptersConfig{
							KubernetesEnv:  &featureEnabled,
							UseAdapterCRDs: &featureDisabled,
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
					"adapters": map[string]interface{}{
						"kubernetesenv": map[string]interface{}{
							"enabled": true,
						},
						"useAdapterCRDs": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "mixer.adapters.prometheus.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type:  v2.TelemetryTypeMixer,
					Mixer: &v2.MixerTelemetryConfig{},
				},
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"prometheus": map[string]interface{}{"enabled": true},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}),
		},
		{
			name: "mixer.adapters.prometheus.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type:  v2.TelemetryTypeMixer,
					Mixer: &v2.MixerTelemetryConfig{},
				},
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
						MetricsExpiryDuration: "10m",
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled":               true,
							"metricsExpiryDuration": "10m",
						},
					},
				},
				"prometheus": map[string]interface{}{"enabled": true},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}),
		},
		{
			name: "mixer.adapters.stackdriver.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type:  v2.TelemetryTypeMixer,
					Mixer: &v2.MixerTelemetryConfig{},
				},
				Addons: &v2.AddonsConfig{
					Stackdriver: &v2.StackdriverAddonConfig{
						Telemetry: &v2.StackdriverTelemetryConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}),
		},
		{
			name: "mixer.adapters.stackdriver.misc." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type:  v2.TelemetryTypeMixer,
					Mixer: &v2.MixerTelemetryConfig{},
				},
				Addons: &v2.AddonsConfig{
					Stackdriver: &v2.StackdriverAddonConfig{
						Telemetry: &v2.StackdriverTelemetryConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
							EnableLogging:      &featureEnabled,
							EnableMetrics:      &featureEnabled,
							EnableContextGraph: &featureEnabled,
							ConfigOverride: v1.NewHelmValues(map[string]interface{}{
								"overrides": map[string]interface{}{
									"some-key": "some-val",
									"some-struct": map[string]interface{}{
										"nested-key": "nested-val",
									},
								},
							}),
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled":      true,
							"logging":      map[string]interface{}{"enabled": true},
							"metrics":      map[string]interface{}{"enabled": true},
							"contextGraph": map[string]interface{}{"enabled": true},
						},
					},
				},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
							"configOverride": map[string]interface{}{
								"overrides": map[string]interface{}{
									"some-key": "some-val",
									"some-struct": map[string]interface{}{
										"nested-key": "nested-val",
									},
								},
							},
							"logging":    true,
							"monitoring": true,
							"topology":   true,
						},
					},
				},
			}),
		},
		{
			name: "mixer.adapters.stackdriver.auth.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeMixer,
				},
				Addons: &v2.AddonsConfig{
					Stackdriver: &v2.StackdriverAddonConfig{
						Telemetry: &v2.StackdriverTelemetryConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
							Auth: &v2.StackdriverAuthConfig{},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}),
		},
		{
			name: "mixer.adapters.stackdriver.auth.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeMixer,
				},
				Addons: &v2.AddonsConfig{
					Stackdriver: &v2.StackdriverAddonConfig{
						Telemetry: &v2.StackdriverTelemetryConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
							Auth: &v2.StackdriverAuthConfig{
								AppCredentials:     &featureEnabled,
								APIKey:             "mykey",
								ServiceAccountPath: "/path/to/sa",
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
							"auth": map[string]interface{}{
								"apiKey":             "mykey",
								"appCredentials":     true,
								"serviceAccountPath": "/path/to/sa",
							},
						},
					},
				},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			}),
		},
		{
			name: "mixer.adapters.stackdriver.tracer.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeMixer,
				},
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeStackdriver,
				},
				Addons: &v2.AddonsConfig{
					Stackdriver: &v2.StackdriverAddonConfig{
						Tracer: &v2.StackdriverTracerConfig{},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "stackdriver",
					},
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"tracer": map[string]interface{}{
								"enabled": true,
							},
						},
					},
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "stackdriver",
				},
			}),
		},
		{
			name: "mixer.adapters.stackdriver.tracer.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeMixer,
				},
				Tracing: &v2.TracingConfig{
					Type:     v2.TracerTypeStackdriver,
					Sampling: &traceSampling,
				},
				Addons: &v2.AddonsConfig{
					Stackdriver: &v2.StackdriverAddonConfig{
						Telemetry: &v2.StackdriverTelemetryConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
						},
						Tracer: &v2.StackdriverTracerConfig{
							Debug:                    &featureEnabled,
							MaxNumberOfAnnotations:   &maxAnnotations,
							MaxNumberOfAttributes:    &maxAttributes,
							MaxNumberOfMessageEvents: &maxEvents,
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "stackdriver",
					},
					"tracer": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"debug":                    true,
							"maxNumberOfAttributes":    maxAttributes,
							"maxNumberOfAnnotations":   maxAnnotations,
							"maxNumberOfMessageEvents": maxEvents,
						},
					},
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
							"tracer": map[string]interface{}{
								"enabled":           true,
								"sampleProbability": .01,
							},
						},
					},
				},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"stackdriver": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"pilot": map[string]interface{}{
					"traceSampling": 0.01,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "stackdriver",
				},
			}),
		},
		{
			name: "mixer.adapters.stdio.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeMixer,
					Mixer: &v2.MixerTelemetryConfig{
						Adapters: &v2.MixerTelemetryAdaptersConfig{
							Stdio: &v2.MixerTelemetryStdioConfig{
								Enablement: v2.Enablement{
									Enabled: &featureEnabled,
								},
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
					"adapters": map[string]interface{}{
						"stdio": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "mixer.adapters.stdio.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeMixer,
					Mixer: &v2.MixerTelemetryConfig{
						Adapters: &v2.MixerTelemetryAdaptersConfig{
							Stdio: &v2.MixerTelemetryStdioConfig{
								Enablement: v2.Enablement{
									Enabled: &featureEnabled,
								},
								OutputAsJSON: &featureEnabled,
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": true,
					},
					"adapters": map[string]interface{}{
						"stdio": map[string]interface{}{
							"enabled":      true,
							"outputAsJson": true,
						},
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Mixer",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "remote.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeRemote,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Remote",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "remote.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type:   v2.TelemetryTypeRemote,
					Remote: &v2.RemoteTelemetryConfig{},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled": false,
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Remote",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "remote.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeRemote,
					Remote: &v2.RemoteTelemetryConfig{
						Address:       "mixer-telemetry.some-namespace.svc.cluster.local",
						CreateService: &featureEnabled,
						Batching: &v2.TelemetryBatchingConfig{
							MaxEntries: &batchMaxEntries100,
							MaxTime:    "5",
						},
					},
				},
			},
			roundTripSpec: &v2.ControlPlaneSpec{
				Version: ver,
				Telemetry: &v2.TelemetryConfig{
					Type: v2.TelemetryTypeRemote,
					Remote: &v2.RemoteTelemetryConfig{
						Address:       "mixer-telemetry.some-namespace.svc.cluster.local",
						CreateService: &featureEnabled,
						Batching: &v2.TelemetryBatchingConfig{
							MaxEntries: &batchMaxEntries100,
							MaxTime:    "5",
						},
					},
				},
				Policy: &v2.PolicyConfig{
					Remote: &v2.RemotePolicyConfig{
						CreateService: &featureEnabled,
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"createRemoteSvcEndpoints": true,
					"remoteTelemetryAddress":   "mixer-telemetry.some-namespace.svc.cluster.local",
				},
				"mixer": map[string]interface{}{
					"telemetry": map[string]interface{}{
						"enabled":               false,
						"reportBatchMaxEntries": int64(100),
						"reportBatchMaxTime":    "5",
					},
				},
				"telemetry": map[string]interface{}{
					"implementation": "Remote",
					"enabled":        true,
					"v1": map[string]interface{}{
						"enabled": true,
					},
					"v2": map[string]interface{}{
						"enabled": false,
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
	}
}

func init() {
	telemetryTestCases = append(telemetryTestCases, telemetryTestCasesV1...)
	for _, v := range versions.AllV2Versions {
		telemetryTestCases = append(telemetryTestCases, telemetryTestCasesV2(v)...)
	}
}


func TestTelemetryConversionFromV2(t *testing.T) {
	for _, tc := range telemetryTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateTelemetryValues(specCopy, helmValues.GetContent()); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			if diff := cmp.Diff(tc.isolatedIstio.GetContent(), helmValues.GetContent()); diff != "" {
				t.Errorf("unexpected output converting v2 to values:\n%s", diff)
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), helmValues.GetContent())
			if version, err := versions.ParseVersion(tc.spec.Version); err == nil {
				if err := populateTelemetryConfig(helmValues.DeepCopy(), specv2, version); err != nil {
					t.Fatalf("error converting from values: %s", err)
				}
			} else {
				t.Fatalf("error parsing version: %s", err)
			}
			assertEquals(t, tc.spec.Telemetry, specv2.Telemetry)
		})
	}
}
