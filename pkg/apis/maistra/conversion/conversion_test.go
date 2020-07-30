package conversion

import (
	"fmt"
	"reflect"
	"testing"

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
	isolatedIstio *v1.HelmValues
	completeIstio *v1.HelmValues
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

func runTestCasesFromV2(testCases []conversionTestCase, t *testing.T) {
	t.Helper()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			smcpv1 := &v1.ServiceMeshControlPlane{}
			smcpv2 := &v2.ServiceMeshControlPlane{
				Spec: *tc.spec.DeepCopy(),
			}
			if err := Convert_v2_ServiceMeshControlPlane_To_v1_ServiceMeshControlPlane(smcpv2, smcpv1, nil); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			istio := tc.isolatedIstio.DeepCopy().GetContent()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), istio)
			if !reflect.DeepEqual(istio, smcpv1.Spec.Istio.DeepCopy().GetContent()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", istio, smcpv1.Spec.Istio.GetContent())
			}
			newsmcpv2 := &v2.ServiceMeshControlPlane{}
			smcpv1 = smcpv1.DeepCopy()
			if err := Convert_v1_ServiceMeshControlPlane_To_v2_ServiceMeshControlPlane(smcpv1, newsmcpv2, nil); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			if !reflect.DeepEqual(smcpv2, newsmcpv2) {
				expected, _ := yaml.Marshal(smcpv2)
				got, _ := yaml.Marshal(newsmcpv2)
				t.Errorf("unexpected output converting values back to v2:\n\texpected:\n%s\n\tgot:\n%s", string(expected), string(got))
			}
		})
	}
}

func mergeMaps(source, target map[string]interface{}) {
	for key, val := range source {
		if targetvalue, ok := target[key]; ok {
			if targetmap, ok := targetvalue.(map[string]interface{}); ok {
				if valmap, ok := val.(map[string]interface{}); ok {
					mergeMaps(valmap, targetmap)
				} else {
					panic(fmt.Sprintf("can only merge map types: key=%v, value=:%v", key, targetvalue))
				}
			} else {
				panic(fmt.Sprintf("can only merge map types: key=%v, value=:%v", key, val))
			}
		} else {
			target[key] = val
		}
	}
}
