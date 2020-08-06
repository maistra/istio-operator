package conversion

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

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
)

type conversionTestCase struct {
	name          string
	spec          *v2.ControlPlaneSpec
	roundTripSpec *v2.ControlPlaneSpec
	isolatedIstio *v1.HelmValues
	completeIstio *v1.HelmValues
}

func assertEquals(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Logf("DeepEqual() failed, retrying after pruning empty/nil objects")
		prunedExpected := pruneEmptyObjects(expected)
		prunedActual := pruneEmptyObjects(actual)
		if !reflect.DeepEqual(prunedExpected, prunedActual) {
			expectedYAML, _ := yaml.Marshal(expected)
			actualYAML, _ := yaml.Marshal(actual)
			t.Errorf("unexpected output converting values back to v2:\n\texpected:\n%s\n\tgot:\n%s", string(expectedYAML), string(actualYAML))
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
				Spec: *tc.spec.DeepCopy(),
			}

			if err := scheme.Convert(smcpv2, smcpv1, nil); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			istio := tc.isolatedIstio.DeepCopy().GetContent()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), istio)
			if !reflect.DeepEqual(istio, smcpv1.Spec.Istio.DeepCopy().GetContent()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", istio, smcpv1.Spec.Istio.GetContent())
			}
			newsmcpv2 := &v2.ServiceMeshControlPlane{}
			// use expected data
			smcpv1.Spec.Istio = v1.NewHelmValues(istio).DeepCopy()
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
