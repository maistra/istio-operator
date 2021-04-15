package conversion

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	jaegerMaxTraces               = int64(15000)
	jaegerElasticsearchNodeCount  = int32(5)
	jaegerElasticsearchNodeCount3 = int32(3)
	traceSampling                 = int32(1)
	traceSamplingInt              = int32(100)
	traceSamplingInt10000         = int32(10000)
)

var jaegerTestCases []conversionTestCase

func jaegerTestCasesV2(version versions.Version) []conversionTestCase{
	ver := version.String()
	return []conversionTestCase{
		{
			name: "none." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type:     v2.TracerTypeNone,
					Sampling: &traceSampling,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": false,
					"proxy": map[string]interface{}{
						"tracer": "none",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": false,
				},
				"pilot": map[string]interface{}{
					"traceSampling": 0.01,
				},
				"tracing": map[string]interface{}{
					"enabled":  false,
					"provider": "none",
				},
			}),
		},
		{
			name: "nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Sampling: &traceSamplingInt,
					Type:     v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"pilot": map[string]interface{}{
					"traceSampling": 1,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "simple." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Name: "my-jaeger",
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "my-jaeger",
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "install.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Name:    "my-jaeger",
						Install: &v2.JaegerInstallConfig{},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "my-jaeger",
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "install.storage.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "my-jaeger",
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "install.storage.memory.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Type: v2.JaegerStorageTypeMemory,
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "my-jaeger",
						"template":     "all-in-one",
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "install.storage.memory.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
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
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "my-jaeger",
						"template":     "all-in-one",
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "install.storage.memory.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
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
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "install.storage.elasticsearch.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Storage: &v2.JaegerStorageConfig{
								Type: v2.JaegerStorageTypeElasticsearch,
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "my-jaeger",
						"template":     "production-elasticsearch",
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "install.storage.elasticsearch.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
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
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "my-jaeger",
						"template":     "production-elasticsearch",
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "install.storage.elasticsearch.basic." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
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
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "my-jaeger",
						"template":     "production-elasticsearch",
						"elasticsearch": map[string]interface{}{
							"nodeCount":        5,
							"redundancyPolicy": "ZeroRedundancy",
							"storage": map[string]interface{}{
								"size":             "5Gi",
								"storageClassName": "gp2",
							},
						},
						"esIndexCleaner": map[string]interface{}{
							"enabled":      true,
							"numberOfDays": 7,
							"schedule":     "55 23 * * *"},
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "install.ingress.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Ingress: &v2.JaegerIngressConfig{},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
						"resourceName": "my-jaeger",
					},
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
		{
			name: "install.ingress.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Tracing: &v2.TracingConfig{
					Type: v2.TracerTypeJaeger,
				},
				Addons: &v2.AddonsConfig{
					Jaeger: &v2.JaegerAddonConfig{
						Name: "my-jaeger",
						Install: &v2.JaegerInstallConfig{
							Ingress: &v2.JaegerIngressConfig{
								Enablement: v2.Enablement{
									Enabled: &featureEnabled,
								},
								Metadata: &v2.MetadataConfig{
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
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"tracing": map[string]interface{}{
					"jaeger": map[string]interface{}{
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
					"enableTracing": true,
					"proxy": map[string]interface{}{
						"tracer": "zipkin",
					},
				},
				"meshConfig": map[string]interface{}{
					"enableTracing": true,
				},
				"tracing": map[string]interface{}{
					"enabled":  true,
					"provider": "jaeger",
				},
			}),
		},
	}
}

func init() {
	for _, v := range versions.AllV2Versions {
		jaegerTestCases = append(jaegerTestCases, jaegerTestCasesV2(v)...)
	}
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
