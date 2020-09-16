package conversion

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	grafanaTestAddress  = "grafana.other-namespace.svc.cluster.local:3001"
	grafanaTestNodePort = int32(12345)
)

var grafanaTestCases = []conversionTestCase{
	{
		name: "nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: nil,
			},
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
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{},
			},
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
		name: "enablement." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Enablement: v2.Enablement{
						Enabled: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"grafana": map[string]interface{}{
				"enabled": true,
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
		name: "existing." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Address: &grafanaTestAddress,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"kiali": map[string]interface{}{
				"dashboard": map[string]interface{}{
					"grafanaURL": "grafana.other-namespace.svc.cluster.local:3001",
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
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{},
				},
			},
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
		name: "install.env." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Config: v2.GrafanaConfig{
							Env: map[string]string{
								"GF_SMTP_ENABLED": "true",
							},
							EnvSecrets: map[string]string{
								"GF_SMTP_USER": "grafana-secrets",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"grafana": map[string]interface{}{
				"env": map[string]interface{}{
					"GF_SMTP_ENABLED": "true",
				},
				"envSecrets": map[string]interface{}{
					"GF_SMTP_USER": "grafana-secrets",
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
		name: "install.persistence.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Persistence: &v2.ComponentPersistenceConfig{},
					},
				},
			},
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
		name: "install.persistence.simple." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Persistence: &v2.ComponentPersistenceConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
							AccessMode:       corev1.ReadWriteOnce,
							StorageClassName: "standarad",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"grafana": map[string]interface{}{
				"accessMode":       "ReadWriteOnce",
				"persist":          true,
				"storageClassName": "standarad",
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
		name: "install.persistence.resources.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Persistence: &v2.ComponentPersistenceConfig{
							Resources: &corev1.ResourceRequirements{},
						},
					},
				},
			},
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
		name: "install.persistence.resources.values." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Persistence: &v2.ComponentPersistenceConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("5Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("25Gi"),
								},
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"grafana": map[string]interface{}{
				"persistenceResources": map[string]interface{}{
					"limits": map[string]interface{}{
						"storage": "25Gi",
					},
					"requests": map[string]interface{}{
						"storage": "5Gi",
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
		name: "install.security.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Security: &v2.GrafanaSecurityConfig{},
					},
				},
			},
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
		name: "install.security.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Security: &v2.GrafanaSecurityConfig{
							Enablement: v2.Enablement{
								Enabled: &featureEnabled,
							},
							PassphraseKey: "passphrase",
							SecretName:    "htpasswd",
							UsernameKey:   "username",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"grafana": map[string]interface{}{
				"security": map[string]interface{}{
					"enabled":       true,
					"passphraseKey": "passphrase",
					"secretName":    "htpasswd",
					"usernameKey":   "username",
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
		name: "install.service.misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Service: v2.ComponentServiceConfig{
							Metadata: v2.MetadataConfig{
								Annotations: map[string]string{
									"some-service-annotation": "service-annotation-value",
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
			"grafana": map[string]interface{}{
				"service": map[string]interface{}{
					"annotations": map[string]interface{}{
						"some-service-annotation": "service-annotation-value",
					},
					"labels": map[string]interface{}{
						"some-service-label": "service-label-value",
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
		name: "install.service.ingress.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Service: v2.ComponentServiceConfig{
							Ingress: &v2.ComponentIngressConfig{},
						},
					},
				},
			},
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
		name: "install.service.ingress.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Service: v2.ComponentServiceConfig{
							Ingress: &v2.ComponentIngressConfig{
								Enablement: v2.Enablement{
									Enabled: &featureEnabled,
								},
								ContextPath: "/grafana",
								Hosts: []string{
									"grafana.example.com",
								},
								Metadata: v2.MetadataConfig{
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
			"grafana": map[string]interface{}{
				"contextPath": "/grafana",
				"ingress": map[string]interface{}{
					"enabled":     true,
					"contextPath": "/grafana",
					"annotations": map[string]interface{}{
						"ingress-annotation": "ingress-annotation-value",
					},
					"labels": map[string]interface{}{
						"ingress-label": "ingress-label-value",
					},
					"hosts": []interface{}{
						"grafana.example.com",
					},
					"tls": map[string]interface{}{
						"termination": "reencrypt",
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
		name: "install.service.nodeport." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Addons: &v2.AddonsConfig{
				Grafana: &v2.GrafanaAddonConfig{
					Install: &v2.GrafanaInstallConfig{
						Service: v2.ComponentServiceConfig{
							NodePort: &grafanaTestNodePort,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"grafana": map[string]interface{}{
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

func TestGrafanaConversionFromV2(t *testing.T) {
	for _, tc := range grafanaTestCases {
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
