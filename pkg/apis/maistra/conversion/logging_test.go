package conversion

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
	"sigs.k8s.io/yaml"
)

var loggingTestCases = []conversionTestCase{
	{
		name: "defaults." + versions.V2_0.String(),
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
		name: "empty." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Logging: &v2.LoggingConfig{},
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
		name: "all." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			Logging: &v2.LoggingConfig{
				ComponentLevels: v2.ComponentLogLevels{
					v2.EnvoyComponentAdmin:  v2.LogLevelDebug,
					v2.EnvoyComponentClient: v2.LogLevelTrace,
				},
				LogAsJSON: &featureEnabled,
			},
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{

				"logAsJson": true,
				"logging": map[string]interface{}{
					"level": "admin:debug,client:trace",
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
}

func TestLoggingConversionFromV2(t *testing.T) {
	for _, tc := range loggingTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateControlPlaneLogging(specCopy.Logging, helmValues.GetContent()); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			if !reflect.DeepEqual(tc.isolatedIstio.DeepCopy(), helmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.isolatedIstio.GetContent(), helmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), helmValues.GetContent())
			if err := populateControlPlaneLoggingConfig(helmValues.DeepCopy(), specv2); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			if !reflect.DeepEqual(tc.spec.Proxy, specv2.Proxy) {
				expected, _ := yaml.Marshal(tc.spec.Proxy)
				got, _ := yaml.Marshal(specv2.Proxy)
				t.Errorf("unexpected output converting values back to v2:\n\texpected:\n%s\n\tgot:\n%s", string(expected), string(got))
			}
		})
	}
}
