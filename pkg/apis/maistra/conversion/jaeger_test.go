package conversion

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	jaegerMaxTraces              = int64(15000)
	jaegerElasticsearchNodeCount = int32(5)
)

var jaegerTestCases = []conversionTestCase{
	{
		name: "none." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeNone,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"tracing": map[string]interface{}{
				"enabled":  false,
				"provider": "none",
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
		name: "nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type:   v2.TracerTypeJaeger,
					Jaeger: nil,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
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
		name: "defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type:   v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install": false,
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
		name: "simple." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      false,
					"resourceName": "my-jaeger",
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
		name: "install.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name:    "my-jaeger",
						Install: &v2.JaegerInstallConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
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
		name: "install.storage.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
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
		name: "install.storage.memory.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Type: v2.JaegerStorageTypeMemory,
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "all-in-one",
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
		name: "install.storage.memory.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Type:   v2.JaegerStorageTypeMemory,
								Memory: &v2.JaegerMemoryStorageConfig{},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "all-in-one",
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
		name: "install.storage.memory.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Type: v2.JaegerStorageTypeMemory,
								Memory: &v2.JaegerMemoryStorageConfig{
									MaxTraces: &jaegerMaxTraces,
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "all-in-one",
					"memory": map[string]interface{}{
						"max_traces": 15000,
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
		name: "install.storage.elasticsearch.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Type: v2.JaegerStorageTypeElasticsearch,
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "production-elasticsearch",
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
		name: "install.storage.elasticsearch.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Type:          v2.JaegerStorageTypeElasticsearch,
								Elasticsearch: &v2.JaegerElasticsearchStorageConfig{},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "production-elasticsearch",
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
		name: "install.storage.elasticsearch.basic." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Type: v2.JaegerStorageTypeElasticsearch,
								Elasticsearch: &v2.JaegerElasticsearchStorageConfig{
									NodeCount: &jaegerElasticsearchNodeCount,
									IndexCleaner: v1.NewHelmValues(map[string]interface{}{
										"enabled":      true,
										"numberOfDays": 7,
										"schedule":     "55 23 * * *",
									}).DeepCopy(),
									RedundancyPolicy: "ZeroRedundancy",
									Storage: v1.NewHelmValues(map[string]interface{}{
										"storageClassName": "gp2",
										"size":             "5Gi",
									}).DeepCopy(),
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
					"template":     "production-elasticsearch",
					"elasticsearch": map[string]interface{}{
						"nodeCount": 5,
						"esIndexCleaner": map[string]interface{}{
							"enabled":      true,
							"numberOfDays": 7,
							"schedule":     "55 23 * * *"},
						"redundancyPolicy": "ZeroRedundancy",
						"storage": map[string]interface{}{
							"size":             "5Gi",
							"storageClassName": "gp2",
						},
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
		name: "install.ingress.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Ingress: &v2.JaegerIngressConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
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
		name: "install.ingress.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Tracing: v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
					Jaeger: &v2.JaegerTracerConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Ingress: &v2.JaegerIngressConfig{
								Enablement: v2.Enablement{
									Enabled: &featureEnabled,
								},
								Metadata: v2.MetadataConfig{
									Annotations: map[string]string{
										"ingress-annotation": "ingress-annotation-value",
									},
									Labels: map[string]string{
										"ingress-label": "ingress-label-value",
									},
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"proxy": map[string]interface{}{
					"tracer": "jaeger",
				},
			},
			"tracing": map[string]interface{}{
				"enabled":  true,
				"provider": "jaeger",
				"jaeger": map[string]interface{}{
					"install":      true,
					"resourceName": "my-jaeger",
				},
				"ingress": map[string]interface{}{
					"enabled": true,
					"annotations": map[string]interface{}{
						"ingress-annotation": "ingress-annotation-value",
					},
					"labels": map[string]interface{}{
						"ingress-label": "ingress-label-value",
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
}

func TestJaegerConversionFromV2(t *testing.T) {
	for _, tc := range jaegerTestCases {
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
