package conversion

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	prometheusTestAddress  = "prometheus.other-namespace.svc.cluster.local:9000"
	prometheusTestNodePort = int32(12345)
)

var prometheusTestCases []conversionTestCase

var prometheusTestCasesV1 = []conversionTestCase{
	{
		name: "install.misc." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Addons: &v2.AddonsConfig{
				Prometheus: &v2.PrometheusAddonConfig{
					Install: &v2.PrometheusInstallConfig{
						UseTLS:         &featureEnabled,
						Retention:      "6h",
						ScrapeInterval: "15s",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"prometheus": map[string]interface{}{
				"security": map[string]interface{}{
					"enabled": true,
				},
				"retention":      "6h",
				"scrapeInterval": "15s",
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

func prometheusTestCasesV2(version versions.Version) []conversionTestCase{
	ver := version.String()
	return []conversionTestCase{
		{
			name: "nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Addons: &v2.AddonsConfig{
					Prometheus: nil,
				},
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
				Version: ver,
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{},
				},
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
			name: "enablement." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"prometheus": map[string]interface{}{
					"enabled": true,
				},
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
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
			name: "existing." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Address: &prometheusTestAddress,
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"kiali": map[string]interface{}{
					"prometheusAddr": "prometheus.other-namespace.svc.cluster.local:9000",
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
			name: "install.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Install: &v2.PrometheusInstallConfig{},
					},
				},
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
			name: "install.misc." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Install: &v2.PrometheusInstallConfig{
							UseTLS:         &featureEnabled,
							Retention:      "6h",
							ScrapeInterval: "15s",
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"prometheus": map[string]interface{}{
					"provisionPrometheusCert": true,
					"retention":               "6h",
					"scrapeInterval":          "15s",
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
			name: "install.service.misc." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Install: &v2.PrometheusInstallConfig{
							Service: &v2.ComponentServiceConfig{
								Metadata: &v2.MetadataConfig{
									Annotations: map[string]string{
										"some-service-annotation":                     "service-annotation-value",
										"alpha.openshift.io/serving-cert-secret-name": "prometheus-tls",
									},
									Labels: map[string]string{
										"some-service-label": "service-label-value",
									},
								},
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"prometheus": map[string]interface{}{
					"service": map[string]interface{}{
						"annotations": map[string]interface{}{
							"some-service-annotation":                     "service-annotation-value",
							"alpha.openshift.io/serving-cert-secret-name": "prometheus-tls",
						},
						"labels": map[string]interface{}{
							"some-service-label": "service-label-value",
						},
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
			name: "install.service.ingress.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Install: &v2.PrometheusInstallConfig{
							Service: &v2.ComponentServiceConfig{
								Ingress: &v2.ComponentIngressConfig{},
							},
						},
					},
				},
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
			name: "install.service.ingress.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Install: &v2.PrometheusInstallConfig{
							Service: &v2.ComponentServiceConfig{
								Ingress: &v2.ComponentIngressConfig{
									Enablement: v2.Enablement{
										Enabled: &featureEnabled,
									},
									ContextPath: "/prometheus",
									Hosts: []string{
										"prometheus.example.com",
									},
									Metadata: &v2.MetadataConfig{
										Annotations: map[string]string{
											"ingress-annotation": "ingress-annotation-value",
										},
										Labels: map[string]string{
											"ingress-label": "ingress-label-value",
										},
									},
									TLS: v1.NewHelmValues(map[string]interface{}{
										"termination": "reencrypt",
									}),
								},
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"prometheus": map[string]interface{}{
					"contextPath": "/prometheus",
					"ingress": map[string]interface{}{
						"enabled":     true,
						"contextPath": "/prometheus",
						"annotations": map[string]interface{}{
							"ingress-annotation": "ingress-annotation-value",
						},
						"labels": map[string]interface{}{
							"ingress-label": "ingress-label-value",
						},
						"hosts": []interface{}{
							"prometheus.example.com",
						},
						"tls": map[string]interface{}{
							"termination": "reencrypt",
						},
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
			name: "install.service.nodeport." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Addons: &v2.AddonsConfig{
					Prometheus: &v2.PrometheusAddonConfig{
						Install: &v2.PrometheusInstallConfig{
							Service: &v2.ComponentServiceConfig{
								NodePort: &prometheusTestNodePort,
							},
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"prometheus": map[string]interface{}{
					"service": map[string]interface{}{
						"nodePort": map[string]interface{}{
							"enabled": true,
							"port":    12345,
						},
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
	prometheusTestCases = append(prometheusTestCases, prometheusTestCasesV1...)
	for _, v := range versions.AllV2Versions {
		prometheusTestCases = append(prometheusTestCases, prometheusTestCasesV2(v)...)
	}
}

func TestPrometheusConversionFromV2(t *testing.T) {
	for _, tc := range prometheusTestCases {
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
