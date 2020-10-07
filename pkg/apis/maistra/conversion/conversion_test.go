package conversion

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

var (
	featureEnabled   = true
	featureDisabled  = false
	replicaCount1    = int32(1)
	replicaCount2    = int32(2)
	replicaCount5    = int32(5)
	cpuUtilization80 = int32(80)
	intStrInt1       = intstr.FromInt(1)
	intStr25Percent  = intstr.FromString("25%")

	globalMultiClusterDefaults = map[string]interface{}{
		"enabled": false,
		"multiClusterOverrides": map[string]interface{}{
			"expansionEnabled":    nil,
			"multiClusterEnabled": nil,
		},
	}
	globalMeshExpansionDefaults = map[string]interface{}{
		"enabled": false,
		"useILB":  false,
	}

	roundTripTestCases = []struct {
		name   string
		smcpv1 v1.ControlPlaneSpec
		cruft  *v1.HelmValues
	}{
		{
			name: "simple",
			smcpv1: v1.ControlPlaneSpec{
				Version: "v1.1",
				Istio: v1.NewHelmValues(map[string]interface{}{
					"global": map[string]interface{}{
						"proxy": map[string]interface{}{
							"image": "asd",
						},
						"some-unmapped-field": map[string]interface{}{
							"foo":   "bar",
							"fooey": true,
						},
					},
					"prometheus": map[string]interface{}{
						"enabled": true,
					},
					"tracing": map[string]interface{}{
						"enabled": true,
					},
				}),
			},
			cruft: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					// a result of enabling tracing
					"enableTracing": true,
					// mesh expansion is disabled by default
					"meshExpansion": map[string]interface{}{
						"enabled": false,
						"useILB":  false,
					},
					// multicluster is disabled by default
					"multiCluster": map[string]interface{}{
						"enabled": false,
						"multiClusterOverrides": map[string]interface{}{
							"expansionEnabled":    nil,
							"multiClusterEnabled": nil,
						},
					},
					// a result of enabling tracing, default provider is jaeger
					"proxy": map[string]interface{}{
						"tracer": string("jaeger"),
					},
					"useMCP": true,
				},
				// a result of enabling prometheus
				"mixer": map[string]interface{}{
					"adapters": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				// a result of enabling prometheus
				"telemetry": map[string]interface{}{
					"v2": map[string]interface{}{
						"prometheus": map[string]interface{}{
							"enabled": true,
						},
					},
				},
				// a result of enabling tracing, default provider is jaeger
				"tracing": map[string]interface{}{
					"provider": string("jaeger"),
				},
			}),
		},
	}
)

type conversionTestCase struct {
	name               string
	namespace          string
	spec               *v2.ControlPlaneSpec
	roundTripSpec      *v2.ControlPlaneSpec
	isolatedIstio      *v1.HelmValues
	isolatedThreeScale *v1.HelmValues
	completeIstio      *v1.HelmValues
	completeThreeScale *v1.HelmValues
}

func assertEquals(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if diff := cmp.Diff(expected, actual, cmp.AllowUnexported(v1.HelmValues{})); diff != "" {
		t.Logf("DeepEqual() failed, retrying after pruning empty/nil objects: %s", diff)
		prunedExpected := pruneEmptyObjects(expected)
		prunedActual := pruneEmptyObjects(actual)
		if diff := cmp.Diff(prunedExpected, prunedActual, cmp.AllowUnexported(v1.HelmValues{})); diff != "" {
			t.Errorf("unexpected output converting values back to v2: %s", diff)
		}
	}

}
func pruneEmptyObjects(in interface{}) *v1.HelmValues {
	values, err := toValues(in)
	if err != nil {
		panic(fmt.Errorf("unexpected error converting value: %v", in))
	}
	pruneTree(values)
	return v1.NewHelmValues(values)
}

func pruneTree(in map[string]interface{}) {
	for restart := true; restart; {
		restart = false
		for key, rawValue := range in {
			switch value := rawValue.(type) {
			case []interface{}:
				if len(value) == 0 {
					delete(in, key)
				}
			case map[string]interface{}:
				pruneTree(value)
				if len(value) == 0 {
					delete(in, key)
				}
			}
		}
	}
}

func TestCompleteClusterConversionFromV2(t *testing.T) {
	runTestCasesFromV2(clusterTestCases, t)
}

func TestCompleteGatewaysConversionFromV2(t *testing.T) {
	runTestCasesFromV2(gatewaysTestCases, t)
}

func TestCompleteRuntimeConversionFromV2(t *testing.T) {
	runTestCasesFromV2(runtimeTestCases, t)
}

func TestCompleteProxyConversionFromV2(t *testing.T) {
	runTestCasesFromV2(proxyTestCases, t)
}

func TestCompleteLoggingConversionFromV2(t *testing.T) {
	runTestCasesFromV2(loggingTestCases, t)
}

func TestCompletePolicyConversionFromV2(t *testing.T) {
	runTestCasesFromV2(policyTestCases, t)
}

func TestCompleteTelemetryConversionFromV2(t *testing.T) {
	runTestCasesFromV2(telemetryTestCases, t)
}

func TestCompleteSecurityConversionFromV2(t *testing.T) {
	runTestCasesFromV2(securityTestCases, t)
}

func TestCompletePrometheusConversionFromV2(t *testing.T) {
	runTestCasesFromV2(prometheusTestCases, t)
}

func TestCompleteGrafanaConversionFromV2(t *testing.T) {
	runTestCasesFromV2(grafanaTestCases, t)
}

func TestCompleteKialiConversionFromV2(t *testing.T) {
	runTestCasesFromV2(kialiTestCases, t)
}

func TestCompleteJaegerConversionFromV2(t *testing.T) {
	runTestCasesFromV2(jaegerTestCases, t)
}

func TestCompleteThreeScaleConversionFromV2(t *testing.T) {
	runTestCasesFromV2(threeScaleTestCases, t)
}

func TestTechPreviewConversionFromV2(t *testing.T) {
	runTestCasesFromV2(techPreviewTestCases, t)
}

func TestRoundTripConversion(t *testing.T) {
	for _, tc := range roundTripTestCases {
		t.Run(tc.name, func(t *testing.T) {
			smcpv1 := *tc.smcpv1.DeepCopy()
			smcpv2 := v2.ControlPlaneSpec{}
			err := Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(&smcpv1, &smcpv2, nil)
			if err != nil {
				t.Fatalf("error converting smcpv1 to smcpv2: %v", err)
			}
			smcpv1 = v1.ControlPlaneSpec{}
			err = Convert_v2_ControlPlaneSpec_To_v1_ControlPlaneSpec(&smcpv2, &smcpv1, nil)
			if err != nil {
				t.Fatalf("error converting smcpv2 to smcpv1: %v", err)
			}
			if diff := cmp.Diff(tc.smcpv1, smcpv1, cmp.AllowUnexported(v1.HelmValues{})); diff != "" {
				t.Logf("TestRoundTripConversion() case %s mismatch, will try again after removing cruft (-want +got):\n%s", tc.name, diff)
				removeHelmValues(smcpv1.Istio.GetContent(), tc.cruft.GetContent())
				if diff := cmp.Diff(tc.smcpv1, smcpv1, cmp.AllowUnexported(v1.HelmValues{})); diff != "" {
					t.Errorf("TestRoundTripConversion() case %s mismatch (-want +got):\n%s", tc.name, diff)
				}
			}
		})
	}
}

func runTestCasesFromV2(testCases []conversionTestCase, t *testing.T) {
	scheme := runtime.NewScheme()
	v1.SchemeBuilder.AddToScheme(scheme)
	v2.SchemeBuilder.AddToScheme(scheme)
	localSchemeBuilder.AddToScheme(scheme)
	t.Helper()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			smcpv1 := &v1.ServiceMeshControlPlane{}
			smcpv2 := &v2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: tc.namespace,
				},
				Spec: *tc.spec.DeepCopy(),
			}

			if err := scheme.Convert(smcpv2, smcpv1, nil); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			istio := tc.isolatedIstio.DeepCopy().GetContent()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), istio)
			if diff := cmp.Diff(istio, smcpv1.Spec.Istio.DeepCopy().GetContent()); diff != "" {
				t.Errorf("unexpected output converting v2 to Istio values: %s", diff)
			}
			threeScale := tc.isolatedThreeScale.DeepCopy().GetContent()
			mergeMaps(tc.completeThreeScale.DeepCopy().GetContent(), threeScale)
			if diff := cmp.Diff(threeScale, smcpv1.Spec.ThreeScale.DeepCopy().GetContent()); diff != "" {
				t.Errorf("unexpected output converting v2 to ThreeScale values:%s", diff)
			}
			newsmcpv2 := &v2.ServiceMeshControlPlane{}
			// use expected data
			smcpv1.Spec.Istio = v1.NewHelmValues(istio).DeepCopy()
			smcpv1.Spec.ThreeScale = v1.NewHelmValues(threeScale).DeepCopy()
			if err := scheme.Convert(smcpv1, newsmcpv2, nil); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			assertEquals(t, smcpv2, newsmcpv2)
		})
	}
}

func mergeMaps(source, target map[string]interface{}) {
	for key, val := range source {
		if targetvalue, ok := target[key]; ok {
			if targetmap, ok := targetvalue.(map[string]interface{}); ok {
				if valmap, ok := val.(map[string]interface{}); ok {
					mergeMaps(valmap, targetmap)
					continue
				} else if valmap == nil {
					delete(target, key)
					continue
				} else {
					panic(fmt.Sprintf("trying to merge non-map into map: key=%v, value=:%v", key, val))
				}
			} else if _, ok := val.(map[string]interface{}); ok {
				panic(fmt.Sprintf("trying to merge map into non-map: key=%v, value=:%v", key, targetvalue))
			}
		}
		target[key] = val
	}
}
