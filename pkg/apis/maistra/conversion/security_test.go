package conversion

import (
	"reflect"
	"testing"

	"sigs.k8s.io/yaml"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var securityTestCases = []conversionTestCase{
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version:  versions.V2_0.String(),
			Security: &v2.SecurityConfig{},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Enable: &featureEnabled,
					Auto:   &featureEnabled,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"mtls": map[string]interface{}{
					"enabled": true,
					"auto":    true,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type:   v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.common." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							WorkloadCertTTLDefault: "24h",
							WorkloadCertTTLMax:     "7d",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
				"env": map[string]interface{}{
					"DEFAULT_WORKLOAD_CERT_TTL": "24h",
					"MAX_WORKLOAD_CERT_TTL":     "7d",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.selfsigned.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type: v2.IstioCertificateSignerTypeSelfSigned,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": true,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.selfsigned.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type:       v2.IstioCertificateSignerTypeSelfSigned,
							SelfSigned: &v2.IstioSelfSignedCertificateSignerConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": true,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.selfsigned.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type: v2.IstioCertificateSignerTypeSelfSigned,
							SelfSigned: &v2.IstioSelfSignedCertificateSignerConfig{
								CheckPeriod:  "1h",
								EnableJitter: &featureEnabled,
								GracePeriod:  "20%",
								TTL:          "1y",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
				"env": map[string]interface{}{
					"CITADEL_ENABLE_JITTER_FOR_ROOT_CERT_ROTATOR":           true,
					"CITADEL_SELF_SIGNED_CA_CERT_TTL":                       "1y",
					"CITADEL_SELF_SIGNED_ROOT_CERT_CHECK_INTERVAL":          "1h",
					"CITADEL_SELF_SIGNED_ROOT_CERT_GRACE_PERIOD_PERCENTILE": "20%",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": true,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.privatekey.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type: v2.IstioCertificateSignerTypePrivateKey,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": false,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.privatekey.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type:       v2.IstioCertificateSignerTypePrivateKey,
							PrivateKey: &v2.IstioPrivateKeyCertificateSignerConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": false,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.privatekey.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type: v2.IstioCertificateSignerTypePrivateKey,
							PrivateKey: &v2.IstioPrivateKeyCertificateSignerConfig{
								RootCADir: "/etc/cacerts",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
				"env": map[string]interface{}{
					"ROOT_CA_DIR": "/etc/cacerts",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": false,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.custom.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeCustom,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Custom",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.custom.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type:   v2.CertificateAuthorityTypeCustom,
						Custom: &v2.CustomCertificateAuthorityConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"caAddress": "",
			},
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Custom",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.custom.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeCustom,
						Custom: &v2.CustomCertificateAuthorityConfig{
							Address: "my-caprovider.example.com",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"caAddress": "my-caprovider.example.com",
			},
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Custom",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.nil." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type:   v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.common." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							WorkloadCertTTLDefault: "24h",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"workloadCertTtl": "24h",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.selfsigned.nil." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type: v2.IstioCertificateSignerTypeSelfSigned,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": true,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.selfsigned.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type:       v2.IstioCertificateSignerTypeSelfSigned,
							SelfSigned: &v2.IstioSelfSignedCertificateSignerConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": true,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.selfsigned.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type: v2.IstioCertificateSignerTypeSelfSigned,
							SelfSigned: &v2.IstioSelfSignedCertificateSignerConfig{
								CheckPeriod:  "1h",
								EnableJitter: &featureEnabled,
								GracePeriod:  "20%",
								TTL:          "1y",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": true,
				"env": map[string]interface{}{
					"CITADEL_ENABLE_JITTER_FOR_ROOT_CERT_ROTATOR":           true,
					"CITADEL_SELF_SIGNED_CA_CERT_TTL":                       "1y",
					"CITADEL_SELF_SIGNED_ROOT_CERT_CHECK_INTERVAL":          "1h",
					"CITADEL_SELF_SIGNED_ROOT_CERT_GRACE_PERIOD_PERCENTILE": "20%",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.privatekey.nil." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type: v2.IstioCertificateSignerTypePrivateKey,
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": false,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.privatekey.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type:       v2.IstioCertificateSignerTypePrivateKey,
							PrivateKey: &v2.IstioPrivateKeyCertificateSignerConfig{},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": false,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.istiod.privatekey.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type: v2.IstioCertificateSignerTypePrivateKey,
							PrivateKey: &v2.IstioPrivateKeyCertificateSignerConfig{
								RootCADir: "/etc/cacerts",
							},
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Istiod",
				},
			},
			"security": map[string]interface{}{
				"selfSigned": false,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.custom.nil." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeCustom,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Custom",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.custom.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type:   v2.CertificateAuthorityTypeCustom,
						Custom: &v2.CustomCertificateAuthorityConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"caAddress": "",
			},
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Custom",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "ca.custom.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					CertificateAuthority: v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeCustom,
						Custom: &v2.CustomCertificateAuthorityConfig{
							Address: "my-caprovider.example.com",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"caAddress": "my-caprovider.example.com",
			},
			"pilot": map[string]interface{}{
				"ca": map[string]interface{}{
					"implementation": "Custom",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "identity.kubernetes." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Identity: v2.IdentityConfig{
						Type: v2.IdentityConfigTypeKubernetes,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"jwtPolicy": "first-party-jwt",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "identity.thirdparty.nil." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Identity: v2.IdentityConfig{
						Type: v2.IdentityConfigTypeThirdParty,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"jwtPolicy": "third-party-jwt",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "identity.thirdparty.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Identity: v2.IdentityConfig{
						Type:       v2.IdentityConfigTypeThirdParty,
						ThirdParty: &v2.ThirdPartyIdentityConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"jwtPolicy": "third-party-jwt",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "identity.thirdparty.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Identity: v2.IdentityConfig{
						Type: v2.IdentityConfigTypeThirdParty,
						ThirdParty: &v2.ThirdPartyIdentityConfig{
							Audience: "istio-ca",
							Issuer:   "https://my-issuer.example.com",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"jwtPolicy": "third-party-jwt",
				"sds": map[string]interface{}{
					"token": map[string]interface{}{
						"aud": "istio-ca",
					},
				},
			},
			"pilot": map[string]interface{}{
				"env": map[string]interface{}{
					"TOKEN_ISSUER": "https://my-issuer.example.com",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "identity.kubernetes." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Identity: v2.IdentityConfig{
						Type: v2.IdentityConfigTypeKubernetes,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"jwtPolicy": "first-party-jwt",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "identity.thirdparty.nil." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Identity: v2.IdentityConfig{
						Type: v2.IdentityConfigTypeThirdParty,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"jwtPolicy": "third-party-jwt",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "identity.thirdparty.defaults." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Identity: v2.IdentityConfig{
						Type:       v2.IdentityConfigTypeThirdParty,
						ThirdParty: &v2.ThirdPartyIdentityConfig{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"jwtPolicy": "third-party-jwt",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "identity.thirdparty.full." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Identity: v2.IdentityConfig{
						Type: v2.IdentityConfigTypeThirdParty,
						ThirdParty: &v2.ThirdPartyIdentityConfig{
							Audience: "istio-ca",
							Issuer:   "https://my-issuer.example.com",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"jwtPolicy": "third-party-jwt",
				"sds": map[string]interface{}{
					"token": map[string]interface{}{
						"aud": "istio-ca",
					},
				},
			},
			"pilot": map[string]interface{}{
				"env": map[string]interface{}{
					"TOKEN_ISSUER": "https://my-issuer.example.com",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "trust.domain." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Trust: v2.TrustConfig{
						Domain: "example.com",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"trustDomain": "example.com",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "trust.additionaldomains.empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Trust: v2.TrustConfig{
						AdditionalDomains: []string{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"trustDomainAliases": []interface{}{},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "trust.additionaldomains.nonempty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Trust: v2.TrustConfig{
						AdditionalDomains: []string{
							"extra-trust.mydomain.com",
							"another-trusted.anotherdomain.com",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"trustDomainAliases": []interface{}{
					"extra-trust.mydomain.com",
					"another-trusted.anotherdomain.com",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "trust.domain." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Trust: v2.TrustConfig{
						Domain: "example.com",
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"trustDomain": "example.com",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "trust.additionaldomains.empty." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Trust: v2.TrustConfig{
						AdditionalDomains: []string{},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"trustDomainAliases": []interface{}{},
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "trust.additionaldomains.nonempty." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					Trust: v2.TrustConfig{
						AdditionalDomains: []string{
							"extra-trust.mydomain.com",
							"another-trusted.anotherdomain.com",
						},
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"trustDomainAliases": []interface{}{
					"extra-trust.mydomain.com",
					"another-trusted.anotherdomain.com",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "controlplane.misc." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					ControlPlane: v2.ControlPlaneMTLSConfig{
						Enable: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"controlPlaneSecurityEnabled": true,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "controlplane.certprovider.istiod." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					ControlPlane: v2.ControlPlaneMTLSConfig{
						CertProvider: v2.ControlPlaneCertProviderTypeIstiod,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"pilotCertProvider": "istiod",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "controlplane.certprovider.kubernetes." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					ControlPlane: v2.ControlPlaneMTLSConfig{
						CertProvider: v2.ControlPlaneCertProviderTypeKubernetes,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"pilotCertProvider": "kubernetes",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "controlplane.certprovider.custom." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					ControlPlane: v2.ControlPlaneMTLSConfig{
						CertProvider: v2.ControlPlaneCertProviderTypeCustom,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"pilotCertProvider": "custom",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "controlplane.misc." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					ControlPlane: v2.ControlPlaneMTLSConfig{
						Enable: &featureEnabled,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"controlPlaneSecurityEnabled": true,
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "controlplane.certprovider.istiod." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					ControlPlane: v2.ControlPlaneMTLSConfig{
						CertProvider: v2.ControlPlaneCertProviderTypeIstiod,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"pilotCertProvider": "istiod",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "controlplane.certprovider.kubernetes." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					ControlPlane: v2.ControlPlaneMTLSConfig{
						CertProvider: v2.ControlPlaneCertProviderTypeKubernetes,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"pilotCertProvider": "kubernetes",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
	{
		name: "controlplane.certprovider.custom." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Security: &v2.SecurityConfig{
				MutualTLS: v2.MutualTLSConfig{
					ControlPlane: v2.ControlPlaneMTLSConfig{
						CertProvider: v2.ControlPlaneCertProviderTypeCustom,
					},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"pilotCertProvider": "custom",
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
			"istio_cni": map[string]interface{}{
				"enabled": true,
			},
		}),
	},
}

func TestSecurityConversionFromV2(t *testing.T) {
	for _, tc := range securityTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateSecurityValues(specCopy, helmValues.GetContent()); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			if !reflect.DeepEqual(tc.isolatedIstio.DeepCopy(), helmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.isolatedIstio.GetContent(), helmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), helmValues.GetContent())
			if err := populateSecurityConfig(helmValues.DeepCopy(), specv2); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			if !reflect.DeepEqual(tc.spec.Security, specv2.Security) {
				expected, _ := yaml.Marshal(tc.spec.Security)
				got, _ := yaml.Marshal(specv2.Security)
				t.Errorf("unexpected output converting values back to v2:\n\texpected:\n%s\n\tgot:\n%s", string(expected), string(got))
			}
		})
	}
}
