package conversion

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var discoverySelectorsTestCases []conversionDiscoverySelectorsTestCase

type conversionDiscoverySelectorsTestCase struct {
	name       string
	spec       *v2.ControlPlaneSpec
	helmValues string
}

func init() {
	for _, v := range versions.TestedVersions {
		if v.AtLeast(versions.V2_4) {
			discoverySelectorsTestCases = append(discoverySelectorsTestCases, discoverySelectorsTestCasesV2(v)...)
		}
	}
}

func TestDiscoverySelectorsConversionFromV2(t *testing.T) {
	for _, tc := range discoverySelectorsTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			actualHelmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateDiscoverySelectorsValues(specCopy, actualHelmValues.GetContent()); err != nil {
				t.Errorf("error converting to values: %s", err)
			}

			expectedHelmValues := v1.HelmValues{}
			if err := expectedHelmValues.UnmarshalYAML([]byte(tc.helmValues)); err != nil {
				t.Fatalf("failed to parse helm values: %s", err)
			}
			if !reflect.DeepEqual(expectedHelmValues.DeepCopy(), actualHelmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", expectedHelmValues.GetContent(), actualHelmValues.GetContent())
			}
			specv2 := v2.ControlPlaneSpec{}
			if err := populateDiscoverySelectorsConfig(expectedHelmValues.DeepCopy(), &specv2); err != nil {
				t.Errorf("error converting from values: %s", err)
			}
			assertEquals(t, tc.spec.MeshConfig, specv2.MeshConfig)
		})
	}
}

func discoverySelectorsTestCasesV2(version versions.Version) []conversionDiscoverySelectorsTestCase {
	ver := version.String()
	return []conversionDiscoverySelectorsTestCase{
		{
			name: "nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
			},
			helmValues: "{}",
		},
		{
			name: "empty." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					DiscoverySelectors: []*metav1.LabelSelector{},
				},
			},
			helmValues: `
meshConfig:
  discoverySelectors: []
`,
		},
		{
			name: "discoverySelectors." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					DiscoverySelectors: []*metav1.LabelSelector{
						{
							MatchLabels: map[string]string{
								"env":    "prod",
								"region": "us-east1",
							},
						},
						{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "app",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"cassandra", "spark"},
								},
							},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  discoverySelectors:
  - matchLabels:
      env: prod
      region: us-east1
  - matchExpressions:
    - key: app
      operator: In
      values:
        - cassandra
        - spark
`,
		},
	}
}
