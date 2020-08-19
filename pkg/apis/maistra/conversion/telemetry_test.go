package conversion

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	batchMaxEntries100 = int32(100)
)

var telemetryTestCases = []conversionTestCase{
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
			Version:   versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{},
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
			Version:   versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{},
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
		name: "istiod.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeIstiod,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
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
			Telemetry: &v2.TelemetryConfig{
				Type:   v2.TelemetryTypeIstiod,
				Istiod: &v2.IstiodTelemetryConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
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
		name: "istiod.prometheus.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeIstiod,
				Istiod: &v2.IstiodTelemetryConfig{
					PrometheusFilter: &v2.PrometheusFilterConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
			},
			"telemetry": map[string]interface{}{
				"implementation": "Istiod",
				"enabled":        true,
				"v1": map[string]interface{}{
					"enabled": false,
				},
				"v2": map[string]interface{}{
					"enabled": true,
					"prometheus": map[string]interface{}{
						"enabled":     true,
						"wasmEnabled": false,
					},
				},
			},
			"meshConfig": map[string]interface{}{
				"enablePrometheusMerge": false,
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
		name: "istiod.prometheus.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeIstiod,
				Istiod: &v2.IstiodTelemetryConfig{
					PrometheusFilter: &v2.PrometheusFilterConfig{
						Scrape:      featureEnabled,
						WASMEnabled: featureDisabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
			},
			"telemetry": map[string]interface{}{
				"implementation": "Istiod",
				"enabled":        true,
				"v1": map[string]interface{}{
					"enabled": false,
				},
				"v2": map[string]interface{}{
					"enabled": true,
					"prometheus": map[string]interface{}{
						"enabled":     true,
						"wasmEnabled": false,
					},
				},
			},
			"meshConfig": map[string]interface{}{
				"enablePrometheusMerge": true,
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
		name: "istiod.metadata.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeIstiod,
				Istiod: &v2.IstiodTelemetryConfig{
					MetadataExchange: &v2.MetadataExchangeConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
			},
			"telemetry": map[string]interface{}{
				"implementation": "Istiod",
				"enabled":        true,
				"v1": map[string]interface{}{
					"enabled": false,
				},
				"v2": map[string]interface{}{
					"enabled": true,
					"metadataExchange": map[string]interface{}{
						"wasmEnabled": false,
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
		name: "istiod.metadata.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeIstiod,
				Istiod: &v2.IstiodTelemetryConfig{
					MetadataExchange: &v2.MetadataExchangeConfig{
						WASMEnabled: featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
			},
			"telemetry": map[string]interface{}{
				"implementation": "Istiod",
				"enabled":        true,
				"v1": map[string]interface{}{
					"enabled": false,
				},
				"v2": map[string]interface{}{
					"enabled": true,
					"metadataExchange": map[string]interface{}{
						"wasmEnabled": true,
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
		name: "istiod.accesslog.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeIstiod,
				Istiod: &v2.IstiodTelemetryConfig{
					AccessLogTelemetryFilter: &v2.AccessLogTelemetryFilterConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
			},
			"telemetry": map[string]interface{}{
				"implementation": "Istiod",
				"enabled":        true,
				"v1": map[string]interface{}{
					"enabled": false,
				},
				"v2": map[string]interface{}{
					"enabled": true,
					"accessLogPolicy": map[string]interface{}{
						"enabled":           true,
						"logWindowDuration": "",
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
		name: "istiod.accesslog.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeIstiod,
				Istiod: &v2.IstiodTelemetryConfig{
					AccessLogTelemetryFilter: &v2.AccessLogTelemetryFilterConfig{
						LogWindowDuration: "43200s",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
			},
			"telemetry": map[string]interface{}{
				"implementation": "Istiod",
				"enabled":        true,
				"v1": map[string]interface{}{
					"enabled": false,
				},
				"v2": map[string]interface{}{
					"enabled": true,
					"accessLogPolicy": map[string]interface{}{
						"enabled":           true,
						"logWindowDuration": "43200s",
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
		name: "istiod.stackdriver.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeIstiod,
				Istiod: &v2.IstiodTelemetryConfig{
					StackDriverFilter: &v2.StackDriverFilterConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
			},
			"telemetry": map[string]interface{}{
				"implementation": "Istiod",
				"enabled":        true,
				"v1": map[string]interface{}{
					"enabled": false,
				},
				"v2": map[string]interface{}{
					"enabled": true,
					"stackdriver": map[string]interface{}{
						"enabled":         true,
						"configOverride":  map[string]interface{}(nil),
						"disableOutbound": false,
						"logging":         false,
						"monitoring":      false,
						"topology":        false,
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
		name: "istiod.stackdriver.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeIstiod,
				Istiod: &v2.IstiodTelemetryConfig{
					StackDriverFilter: &v2.StackDriverFilterConfig{
						DisableOutbound: true,
						Logging:         true,
						Monitoring:      true,
						Topology:        true,
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
				"enabled": false,
			},
			"telemetry": map[string]interface{}{
				"implementation": "Istiod",
				"enabled":        true,
				"v1": map[string]interface{}{
					"enabled": false,
				},
				"v2": map[string]interface{}{
					"enabled": true,
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
						"disableOutbound": true,
						"logging":         true,
						"monitoring":      true,
						"topology":        true,
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
		name: "mixer.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
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
			Telemetry: &v2.TelemetryConfig{
				Type:  v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
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
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					SessionAffinity: &featureEnabled,
					Batching: &v2.TelemetryBatchingConfig{
						MaxEntries: &batchMaxEntries100,
						MaxTime:    "5",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled":                true,
					"reportBatchMaxEntries":  100,
					"reportBatchMaxTime":     "5",
					"sessionAffinityEnabled": true,
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
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
					"stdio": map[string]interface{}{
						"enabled": false,
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
		name: "mixer.adapters.misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
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
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"kubernetesenv": map[string]interface{}{
						"enabled": true,
					},
					"useAdapterCRDs": false,
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
					"stdio": map[string]interface{}{
						"enabled": false,
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
		name: "mixer.adapters.prometheus.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Prometheus: &v2.MixerTelemetryPrometheusConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": true,
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
					"stdio": map[string]interface{}{
						"enabled": false,
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
		name: "mixer.adapters.prometheus.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Prometheus: &v2.MixerTelemetryPrometheusConfig{
							MetricsExpiryDuration: "10m",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled":               true,
						"metricsExpiryDuration": "10m",
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
					"stdio": map[string]interface{}{
						"enabled": false,
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
		name: "mixer.adapters.stackdriver.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": false,
						},
						"logging": map[string]interface{}{
							"enabled": false,
						},
						"metrics": map[string]interface{}{
							"enabled": false,
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
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
		name: "mixer.adapters.stackdriver.misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{
							EnableContextGraph: featureEnabled,
							EnableLogging:      featureEnabled,
							EnableMetrics:      featureEnabled,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": true,
						},
						"logging": map[string]interface{}{
							"enabled": true,
						},
						"metrics": map[string]interface{}{
							"enabled": true,
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
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
		name: "mixer.adapters.stackdriver.auth.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{
							Auth: &v2.MixerTelemetryStackdriverAuthConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": false,
						},
						"logging": map[string]interface{}{
							"enabled": false,
						},
						"metrics": map[string]interface{}{
							"enabled": false,
						},
						"auth": map[string]interface{}{
							"apiKey":             "",
							"appCredentials":     false,
							"serviceAccountPath": "",
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
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
		name: "mixer.adapters.stackdriver.auth.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{
							Auth: &v2.MixerTelemetryStackdriverAuthConfig{
								AppCredentials:     true,
								APIKey:             "mykey",
								ServiceAccountPath: "/path/to/sa",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": false,
						},
						"logging": map[string]interface{}{
							"enabled": false,
						},
						"metrics": map[string]interface{}{
							"enabled": false,
						},
						"auth": map[string]interface{}{
							"apiKey":             "mykey",
							"appCredentials":     true,
							"serviceAccountPath": "/path/to/sa",
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
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
		name: "mixer.adapters.stackdriver.tracer.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{
							Tracer: &v2.MixerTelemetryStackdriverTracerConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": false,
						},
						"logging": map[string]interface{}{
							"enabled": false,
						},
						"metrics": map[string]interface{}{
							"enabled": false,
						},
						"tracer": map[string]interface{}{
							"sampleProbability": 0,
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
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
		name: "mixer.adapters.stackdriver.tracer.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{
							Tracer: &v2.MixerTelemetryStackdriverTracerConfig{
								SampleProbability: 50,
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": false,
						},
						"logging": map[string]interface{}{
							"enabled": false,
						},
						"metrics": map[string]interface{}{
							"enabled": false,
						},
						"tracer": map[string]interface{}{
							"sampleProbability": 50,
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
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
		name: "mixer.adapters.stdio.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stdio: &v2.MixerTelemetryStdioConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
					"stdio": map[string]interface{}{
						"enabled":      true,
						"outputAsJson": false,
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
		name: "mixer.adapters.stdio.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stdio: &v2.MixerTelemetryStdioConfig{
							OutputAsJSON: featureEnabled,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
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
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
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
			Telemetry: &v2.TelemetryConfig{
				Type:  v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
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
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					SessionAffinity: &featureEnabled,
					Batching: &v2.TelemetryBatchingConfig{
						MaxEntries: &batchMaxEntries100,
						MaxTime:    "5",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled":                true,
					"reportBatchMaxEntries":  100,
					"reportBatchMaxTime":     "5",
					"sessionAffinityEnabled": true,
				},
			},
			"telemetry": map[string]interface{}{
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
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
					"stdio": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"kubernetesenv": map[string]interface{}{
						"enabled": true,
					},
					"useAdapterCRDs": false,
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
					"stdio": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		name: "mixer.adapters.prometheus.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Prometheus: &v2.MixerTelemetryPrometheusConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": true,
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
					"stdio": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		name: "mixer.adapters.prometheus.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Prometheus: &v2.MixerTelemetryPrometheusConfig{
							MetricsExpiryDuration: "10m",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled":               true,
						"metricsExpiryDuration": "10m",
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
					"stdio": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		name: "mixer.adapters.stackdriver.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": false,
						},
						"logging": map[string]interface{}{
							"enabled": false,
						},
						"metrics": map[string]interface{}{
							"enabled": false,
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		name: "mixer.adapters.stackdriver.misc." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{
							EnableContextGraph: featureEnabled,
							EnableLogging:      featureEnabled,
							EnableMetrics:      featureEnabled,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": true,
						},
						"logging": map[string]interface{}{
							"enabled": true,
						},
						"metrics": map[string]interface{}{
							"enabled": true,
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		name: "mixer.adapters.stackdriver.auth.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{
							Auth: &v2.MixerTelemetryStackdriverAuthConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": false,
						},
						"logging": map[string]interface{}{
							"enabled": false,
						},
						"metrics": map[string]interface{}{
							"enabled": false,
						},
						"auth": map[string]interface{}{
							"apiKey":             "",
							"appCredentials":     false,
							"serviceAccountPath": "",
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		name: "mixer.adapters.stackdriver.auth.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{
							Auth: &v2.MixerTelemetryStackdriverAuthConfig{
								AppCredentials:     true,
								APIKey:             "mykey",
								ServiceAccountPath: "/path/to/sa",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": false,
						},
						"logging": map[string]interface{}{
							"enabled": false,
						},
						"metrics": map[string]interface{}{
							"enabled": false,
						},
						"auth": map[string]interface{}{
							"apiKey":             "mykey",
							"appCredentials":     true,
							"serviceAccountPath": "/path/to/sa",
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		name: "mixer.adapters.stackdriver.tracer.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{
							Tracer: &v2.MixerTelemetryStackdriverTracerConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": false,
						},
						"logging": map[string]interface{}{
							"enabled": false,
						},
						"metrics": map[string]interface{}{
							"enabled": false,
						},
						"tracer": map[string]interface{}{
							"sampleProbability": 0,
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		name: "mixer.adapters.stackdriver.tracer.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stackdriver: &v2.MixerTelemetryStackdriverConfig{
							Tracer: &v2.MixerTelemetryStackdriverTracerConfig{
								SampleProbability: 50,
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": true,
						"contextGraph": map[string]interface{}{
							"enabled": false,
						},
						"logging": map[string]interface{}{
							"enabled": false,
						},
						"metrics": map[string]interface{}{
							"enabled": false,
						},
						"tracer": map[string]interface{}{
							"sampleProbability": 50,
						},
					},
					"stdio": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		name: "mixer.adapters.stdio.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stdio: &v2.MixerTelemetryStdioConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
					"stdio": map[string]interface{}{
						"enabled":      true,
						"outputAsJson": false,
					},
				},
			},
			"telemetry": map[string]interface{}{
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
		name: "mixer.adapters.stdio.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeMixer,
				Mixer: &v2.MixerTelemetryConfig{
					Adapters: &v2.MixerTelemetryAdaptersConfig{
						Stdio: &v2.MixerTelemetryStdioConfig{
							OutputAsJSON: featureEnabled,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
				"adapters": map[string]interface{}{
					"prometheus": map[string]interface{}{
						"enabled": false,
					},
					"stackdriver": map[string]interface{}{
						"enabled": false,
					},
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
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeRemote,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remoteTelemetryAddress":   "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"telemetry": map[string]interface{}{
					"enabled": true,
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
			Telemetry: &v2.TelemetryConfig{
				Type:   v2.TelemetryTypeRemote,
				Remote: &v2.RemoteTelemetryConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remoteTelemetryAddress":   "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"telemetry": map[string]interface{}{
					"enabled": true,
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
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeRemote,
				Remote: &v2.RemoteTelemetryConfig{
					Address:       "mixer-telemetry.some-namespace.svc.cluster.local",
					CreateService: true,
					Batching: &v2.TelemetryBatchingConfig{
						MaxEntries: &batchMaxEntries100,
						MaxTime:    "5",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": true,
				"remoteTelemetryAddress":   "mixer-telemetry.some-namespace.svc.cluster.local",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"telemetry": map[string]interface{}{
					"enabled":               true,
					"reportBatchMaxEntries": 100,
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
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeRemote,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remoteTelemetryAddress":   "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
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
			Telemetry: &v2.TelemetryConfig{
				Type:   v2.TelemetryTypeRemote,
				Remote: &v2.RemoteTelemetryConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remoteTelemetryAddress":   "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"telemetry": map[string]interface{}{
					"enabled": true,
				},
			},
			"telemetry": map[string]interface{}{
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
			Telemetry: &v2.TelemetryConfig{
				Type: v2.TelemetryTypeRemote,
				Remote: &v2.RemoteTelemetryConfig{
					Address:       "mixer-telemetry.some-namespace.svc.cluster.local",
					CreateService: true,
					Batching: &v2.TelemetryBatchingConfig{
						MaxEntries: &batchMaxEntries100,
						MaxTime:    "5",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": true,
				"remoteTelemetryAddress":   "mixer-telemetry.some-namespace.svc.cluster.local",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"telemetry": map[string]interface{}{
					"enabled":               true,
					"reportBatchMaxEntries": 100,
					"reportBatchMaxTime":    "5",
				},
			},
			"telemetry": map[string]interface{}{
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

func TestTelemetryConversionFromV2(t *testing.T) {
	for _, tc := range telemetryTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateTelemetryValues(specCopy, helmValues.GetContent()); err != nil {
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
