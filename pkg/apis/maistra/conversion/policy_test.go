package conversion

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var policyTestCases = []conversionTestCase{
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
			Version: versions.V2_0.String(),
			Policy:  &v2.PolicyConfig{},
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
			Version: versions.V1_1.String(),
			Policy:  &v2.PolicyConfig{},
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
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeIstiod,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": false,
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
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
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
			Policy: &v2.PolicyConfig{
				Type:  v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
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
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					EnableChecks: &featureEnabled,
					FailOpen:     &featureDisabled,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"disablePolicyChecks": false,
				"policyCheckFailOpen": false,
			},
			"mixer": map[string]interface{}{
				"enabled": true,
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
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					Adapters: &v2.MixerPolicyAdaptersConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
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
		name: "mixer.adapters." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
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
				"enabled": true,
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
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
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
			Policy: &v2.PolicyConfig{
				Type:  v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
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
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					EnableChecks: &featureEnabled,
					FailOpen:     &featureDisabled,
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"disablePolicyChecks": false,
				"policyCheckFailOpen": false,
			},
			"mixer": map[string]interface{}{
				"enabled": true,
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
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeMixer,
				Mixer: &v2.MixerPolicyConfig{
					Adapters: &v2.MixerPolicyAdaptersConfig{},
				},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"mixer": map[string]interface{}{
				"enabled": true,
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
		name: "mixer.adapters." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
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
				"enabled": true,
				"adapters": map[string]interface{}{
					"kubernetesenv": map[string]interface{}{
						"enabled": true,
					},
					"useAdapterCRDs": false,
				},
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
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remotePolicyAddress":      "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
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
		name: "remote.defaults." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type:   v2.PolicyTypeRemote,
				Remote: &v2.RemotePolicyConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remotePolicyAddress":      "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
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
		name: "remote.full." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
				Remote: &v2.RemotePolicyConfig{
					Address:       "mixer-policy.some-namespace.svc.cluster.local",
					CreateService: true,
					EnableChecks:  &featureEnabled,
					FailOpen:      &featureDisabled,
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
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
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
		name: "remote.nil." + versions.V1_1.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V1_1.String(),
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remotePolicyAddress":      "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
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
			Policy: &v2.PolicyConfig{
				Type:   v2.PolicyTypeRemote,
				Remote: &v2.RemotePolicyConfig{},
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"createRemoteSvcEndpoints": false,
				"remotePolicyAddress":      "",
			},
			"mixer": map[string]interface{}{
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
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
			Policy: &v2.PolicyConfig{
				Type: v2.PolicyTypeRemote,
				Remote: &v2.RemotePolicyConfig{
					Address:       "mixer-policy.some-namespace.svc.cluster.local",
					CreateService: true,
					EnableChecks:  &featureEnabled,
					FailOpen:      &featureDisabled,
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
				"enabled": false,
				"policy": map[string]interface{}{
					"enabled": true,
				},
			},
			"policy": map[string]interface{}{
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

func TestPolicyConversionFromV2(t *testing.T) {
	for _, tc := range policyTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populatePolicyValues(specCopy, helmValues.GetContent()); err != nil {
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
