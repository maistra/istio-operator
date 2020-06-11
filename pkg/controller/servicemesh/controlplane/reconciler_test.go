package controlplane

import (
	"reflect"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/kubernetes/scheme"

	"github.com/maistra/istio-operator/pkg/apis/maistra"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func TestGetSMCPTemplateWithSlashReturnsError(t *testing.T) {
	instanceReconciler := newTestReconciler()
	_, err := instanceReconciler.getSMCPTemplate("/", maistra.DefaultVersion.String())
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
	_, err := instanceReconciler.recursivelyApplyTemplates(ctx, maistrav1.ControlPlaneSpec{Template: "visited"}, "", sets.NewString("visited"))
	if err == nil {
		t.Fatalf("Expected error to not be nil. Cyclic dependencies should not be allowed.")
	}
}

func TestInstallationErrorDoesNotUpdateLastTransitionTimeWhenNoStateTransitionOccurs(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.Spec.Istio = maistrav1.HelmValuesType{}
	controlPlane.Spec.Template = "maistra"
	controlPlane.Status.SetCondition(maistrav1.Condition{
		Type:               maistrav1.ConditionTypeReconciled,
		Status:             maistrav1.ConditionStatusFalse,
		Reason:             "",
		Message:            "",
		LastTransitionTime: oneMinuteAgo,
	})

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "cp-namespace"},
	}

	operatorNamespace := "istio-operator"
	InitializeGlobals(operatorNamespace)()

	cl, tracker := test.CreateClient(controlPlane, namespace)
	fakeEventRecorder := &record.FakeRecorder{}

	r := NewControlPlaneInstanceReconciler(
		common.ControllerResources{
			Client:            cl,
			Scheme:            scheme.Scheme,
			EventRecorder:     fakeEventRecorder,
			PatchFactory:      common.NewPatchFactory(cl),
			OperatorNamespace: operatorNamespace,
		},
		controlPlane,
		common.CNIConfig{Enabled: true})

	// make installation fail
	tracker.AddReactor("create", "deployments", test.ClientFails())

	// run initial reconcile to update the SMCP status
	_, err := r.Reconcile(ctx)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}

	// run reconcile again to work around the problem where the condition reason "InstallError" gets changed to "ReconcileError" in 2nd reconcile
	_, err = r.Reconcile(ctx)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}

	// remember the SMCP status at this point
	updatedControlPlane := &maistrav1.ServiceMeshControlPlane{}
	test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(controlPlane.ObjectMeta), updatedControlPlane))
	initialStatus := updatedControlPlane.Status.DeepCopy()

	// the resolution of lastTransitionTime is one second, so we need to wait at least one second before
	// performing another reconciliation to ensure that if the lastTransitionTime field is reset, the new
	// value will actually be different from the previous one
	time.Sleep(1 * time.Second)

	// run reconcile again to check if the status is still the same
	_, err = r.Reconcile(ctx)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}

	test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(controlPlane.ObjectMeta), updatedControlPlane))
	newStatus := &updatedControlPlane.Status

	assert.DeepEquals(newStatus, initialStatus, "didn't expect SMCP status to be updated", t)
}

type customSetup func(client.Client, *test.EnhancedTracker)

func TestManifestValidation(t *testing.T) {
	testCases := []struct {
		name         string
		controlPlane *maistrav1.ServiceMeshControlPlane
		memberRoll   *maistrav1.ServiceMeshMemberRoll
		setupFn      customSetup
		errorMessage string
	}{
		{
			name: "error getting smmr",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: newControlPlane().ObjectMeta,
				Spec: maistrav1.ControlPlaneSpec{
					Template: "maistra",
					Version:  "v1.1",
					Istio: map[string]interface{}{
						"gateways": map[string]interface{}{
							"istio-ingressgateway": map[string]interface{}{
								"namespace": "somewhere",
							},
						},
					},
				},
				Status: maistrav1.ControlPlaneStatus{},
			},
			memberRoll: &maistrav1.ServiceMeshMemberRoll{},
			setupFn: func(cl client.Client, tracker *test.EnhancedTracker) {
				tracker.AddReactor("get", "servicemeshmemberrolls", test.ClientFails())
			},
			errorMessage: "some error",
		},
		{
			name: "gateways outside of mesh",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: newControlPlane().ObjectMeta,
				Spec: maistrav1.ControlPlaneSpec{
					Template: "maistra",
					Version:  "v1.1",
					Istio: map[string]interface{}{
						"gateways": map[string]interface{}{
							"another-gateway": map[string]interface{}{
								"enabled":   "true",
								"namespace": "b",
								"labels":    map[string]interface{}{},
							},
							"istio-ingressgateway": map[string]interface{}{
								"namespace": "c",
							},
							"istio-egressgateway": map[string]interface{}{
								"namespace": "d",
							},
						},
					},
				},
				Status: maistrav1.ControlPlaneStatus{},
			},
			memberRoll: &maistrav1.ServiceMeshMemberRoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: controlPlaneNamespace,
				},
				Spec: maistrav1.ServiceMeshMemberRollSpec{
					Members: []string{
						"a",
					},
				},
				Status: maistrav1.ServiceMeshMemberRollStatus{
					ConfiguredMembers: []string{
						"a",
					},
				},
			},
			errorMessage: "namespace of manifest c/istio-ingressgateway not in mesh",
		},
		{
			name: "valid namespaces",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: newControlPlane().ObjectMeta,
				Spec: maistrav1.ControlPlaneSpec{
					Template: "maistra",
					Version:  "v1.1",
					Istio: map[string]interface{}{
						"gateways": map[string]interface{}{
							"istio-ingressgateway": map[string]interface{}{
								"namespace": "a",
							},
							"istio-egressgateway": map[string]interface{}{
								"namespace": "c",
							},
						},
					},
				},
				Status: maistrav1.ControlPlaneStatus{},
			},
			memberRoll: &maistrav1.ServiceMeshMemberRoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: controlPlaneNamespace,
				},
				Spec: maistrav1.ServiceMeshMemberRollSpec{
					Members: []string{
						"a",
						"c",
					},
				},
				Status: maistrav1.ServiceMeshMemberRollStatus{
					ConfiguredMembers: []string{
						"a",
						"c",
					},
				},
			},
		},
	}

	operatorNamespace := "istio-operator"
	InitializeGlobals(operatorNamespace)()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.controlPlane.Status.SetCondition(maistrav1.Condition{
				Type:               maistrav1.ConditionTypeReconciled,
				Status:             maistrav1.ConditionStatusFalse,
				Reason:             "",
				Message:            "",
				LastTransitionTime: oneMinuteAgo,
			})

			namespace := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: tc.controlPlane.Namespace},
			}

			cl, tracker := test.CreateClient(tc.controlPlane, tc.memberRoll, namespace)
			fakeEventRecorder := &record.FakeRecorder{}

			r := NewControlPlaneInstanceReconciler(
				common.ControllerResources{
					Client:            cl,
					Scheme:            scheme.Scheme,
					EventRecorder:     fakeEventRecorder,
					PatchFactory:      common.NewPatchFactory(cl),
					OperatorNamespace: operatorNamespace,
				},
				tc.controlPlane,
				common.CNIConfig{Enabled: true})

			if tc.setupFn != nil {
				tc.setupFn(cl, tracker)
			}
			// run initial reconcile to update the SMCP status
			_, err := r.Reconcile(ctx)

			if tc.errorMessage != "" {
				if err == nil {
					t.Fatal(tc.name, "-", "Expected reconcile to fail, but it didn't")
				}

				updatedControlPlane := &maistrav1.ServiceMeshControlPlane{}
				test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(tc.controlPlane.ObjectMeta), updatedControlPlane))
				newStatus := &updatedControlPlane.Status

				assert.True(strings.Contains(newStatus.GetCondition(maistrav1.ConditionTypeReconciled).Message, tc.errorMessage), "Expected reconciliation error: "+tc.errorMessage+", but got:"+newStatus.GetCondition(maistrav1.ConditionTypeReconciled).Message, t)
			} else {
				if err != nil {
					t.Fatal(tc.name, "-", "Expected no errors, but got error: ", err)
				}
			}
		})

	}

}

func newTestReconciler() *controlPlaneInstanceReconciler {
	reconciler := NewControlPlaneInstanceReconciler(
		common.ControllerResources{},
		&maistrav1.ServiceMeshControlPlane{},
		common.CNIConfig{Enabled: true})
	return reconciler.(*controlPlaneInstanceReconciler)
}
