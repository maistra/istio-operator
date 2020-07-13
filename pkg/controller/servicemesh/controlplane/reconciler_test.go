package controlplane

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func TestGetSMCPTemplateWithSlashReturnsError(t *testing.T) {
	instanceReconciler := newTestReconciler()
	_, err := instanceReconciler.getSMCPTemplate("/", versions.DefaultVersion)
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
	_, err := instanceReconciler.recursivelyApplyTemplates(ctx, maistrav1.ControlPlaneSpec{Template: "visited"}, versions.DefaultVersion, sets.NewString("visited"))
	if err == nil {
		t.Fatalf("Expected error to not be nil. Cyclic dependencies should not be allowed.")
	}
}

func TestInstallationErrorDoesNotUpdateLastTransitionTimeWhenNoStateTransitionOccurs(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.Spec.Template = "maistra"
	controlPlane.Status.SetCondition(status.Condition{
		Type:               status.ConditionTypeReconciled,
		Status:             status.ConditionStatusFalse,
		Reason:             "",
		Message:            "",
		LastTransitionTime: oneMinuteAgo,
	})

	cl, tracker, r := newReconcilerTestFixture(controlPlane)

	// make installation fail
	tracker.AddReactor("create", "deployments", test.ClientFails())

	// run initial reconcile to update the SMCP status
	assertInstanceReconcilerFails(r, t)

	// run reconcile again to work around the problem where the condition reason "InstallError" gets changed to "ReconcileError" in 2nd reconcile
	assertInstanceReconcilerFails(r, t)

	// remember the SMCP status at this point
	updatedControlPlane := &maistrav2.ServiceMeshControlPlane{}
	test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(controlPlane.ObjectMeta), updatedControlPlane))
	initialStatus := updatedControlPlane.Status.DeepCopy()

	// the resolution of lastTransitionTime is one second, so we need to wait at least one second before
	// performing another reconciliation to ensure that if the lastTransitionTime field is reset, the new
	// value will actually be different from the previous one
	time.Sleep(1 * time.Second)

	// run reconcile again to check if the status is still the same
	assertInstanceReconcilerFails(r, t)

	test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(controlPlane.ObjectMeta), updatedControlPlane))
	newStatus := &updatedControlPlane.Status

	marshal, _ := json.Marshal(initialStatus)
	fmt.Println(string(marshal))
	marshal, _ = json.Marshal(newStatus)
	fmt.Println(string(marshal))

	assert.DeepEquals(newStatus, initialStatus, "didn't expect SMCP status to be updated", t)
}

func assertInstanceReconcilerFails(r ControlPlaneInstanceReconciler, t *testing.T) {
	_, err := r.Reconcile(ctx)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}
}

func TestParallelInstallationOfCharts(t *testing.T) {
	var testCases = []struct {
		name                       string
		reactorsForFirstReconcile  []clienttesting.Reactor
		expectFirstReconcileToFail bool
		base                       map[string]interface{}
		input                      map[string]interface{}
		expectedResult             map[string]interface{}
	}{
		{
			name: "normal-case",
		},
		{
			name: "process-component-manifests-fails",
			reactorsForFirstReconcile: []clienttesting.Reactor{
				&clienttesting.SimpleReactor{Verb: "create", Resource: "deployments", Reaction: func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					create := action.(clienttesting.CreateAction)
					deploy := create.GetObject().(*unstructured.Unstructured)
					if deploy.GetName() == "istio-citadel" {
						return test.ClientFails()(action)
					}
					return false, nil, nil
				}},
			},
			expectFirstReconcileToFail: true,
		},
		{
			name: "calculate-readiness-fails",
			reactorsForFirstReconcile: []clienttesting.Reactor{
				&clienttesting.SimpleReactor{Verb: "list", Resource: "deployments", Reaction: test.ClientFails()},
			},
			expectFirstReconcileToFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			originalOrderedCharts := orderedCharts
			defer func() { orderedCharts = originalOrderedCharts }()

			orderedCharts = [][]string{
				{"istio"},
				{"istio/charts/security", "istio/charts/galley"}, // both are to be deployed at the same time
				{"istio/charts/prometheus"},
			}

			smcp := newControlPlane()
			smcp.Spec.Template = "maistra"

			cl, tracker, r := newReconcilerTestFixture(smcp)

			// run initial reconcile to initialize reconcile status
			assertInstanceReconcilerSucceeds(r, t)

			if tc.reactorsForFirstReconcile != nil {
				tracker.AddReaction(tc.reactorsForFirstReconcile...)

				if tc.expectFirstReconcileToFail {
					// first reconcile should fail
					_, err := r.Reconcile(ctx)
					assert.Failure(err, "Reconcile", t)
				} else {
					assertInstanceReconcilerSucceeds(r, t)
				}

				// we remove any reactors that cause failure
				tracker.RemoveReaction(tc.reactorsForFirstReconcile...)
			}

			// this reconcile must succeed
			assertInstanceReconcilerSucceeds(r, t)

			// check that both galley and citadel deployments have been created
			galleyDeployment := assertDeploymentExists(cl, "istio-galley", t)
			citadelDeployment := assertDeploymentExists(cl, "istio-citadel", t)

			// check if reconciledCondition indicates installation is paused and both galley and security are mentioned
			assertReconciledConditionMatches(cl, smcp, status.ConditionReasonPausingInstall, "[galley security]", t)

			markDeploymentAvailable(cl, galleyDeployment)

			// run reconcile again to see if the Reconciled condition is updated
			assertInstanceReconcilerSucceeds(r, t)
			assertReconciledConditionMatches(cl, smcp, status.ConditionReasonPausingInstall, "[security]", t)

			markDeploymentAvailable(cl, citadelDeployment)

			// run reconcile again to see if the Reconciled condition is updated
			assertInstanceReconcilerSucceeds(r, t)
			assertDeploymentExists(cl, "prometheus", t)
			assertReconciledConditionMatches(cl, smcp, status.ConditionReasonPausingInstall, "[prometheus]", t)
		})
	}
}

func assertDeploymentExists(cl client.Client, name string, t *testing.T) *appsv1.Deployment {
	t.Helper()
	deploy := &appsv1.Deployment{}
	err := cl.Get(ctx, types.NamespacedName{Namespace: controlPlaneNamespace, Name: name}, deploy)
	if errors.IsNotFound(err) {
		t.Fatalf("Expected Deployment %q to exist, but it doesn't", name)
	} else if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	return deploy
}

func newReconcilerTestFixture(smcp *maistrav2.ServiceMeshControlPlane) (client.Client, *test.EnhancedTracker, ControlPlaneInstanceReconciler) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNamespace},
	}

	operatorNamespace := "istio-operator"
	InitializeGlobals(operatorNamespace)()

	cl, tracker := test.CreateClient(smcp, namespace)
	fakeEventRecorder := &record.FakeRecorder{}

	r := NewControlPlaneInstanceReconciler(
		common.ControllerResources{
			Client:            cl,
			Scheme:            tracker.Scheme,
			EventRecorder:     fakeEventRecorder,
			OperatorNamespace: operatorNamespace,
		},
		smcp,
		cni.Config{Enabled: true})

	return cl, tracker, r
}

func assertInstanceReconcilerSucceeds(r ControlPlaneInstanceReconciler, t *testing.T) {
	t.Helper()
	_, err := r.Reconcile(ctx)
	assert.Success(err, "Reconcile", t)
}

func assertReconciledConditionMatches(cl client.Client, smcp *maistrav2.ServiceMeshControlPlane, reason status.ConditionReason, messageSubstring string, t *testing.T) {
	t.Helper()
	test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(smcp.ObjectMeta), smcp))
	reconciledCondition := smcp.Status.GetCondition(status.ConditionTypeReconciled)
	assert.Equals(reconciledCondition.Reason, reason, "Unexpected reconciledCondition.Reason", t)
	assert.True(
		strings.Contains(reconciledCondition.Message, messageSubstring),
		fmt.Sprintf("Expected to find %q in reconciledCondition.Message, but was: %s", messageSubstring, reconciledCondition.Message), t)
}

func markDeploymentAvailable(cl client.Client, deployment *appsv1.Deployment) {
	deployment.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionTrue,
		},
	}
	test.PanicOnError(cl.Update(ctx, deployment))
}

func newTestReconciler() *controlPlaneInstanceReconciler {
	reconciler := NewControlPlaneInstanceReconciler(
		common.ControllerResources{},
		&maistrav2.ServiceMeshControlPlane{},
		cni.Config{Enabled: true})
	return reconciler.(*controlPlaneInstanceReconciler)
}
