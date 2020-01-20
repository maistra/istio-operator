package controlplane

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newTestReconciler(client client.Client) *ReconcileControlPlane {
	return &ReconcileControlPlane{
		ResourceManager: common.ResourceManager{
			Client:       client,
			PatchFactory: common.NewPatchFactory(client),
			Log:          log,
		},
		reconcilers: map[string]*ControlPlaneReconciler{},
	}
}

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
	reconcileControlPlane := newTestReconciler(nil)
	reconciler := reconcileControlPlane.getOrCreateReconciler(&v1.ServiceMeshControlPlane{})
	reconciler.Log = log.WithValues()

	_, err := reconciler.getSMCPTemplate("/", common.DefaultMaistraVersion)
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
	reconcileControlPlane := newTestReconciler(nil)
	reconciler := reconcileControlPlane.getOrCreateReconciler(&v1.ServiceMeshControlPlane{})
	reconciler.Log = log.WithValues()

	_, err := reconciler.recursivelyApplyTemplates(v1.ControlPlaneSpec{Template: "visited"}, sets.NewString("visited"))
	if err == nil {
		t.Fatalf("Expected error to not be nil. Cyclic dependencies should not be allowed.")
	}
}
