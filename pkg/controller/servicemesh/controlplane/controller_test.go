package controlplane

import (
	"reflect"
	"testing"

	"github.com/maistra/istio-operator/pkg/apis/maistra/v1"
)

type mergeTestCases struct {
	name           string
	base           map[string]interface{}
	input          map[string]interface{}
	expectedResult map[string]interface{}
}

var mergeTests = []mergeTestCases{
	{
		name: "input should not override base base",
		base: map[string]interface{}{
			"a": 1,
		},
		input: map[string]interface{}{
			"a": 2,
		},
		expectedResult: map[string]interface{}{
			"a": 1,
		},
	},
	{
		name: "maps should be merged",
		base: map[string]interface{}{
			"a": map[string]interface{}{
				"b": 1,
			},
		},
		input: map[string]interface{}{
			"a": map[string]interface{}{
				"c": 2,
			},
		},
		expectedResult: map[string]interface{}{
			"a": map[string]interface{}{
				"b": 1,
				"c": 2,
			},
		},
	},
	{
		name:           "nil values return empty map",
		base:           nil,
		input:          nil,
		expectedResult: map[string]interface{}{},
	},
	{
		name: "input on empty base returns input",
		base: nil,
		input: map[string]interface{}{
			"a": 3,
		},
		expectedResult: map[string]interface{}{
			"a": 3,
		},
	},
}

func TestGetSMCPTemplateWithSlashReturnsError(t *testing.T) {
	reconcileControlPlane := ReconcileControlPlane{}
	reconciler := reconcileControlPlane.getOrCreateReconciler(&v1.ServiceMeshControlPlane{})
	reconciler.Log = log.WithValues()

	_, err := reconciler.getSMCPTemplate("/")
	if err == nil {
		t.Fatalf("Allowed to access path outside of deployment directory")
	}
}

func TestMerge(t *testing.T) {
	for _, testCase := range mergeTests {
		t.Run(testCase.name, func(t *testing.T) {
			result := mergeValues(testCase.base, testCase.input)
			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Fatalf("test: %s expected: %+v got: %+v", testCase.name, testCase.expectedResult, result)
			}
		})
	}
}

func TestCyclicTemplate(t *testing.T) {
	reconcileControlPlane := ReconcileControlPlane{}
	reconciler := reconcileControlPlane.getOrCreateReconciler(&v1.ServiceMeshControlPlane{})
	reconciler.Log = log.WithValues()

	_, err := reconciler.renderSMCPTemplates(v1.ControlPlaneSpec{Template: "visited"}, map[string]struct{}{"visited": {}})
	if err == nil {
		t.Fatalf("Expected error to not be nil. Cyclic dependencies should not be allowed.")
	}
}
