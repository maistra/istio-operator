package conversion

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	v1 "github.com/maistra/istio-operator/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/apis/maistra/v2"
	"github.com/maistra/istio-operator/controllers/versions"
)

var policyTestCases []conversionTestCase

// Deprecated v1.1 is deprecated and skip TestCasesV1

func policyTestCasesV2(version versions.Version) []conversionTestCase {
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
				Version: ver,
				Policy:  &v2.PolicyConfig{},
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
				Policy: &v2.PolicyConfig{
					Type: v2.PolicyTypeNone,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"policy": map[string]interface{}{
						"enabled": false,
					},
				},
				"policy": map[string]interface{}{
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
			name: "istiod.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Policy: &v2.PolicyConfig{
					Type: v2.PolicyTypeIstiod,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"policy": map[string]interface{}{
						"enabled": false,
					},
				},
				"policy": map[string]interface{}{
					"implementation": "Istiod",
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
			name: "mixer.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Policy: &v2.PolicyConfig{
					Type: v2.PolicyTypeMixer,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"policy": map[string]interface{}{
						"enabled": true,
					},
				},
				"policy": map[string]interface{}{
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
			name: "mixer.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Policy: &v2.PolicyConfig{
					Type:  v2.PolicyTypeMixer,
					Mixer: &v2.MixerPolicyConfig{},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"policy": map[string]interface{}{
						"enabled": true,
					},
				},
				"policy": map[string]interface{}{
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
			name: "mixer.misc." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Policy: &v2.PolicyConfig{
					Type: v2.PolicyTypeMixer,
					Mixer: &v2.MixerPolicyConfig{
						EnableChecks:    &featureEnabled,
						FailOpen:        &featureDisabled,
						SessionAffinity: &featureEnabled,
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"disablePolicyChecks": false,
					"policyCheckFailOpen": false,
				},
				"mixer": map[string]interface{}{
					"policy": map[string]interface{}{
						"enabled":                true,
						"sessionAffinityEnabled": true,
					},
				},
				"policy": map[string]interface{}{
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
			name: "mixer.adapters.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Policy: &v2.PolicyConfig{
					Type: v2.PolicyTypeMixer,
					Mixer: &v2.MixerPolicyConfig{
						Adapters: &v2.MixerPolicyAdaptersConfig{},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"policy": map[string]interface{}{
						"enabled": true,
					},
				},
				"policy": map[string]interface{}{
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
			name: "mixer.adapters." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Policy: &v2.PolicyConfig{
					Type: v2.PolicyTypeMixer,
					Mixer: &v2.MixerPolicyConfig{
						Adapters: &v2.MixerPolicyAdaptersConfig{
							KubernetesEnv:  &featureEnabled,
							UseAdapterCRDs: &featureDisabled,
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"policy": map[string]interface{}{
						"enabled": true,
						"adapters": map[string]interface{}{
							"kubernetesenv": map[string]interface{}{
								"enabled": true,
							},
							"useAdapterCRDs": false,
						},
					},
				},
				"policy": map[string]interface{}{
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
			name: "remote.nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Policy: &v2.PolicyConfig{
					Type: v2.PolicyTypeRemote,
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"policy": map[string]interface{}{
						"enabled": false,
					},
				},
				"policy": map[string]interface{}{
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
			name: "remote.defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Policy: &v2.PolicyConfig{
					Type:   v2.PolicyTypeRemote,
					Remote: &v2.RemotePolicyConfig{},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"mixer": map[string]interface{}{
					"policy": map[string]interface{}{
						"enabled": false,
					},
				},
				"policy": map[string]interface{}{
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
			name: "remote.full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Policy: &v2.PolicyConfig{
					Type: v2.PolicyTypeRemote,
					Remote: &v2.RemotePolicyConfig{
						Address:       "mixer-policy.some-namespace.svc.cluster.local",
						CreateService: &featureEnabled,
						EnableChecks:  &featureEnabled,
						FailOpen:      &featureDisabled,
					},
				},
			},
			roundTripSpec: &v2.ControlPlaneSpec{
				Version: ver,
				Policy: &v2.PolicyConfig{
					Type: v2.PolicyTypeRemote,
					Remote: &v2.RemotePolicyConfig{
						Address:       "mixer-policy.some-namespace.svc.cluster.local",
						CreateService: &featureEnabled,
						EnableChecks:  &featureEnabled,
						FailOpen:      &featureDisabled,
					},
				},
				Telemetry: &v2.TelemetryConfig{
					Remote: &v2.RemoteTelemetryConfig{
						CreateService: &featureEnabled,
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"createRemoteSvcEndpoints": true,
					"remotePolicyAddress":      "mixer-policy.some-namespace.svc.cluster.local",
					"disablePolicyChecks":      false,
					"policyCheckFailOpen":      false,
				},
				"mixer": map[string]interface{}{
					"policy": map[string]interface{}{
						"enabled": false,
					},
				},
				"policy": map[string]interface{}{
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
}

func init() {
	// v1.1 is deprecated and skip policyTestCasesV1
	for _, v := range versions.AllV2Versions {
		policyTestCases = append(policyTestCases, policyTestCasesV2(v)...)
	}
}

func TestPolicyConversionFromV2(t *testing.T) {
	for _, tc := range policyTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populatePolicyValues(specCopy, helmValues.GetContent()); err != nil {
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
				if err := populatePolicyConfig(helmValues.DeepCopy(), specv2, version); err != nil {
					t.Fatalf("error converting from values: %s", err)
				}
			} else {
				t.Fatalf("error parsing version: %s", err)
			}
			assertEquals(t, tc.spec.Policy, specv2.Policy)
		})
	}
}
