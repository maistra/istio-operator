package conversion

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

const (
	testcert = `
-----BEGIN CERTIFICATE-----
MIIFgTCCA2mgAwIBAgIUZzX0jMIVIU3a75olJeNv8qEnMAcwDQYJKoZIhvcNAQEL
BQAwUDELMAkGA1UEBhMCVVMxDzANBgNVBAgMBkRlbmlhbDEOMAwGA1UEBwwFRXRo
ZXIxDDAKBgNVBAoMA0RpczESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTE5MDkxOTEz
NDc0MFoXDTI5MDkxNjEzNDc0MFowUDELMAkGA1UEBhMCVVMxDzANBgNVBAgMBkRl
bmlhbDEOMAwGA1UEBwwFRXRoZXIxDDAKBgNVBAoMA0RpczESMBAGA1UEAwwJbG9j
YWxob3N0MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA3lX/t3mQ//Hf
wdhmwpFvL0eNFxQGlDgPs72Puh+VWDuSs+okSo3pcvQEv+q2rlqGlx81w2owuoFz
8fASBQ3jFH6HPbA50gkBU87RQ8czcn7ASypybukC1aa1o4yhwCqrOudPdtcvqSx9
FS4VA8l1o3QBTIXnTyz68338l/ZNdcdEyQoNwdkmI4qPOxx5IhgQeiuLGVcxnVzw
eBGIPIVuxX8tuSWdu4UKjswHpok3w/PrphTpPsXIWChdlex840namnfIb7B3iYEZ
EDOkKA/VkjImdxEHq9FktjSkdshXh4TcJAjbL2IuJ/V/JyzmXPndIMDvu8tQgdkI
atAgfL8Ah2VkXFbVSqoXkgoSVSQtDBGyBIp2ZOISOCtKeb55Gy/PfpjL6dsr20UE
TS6nDJ9o5Rk7CIobniGxPEoXYYma2ZHXNYraR5KwUGaz4bDpBZll6UtQKifQMp1t
M1bBB6Y2ubjkbevdjkM/le668vIvr31GlkXVXiJrycumpTpgyIJh62yOL9HZTiio
U+qpqUJM2qUBUS0JBddQoNMvbNBBVlP3ZKinrywldtyqJ24Kwml18a/dv3Cxxdjt
wubgdeMV3wPISnnkoArO8jcrjSX3U2spcLDi2W6x11Xg+KJcy4sS3cI2SE4gTrBE
YgyE+awhduu4awknbPPYgL8DXGvo8PsCAwEAAaNTMFEwHQYDVR0OBBYEFB0VZJU5
SznwI4m7beqJPNP+SFHyMB8GA1UdIwQYMBaAFB0VZJU5SznwI4m7beqJPNP+SFHy
MA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggIBAAdOWfGS3UjAucdw
U9/Qr78nKyC4YJJn1keFxzf2tJQOAa6k57Ln6fbJTNoMEwnCPWUBR44kCfQMEOup
TNG4AEvJfHroCIgAuuPZ0AX75TBFB0UU9gnl2g8eou78+6tAw+r+KVcMAr/yvQYs
fCpM7k+rte8rCuIXSb9m9kJxKRNkILRSGhyhCfBDUi0iXqjKYI+Utetg3pe8Vrkl
rhYe6o2sGLkf8VbawC2ZNBqm1ehhjVIDDGcGAz6RcscqIR+f7MLy3oUNypiIoJ3M
yVwTCn34vbIViDcu9yebgrt0ZOq3AyFgtCKCHswO21BM97NelbZ7wdJNJXOXhzuW
aqlciI+5kj3sYwbFq3eNTZuFad+AJvCEO5xLusAl0WZENMJ1TLDymSd0yi8pEtu0
GtbtGLsMrHVhfqW8XWmqiCBUe6nP2W1OgjJt0jGNsiQ3IbK3/M5Fk5iM5kNTtKZY
h9/bp+//GG/VUYpAP9J1/uunf2L+z0XLqwN1+++8RQgUqtN9B9Qe8FgQ+eiUmmyC
xj7nwwfaTzoRw8MIpj17bz/pkDvTGOiDrc5woaEePV7gyfLGrK5dHN+E27Xs8ZCx
iWBfESF8eP/PdF9JG3g7+5Bmz0yuiZErqWp17VH3o/gb2BSWK3MfO6UYYVnCNpok
nFnKhRHgeHQiN8p4DaXm4BDNUVnU
-----END CERTIFICATE-----
`
)

var securityTestCases []conversionTestCase

// Deprecated v1.1 is deprecated and skip TestCasesV1

func securityTestCasesV2(version versions.Version) []conversionTestCase {
	ver := version.String()
	var trustDomainTestCase conversionTestCase
	if version.Version() < versions.V2_2 {
		trustDomainTestCase = conversionTestCase{
			name: "trust.domain." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					Trust: &v2.TrustConfig{
						Domain: "example.com",
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		}
	} else {
		trustDomainTestCase = conversionTestCase{
			name: "trust.domain." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					Trust: &v2.TrustConfig{
						Domain: "example.com",
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"meshConfig": map[string]interface{}{
					"trustDomain": "example.com",
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		}
	}

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
				Version:  ver,
				Security: &v2.SecurityConfig{},
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
			name: "misc." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					DataPlane: &v2.DataPlaneSecurityConfig{
						MTLS:     &featureEnabled,
						AutoMTLS: &featureEnabled,
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
				"meshConfig": map[string]interface{}{
					"enableAutoMtls": true,
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
			name: "networkpolicy.enabled." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					ManageNetworkPolicy: &featureEnabled,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"manageNetworkPolicy": true,
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"manageNetworkPolicy": true,
					"multiCluster":        globalMultiClusterDefaults,
					"meshExpansion":       globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "networkpolicy.disabled." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					ManageNetworkPolicy: &featureDisabled,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"manageNetworkPolicy": false,
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"manageNetworkPolicy": false,
					"multiCluster":        globalMultiClusterDefaults,
					"meshExpansion":       globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "security.jwksResolverCA.empty." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					JwksResolverCA: "",
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
			name: "security.jwksResolverCA.nonempty." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					JwksResolverCA: testcert,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"pilot": map[string]interface{}{
					"jwksResolverExtraRootCA": testcert,
				},
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"pilot": map[string]interface{}{
					"jwksResolverExtraRootCA": testcert,
				},
			}),
		},
		{
			name: "ca.istiod.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "ca.istiod.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
						Type:   v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{},
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "ca.istiod.common." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							WorkloadCertTTLDefault: "24h",
							WorkloadCertTTLMax:     "7d",
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"pilot": map[string]interface{}{
					"env": map[string]interface{}{
						"DEFAULT_WORKLOAD_CERT_TTL": "24h",
						"MAX_WORKLOAD_CERT_TTL":     "7d",
					},
				},
			}),
		},
		{
			name: "ca.istiod.selfsigned.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type: v2.IstioCertificateSignerTypeSelfSigned,
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "ca.istiod.selfsigned.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type:       v2.IstioCertificateSignerTypeSelfSigned,
							SelfSigned: &v2.IstioSelfSignedCertificateSignerConfig{},
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "ca.istiod.selfsigned.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"pilot": map[string]interface{}{
					"env": map[string]interface{}{
						"CITADEL_ENABLE_JITTER_FOR_ROOT_CERT_ROTATOR":           "true",
						"CITADEL_SELF_SIGNED_CA_CERT_TTL":                       "1y",
						"CITADEL_SELF_SIGNED_ROOT_CERT_CHECK_INTERVAL":          "1h",
						"CITADEL_SELF_SIGNED_ROOT_CERT_GRACE_PERIOD_PERCENTILE": "20%",
					},
				},
			}),
		},
		{
			name: "ca.istiod.privatekey.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type: v2.IstioCertificateSignerTypePrivateKey,
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "ca.istiod.privatekey.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeIstiod,
						Istiod: &v2.IstiodCertificateAuthorityConfig{
							Type:       v2.IstioCertificateSignerTypePrivateKey,
							PrivateKey: &v2.IstioPrivateKeyCertificateSignerConfig{},
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "ca.istiod.privatekey.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"pilot": map[string]interface{}{
					"env": map[string]interface{}{
						"ROOT_CA_DIR": "/etc/cacerts",
					},
				},
			}),
		},
		{
			name: "ca.custom.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeCustom,
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "ca.custom.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
						Type:   v2.CertificateAuthorityTypeCustom,
						Custom: &v2.CustomCertificateAuthorityConfig{},
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "ca.custom.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					CertificateAuthority: &v2.CertificateAuthorityConfig{
						Type: v2.CertificateAuthorityTypeCustom,
						Custom: &v2.CustomCertificateAuthorityConfig{
							Address: "my-caprovider.example.com",
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "identity.kubernetes." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					Identity: &v2.IdentityConfig{
						Type: v2.IdentityConfigTypeKubernetes,
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "identity.thirdparty.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					Identity: &v2.IdentityConfig{
						Type: v2.IdentityConfigTypeThirdParty,
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "identity.thirdparty.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					Identity: &v2.IdentityConfig{
						Type:       v2.IdentityConfigTypeThirdParty,
						ThirdParty: &v2.ThirdPartyIdentityConfig{},
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "identity.thirdparty.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					Identity: &v2.IdentityConfig{
						Type: v2.IdentityConfigTypeThirdParty,
						ThirdParty: &v2.ThirdPartyIdentityConfig{
							Audience: "istio-ca",
							Issuer:   "https://my-issuer.example.com",
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
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
				"pilot": map[string]interface{}{
					"env": map[string]interface{}{
						"TOKEN_ISSUER": "https://my-issuer.example.com",
					},
				},
			}),
		},
		trustDomainTestCase,
		{
			name: "trust.additionaldomains.empty." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					Trust: &v2.TrustConfig{
						AdditionalDomains: []string{},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"trustDomainAliases": []interface{}{},
				},
				"meshConfig": map[string]interface{}{
					"trustDomainAliases": []interface{}{},
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
			name: "trust.additionaldomains.nonempty." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					Trust: &v2.TrustConfig{
						AdditionalDomains: []string{
							"extra-trust.mydomain.com",
							"another-trusted.anotherdomain.com",
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
				"meshConfig": map[string]interface{}{
					"trustDomainAliases": []interface{}{
						"extra-trust.mydomain.com",
						"another-trusted.anotherdomain.com",
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
			name: "controlplane.misc." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					ControlPlane: &v2.ControlPlaneSecurityConfig{
						MTLS: &featureEnabled,
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "controlplane.certprovider.istiod." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					ControlPlane: &v2.ControlPlaneSecurityConfig{
						CertProvider: v2.ControlPlaneCertProviderTypeIstiod,
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "controlplane.certprovider.kubernetes." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					ControlPlane: &v2.ControlPlaneSecurityConfig{
						CertProvider: v2.ControlPlaneCertProviderTypeKubernetes,
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "controlplane.certprovider.custom." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					ControlPlane: &v2.ControlPlaneSecurityConfig{
						CertProvider: v2.ControlPlaneCertProviderTypeCustom,
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
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "controlplane.tls." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Security: &v2.SecurityConfig{
					ControlPlane: &v2.ControlPlaneSecurityConfig{
						TLS: &v2.ControlPlaneTLSConfig{
							CipherSuites: []string{
								"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
								"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
							},
							ECDHCurves: []string{
								"CurveP256",
								"CurveP521",
							},
							MinProtocolVersion: "TLSv1_2",
							MaxProtocolVersion: "TLSv1_3",
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"tls": map[string]interface{}{
						"cipherSuites":       "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
						"ecdhCurves":         "CurveP256,CurveP521",
						"minProtocolVersion": "TLSv1_2",
						"maxProtocolVersion": "TLSv1_3",
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
	// v1.1 is deprecated and skip TestCasesV1
	for _, v := range versions.TestedVersions {
		securityTestCases = append(securityTestCases, securityTestCasesV2(v)...)
	}
}

func TestSecurityConversionFromV2(t *testing.T) {
	for _, tc := range securityTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			version, err := versions.ParseVersion(tc.spec.Version)
			if err != nil {
				t.Fatalf("error parsing version: %s", err)
			}

			if err := populateSecurityValues(specCopy, helmValues.GetContent(), version); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			if !reflect.DeepEqual(tc.isolatedIstio.DeepCopy(), helmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.isolatedIstio.GetContent(), helmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), helmValues.GetContent())
			if err := populateSecurityConfig(helmValues.DeepCopy(), specv2, version); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			assertEquals(t, tc.spec.Security, specv2.Security)
		})
	}
}

type conversionFromV2TestCase struct {
	name               string
	spec               *v2.ControlPlaneSpec
	expectedHelmValues v1.HelmValues
}

var conversionFromV2SecurityTestCases = []conversionFromV2TestCase{
	{
		name: "ca.cert-manager.v2_3",
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_3.String(),
			Security: &v2.SecurityConfig{
				CertificateAuthority: &v2.CertificateAuthorityConfig{
					Type: v2.CertificateAuthorityTypeCertManager,
					CertManager: &v2.CertManagerCertificateAuthorityConfig{
						Address: "my-istio-csr.namespace.svc.cluster.local",
					},
				},
			},
		},
		expectedHelmValues: buildHelmValues(`
global:
  caAddress: my-istio-csr.namespace.svc.cluster.local
pilot:
  ca:
    implementation: cert-manager
  env:
    ENABLE_CA_SERVER: "false"
  extraArgs:
  - "--tlsCertFile=/etc/cert-manager/tls/tls.crt"
  - "--tlsKeyFile=/etc/cert-manager/tls/tls.key"
  - "--caCertFile=/etc/cert-manager/ca/root-cert.pem"
  extraVolumeMounts:
  - name: cert-manager
    mountPath: /etc/cert-manager/tls
    readOnly: true
  - name: ca-root-cert
    mountPath: /etc/cert-manager/ca
    readOnly: true
  extraVolumes:
  - name: cert-manager
    secret:
      secretName: istiod-tls
  - name: ca-root-cert
    configMap:
      name: istio-ca-root-cert
`),
	},
	{
		name: "ca.cert-manager.v2_4",
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_4.String(),
			Security: &v2.SecurityConfig{
				CertificateAuthority: &v2.CertificateAuthorityConfig{
					Type: v2.CertificateAuthorityTypeCertManager,
					CertManager: &v2.CertManagerCertificateAuthorityConfig{
						Address: "my-istio-csr.namespace.svc.cluster.local",
					},
				},
			},
		},
		expectedHelmValues: buildHelmValues(`
global:
  caAddress: my-istio-csr.namespace.svc.cluster.local
pilot:
  ca:
    implementation: cert-manager
  env:
    ENABLE_CA_SERVER: "false"
`),
	},
}

func TestSecurityConversionFromV2ToV1(t *testing.T) {
	for _, tc := range conversionFromV2SecurityTestCases {
		t.Run(tc.name+"-v2_to_v1", func(t *testing.T) {
			var specV1 v1.ControlPlaneSpec
			if err := Convert_v2_ControlPlaneSpec_To_v1_ControlPlaneSpec(tc.spec.DeepCopy(), &specV1, nil); err != nil {
				t.Errorf("failed to convert SMCP v2 to v1: %s", err)
			}

			if !reflect.DeepEqual(tc.expectedHelmValues.DeepCopy(), specV1.Istio.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.expectedHelmValues.GetContent(), specV1.Istio.GetContent())
			}
		})

		t.Run(tc.name+"-v1_to_v2", func(t *testing.T) {
			specV1 := v1.ControlPlaneSpec{
				Istio: tc.expectedHelmValues.DeepCopy(),
			}
			specV2 := v2.ControlPlaneSpec{}
			if err := Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(&specV1, &specV2, nil); err != nil {
				t.Errorf("failed to convert SMCP v1 to v2: %s", err)
			}

			assertEquals(t, tc.spec.Security, specV2.Security)
		})
	}
}
