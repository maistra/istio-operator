package controlplane

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

func TestGetSMCPTemplateWithSlashReturnsError(t *testing.T) {
	instanceReconciler := newTestReconciler()
	_, err := instanceReconciler.getSMCPTemplate("/", common.DefaultMaistraVersion)
	if err == nil {
		t.Fatalf("Allowed to access path outside of deployment directory")
	}
}

func TestMerge(t *testing.T) {
	var testCases = []struct {
		name           string
		base           map[string]interface{}
		input          map[string]interface{}
		expectedResult map[string]interface{}
	}{
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mergeValues(tc.base, tc.input)
			if !reflect.DeepEqual(result, tc.expectedResult) {
				t.Fatalf("test: %s expected: %+v got: %+v", tc.name, tc.expectedResult, result)
			}
		})
	}
}

func TestCyclicTemplate(t *testing.T) {
	instanceReconciler := newTestReconciler()
	_, err := instanceReconciler.recursivelyApplyTemplates(maistrav1.ControlPlaneSpec{Template: "visited"}, "", sets.NewString("visited"))
	if err == nil {
		t.Fatalf("Expected error to not be nil. Cyclic dependencies should not be allowed.")
	}
}

func newTestReconciler() *controlPlaneInstanceReconciler {
	reconciler := NewControlPlaneInstanceReconciler(
		common.ControllerResources{Log: logf.Log.WithName("instanceReconciler")},
		&maistrav1.ServiceMeshControlPlane{})
	return reconciler.(*controlPlaneInstanceReconciler)
}
