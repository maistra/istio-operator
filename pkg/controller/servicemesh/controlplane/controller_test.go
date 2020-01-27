package controlplane

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
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

	oneMinuteAgo = metav1.NewTime(time.Now().Add(-time.Minute))

	instanceReconciler *fakeInstanceReconciler
)

func TestReconcileAddsFinalizer(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.Finalizers = []string{}

	cl, _, _, r := createClientAndReconciler(t, controlPlane)

	assertReconcileSucceeds(r, t)

	updatedControlPlane := test.GetUpdatedObject(cl, controlPlane.ObjectMeta, &maistrav1.ServiceMeshControlPlane{}).(*maistrav1.ServiceMeshControlPlane)
	assert.DeepEquals(updatedControlPlane.GetFinalizers(), []string{common.FinalizerName}, "Unexpected finalizers in SMM", t)
}

func TestReconcileFailsIfItCannotAddFinalizer(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.Finalizers = []string{}

	_, tracker, _, r := createClientAndReconciler(t, controlPlane)
	tracker.AddReactor(test.ClientFailsOn("update", "servicemeshcontrolplanes"))

	assertReconcileFails(r, t)
}

func TestReconcileDoesNothingIfResourceIsDeletedAndHasNoFinalizers(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.DeletionTimestamp = &oneMinuteAgo
	controlPlane.Finalizers = nil

	_, tracker, _, r := createClientAndReconciler(t, controlPlane)
	assertReconcileSucceeds(r, t)

	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestDeleteInvokedWhenFinalizerPresentOnDeletedObject(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.DeletionTimestamp = &oneMinuteAgo

	_, _, _, r := createClientAndReconciler(t, controlPlane)
	assertReconcileSucceeds(r, t)

	assert.True(instanceReconciler.deleteInvoked, "Expected Delete() to be invoked on instance reconciler", t)
}

func TestReconcileInvokedWhenInstanceNotFullyReconciled(t *testing.T) {
	controlPlane := newControlPlane()

	_, _, _, r := createClientAndReconciler(t, controlPlane)
	assertReconcileSucceeds(r, t)

	assert.True(instanceReconciler.reconcileInvoked, "Expected Reconcile() to be invoked on instance reconciler", t)
	assert.False(instanceReconciler.updateReadinessInvoked, "Expected UpdateReadiness() NOT to be invoked on instance reconciler", t)
}

func TestUpdateReadinessInvokedWhenInstanceFullyReconciled(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.Status.ReconciledVersion = maistrav1.CurrentReconciledVersion(controlPlane.Generation)
	controlPlane.Status.Conditions = append(controlPlane.Status.Conditions, maistrav1.Condition{
		Type:               maistrav1.ConditionTypeReconciled,
		Status:             maistrav1.ConditionStatusTrue,
		Reason:             "",
		Message:            "",
		LastTransitionTime: oneMinuteAgo,
	})

	_, _, _, r := createClientAndReconciler(t, controlPlane)
	assertReconcileSucceeds(r, t)

	assert.True(instanceReconciler.updateReadinessInvoked, "Expected UpdateReadiness() to be invoked on instance reconciler", t)
	assert.False(instanceReconciler.reconcileInvoked, "Expected Reconcile() to NOT be invoked on instance reconciler", t)
}

func TestReconcileDoesNothingWhenResourceIsNotFound(t *testing.T) {
	_, tracker, _, r := createClientAndReconciler(t)
	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileFailsWhenGetResourceFails(t *testing.T) {
	_, tracker, _, r := createClientAndReconciler(t)
	tracker.AddReactor(test.ClientFailsOn("get", "servicemeshcontrolplanes"))

	assertReconcileFails(r, t)

	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func createClientAndReconciler(t *testing.T, clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, record.EventRecorder, *ControlPlaneReconciler) {
	cl, enhancedTracker := test.CreateClient(clientObjects...)
	fakeEventRecorder := &record.FakeRecorder{}

	r := newReconciler(cl, scheme.Scheme, fakeEventRecorder, "istio-operator")
	r.instanceReconcilerFactory = NewFakeInstanceReconciler
	instanceReconciler = &fakeInstanceReconciler{}
	return cl, enhancedTracker, fakeEventRecorder, r
}

type fakeInstanceReconciler struct {
	reconcileInvoked       bool
	updateReadinessInvoked bool
	deleteInvoked          bool
	finished               bool
}

func NewFakeInstanceReconciler(controllerResources common.ControllerResources, instance *maistrav1.ServiceMeshControlPlane) ControlPlaneInstanceReconciler {
	return instanceReconciler
}

func (r *fakeInstanceReconciler) Reconcile() (reconcile.Result, error) {
	r.reconcileInvoked = true
	return reconcile.Result{}, nil
}

func (r *fakeInstanceReconciler) UpdateReadiness() error {
	r.updateReadinessInvoked = true
	return nil
}

func (r *fakeInstanceReconciler) Delete() error {
	r.deleteInvoked = true
	return nil
}

func (r *fakeInstanceReconciler) SetInstance(instance *maistrav1.ServiceMeshControlPlane) {
}

func (r *fakeInstanceReconciler) IsFinished() bool {
	return r.finished
}

func assertReconcileSucceeds(r *ControlPlaneReconciler, t *testing.T) {
	res, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if res.Requeue {
		t.Error("Reconcile requeued the request, but it shouldn't have")
	}
}

func assertReconcileFails(r *ControlPlaneReconciler, t *testing.T) {
	_, err := r.Reconcile(request)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}
}

func newControlPlane() *maistrav1.ServiceMeshControlPlane {
	return &maistrav1.ServiceMeshControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:       controlPlaneName,
			Namespace:  controlPlaneNamespace,
			Finalizers: []string{common.FinalizerName},
			Generation: 1,
			UID:        controlPlaneUID,
		},
		Spec:   maistrav1.ControlPlaneSpec{},
		Status: maistrav1.ControlPlaneStatus{},
	}
}
