package controlplane

import (
	"reflect"
	"testing"
	"time"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	maistra "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

const (
	controlPlaneName      = "my-mesh"
	controlPlaneNamespace = "cp-namespace"
	controlPlaneUID       = types.UID("2222")
)

var (
	request = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      controlPlaneName,
			Namespace: controlPlaneNamespace,
		},
	}

	oneMinuteAgo = meta.NewTime(time.Now().Add(-time.Minute))
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
	reconciler := reconcileControlPlane.getOrCreateReconciler(&maistra.ServiceMeshControlPlane{})
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
	reconciler := reconcileControlPlane.getOrCreateReconciler(&maistra.ServiceMeshControlPlane{})
	reconciler.Log = log.WithValues()

	_, err := reconciler.recursivelyApplyTemplates(maistra.ControlPlaneSpec{Template: "visited"}, sets.NewString("visited"))
	if err == nil {
		t.Fatalf("Expected error to not be nil. Cyclic dependencies should not be allowed.")
	}
}

func TestReconcileAddsFinalizer(t *testing.T) {
	roll := newControlPlane()
	roll.Finalizers = []string{}

	cl, _, _, r := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(cl, roll.ObjectMeta, &maistra.ServiceMeshControlPlane{}).(*maistra.ServiceMeshControlPlane)
	assert.DeepEquals(updatedRoll.GetFinalizers(), []string{common.FinalizerName}, "Unexpected finalizers in SMM", t)
}

func TestReconcileFailsIfItCannotAddFinalizer(t *testing.T) {
	roll := newControlPlane()
	roll.Finalizers = []string{}

	_, tracker, _, r := createClientAndReconciler(t, roll)
	tracker.AddReactor(test.ClientFailsOn("update", "servicemeshcontrolplanes"))
	assertReconcileFails(r, t)
}

func TestReconcileDoesNothingIfMemberRollIsDeletedAndHasNoFinalizers(t *testing.T) {
	roll := newControlPlane()
	roll.DeletionTimestamp = &oneMinuteAgo
	roll.Finalizers = nil

	_, tracker, _, r := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingWhenMemberRollIsNotFound(t *testing.T) {
	_, tracker, _, r := createClientAndReconciler(t)
	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileFailsWhenGetMemberRollFails(t *testing.T) {
	_, tracker, _, r := createClientAndReconciler(t)
	tracker.AddReactor(test.ClientFailsOn("get", "servicemeshcontrolplanes"))
	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func createClientAndReconciler(t *testing.T, clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, record.EventRecorder, *ReconcileControlPlane) {
	cl, enhancedTracker := test.CreateClient(clientObjects...)
	fakeEventRecorder := &record.FakeRecorder{}

	r := &ReconcileControlPlane{
		ResourceManager: common.ResourceManager{Client: cl, PatchFactory: common.NewPatchFactory(cl), Log: log},
		Scheme:          scheme.Scheme,
		EventRecorder:   fakeEventRecorder,
	}

	return cl, enhancedTracker, fakeEventRecorder, r
}

func assertReconcileSucceeds(r *ReconcileControlPlane, t *testing.T) {
	res, err := r.Reconcile(request)
	if err != nil {
		log.Error(err, "Reconcile failed")
		t.Fatalf("Reconcile failed: %v", err)
	}
	if res.Requeue {
		t.Error("Reconcile requeued the request, but it shouldn't have")
	}
}

func assertReconcileFails(r *ReconcileControlPlane, t *testing.T) {
	_, err := r.Reconcile(request)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}
}

func newControlPlane() *maistra.ServiceMeshControlPlane {
	return &maistra.ServiceMeshControlPlane{
		ObjectMeta: meta.ObjectMeta{
			Name:       controlPlaneName,
			Namespace:  controlPlaneNamespace,
			Finalizers: []string{common.FinalizerName},
			Generation: 1,
			UID:        controlPlaneUID,
		},
		Spec:   maistra.ControlPlaneSpec{},
		Status: maistra.ControlPlaneStatus{},
	}
}
