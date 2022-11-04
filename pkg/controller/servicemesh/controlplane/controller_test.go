package controlplane

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	"github.com/maistra/istio-operator/pkg/controller/versions"
	"github.com/maistra/istio-operator/pkg/version"
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

	now          = metav1.NewTime(time.Now().Truncate(time.Second))
	oneMinuteAgo = metav1.NewTime(time.Now().Truncate(time.Second).Add(-time.Minute))

	instanceReconciler *fakeInstanceReconciler
)

func TestReconcileAddsFinalizer(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.Finalizers = []string{}

	cl, _, r := createClientAndReconciler(controlPlane)

	assertReconcileSucceeds(r, t)

	updatedControlPlane := test.GetUpdatedObject(ctx, cl, controlPlane.ObjectMeta, &maistrav2.ServiceMeshControlPlane{}).(*maistrav2.ServiceMeshControlPlane)
	assert.DeepEquals(updatedControlPlane.GetFinalizers(), []string{common.FinalizerName}, "Unexpected finalizers in SMM", t)
}

func TestReconcileFailsIfItCannotAddFinalizer(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.Finalizers = []string{}

	_, tracker, r := createClientAndReconciler(controlPlane)
	tracker.AddReactor("update", "servicemeshcontrolplanes", test.ClientFails())

	assertReconcileFails(r, t)
}

func TestReconcileDoesNothingIfResourceIsDeletedAndHasNoFinalizers(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.DeletionTimestamp = &oneMinuteAgo
	controlPlane.Finalizers = nil

	_, tracker, r := createClientAndReconciler(controlPlane)
	assertReconcileSucceeds(r, t)

	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestDeleteInvokedWhenFinalizerPresentOnDeletedObject(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.DeletionTimestamp = &oneMinuteAgo

	_, _, r := createClientAndReconciler(controlPlane)
	assertReconcileSucceeds(r, t)

	assert.True(instanceReconciler.deleteInvoked, "Expected Delete() to be invoked on instance reconciler", t)
}

func TestReconcileInvokedWhenInstanceNotFullyReconciled(t *testing.T) {
	controlPlane := newControlPlane()

	_, _, r := createClientAndReconciler(controlPlane)
	assertReconcileSucceeds(r, t)

	assert.True(instanceReconciler.reconcileInvoked, "Expected Reconcile() to be invoked on instance reconciler", t)
	assert.False(instanceReconciler.updateReadinessInvoked, "Expected UpdateReadiness() NOT to be invoked on instance reconciler", t)
}

func TestUpdateReadinessInvokedWhenInstanceFullyReconciled(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.Status.OperatorVersion = version.Info.Version
	controlPlane.Status.ObservedGeneration = controlPlane.Generation
	controlPlane.Status.Conditions = append(controlPlane.Status.Conditions, status.Condition{
		Type:               status.ConditionTypeReconciled,
		Status:             status.ConditionStatusTrue,
		Reason:             "",
		Message:            "",
		LastTransitionTime: oneMinuteAgo,
	})

	_, _, r := createClientAndReconciler(controlPlane)
	assertReconcileSucceeds(r, t)

	assert.True(instanceReconciler.updateReadinessInvoked, "Expected UpdateReadiness() to be invoked on instance reconciler", t)
	assert.False(instanceReconciler.reconcileInvoked, "Expected Reconcile() to NOT be invoked on instance reconciler", t)
}

func TestReconcileDoesNothingWhenResourceIsNotFound(t *testing.T) {
	_, tracker, r := createClientAndReconciler()
	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileFailsWhenGetResourceFails(t *testing.T) {
	_, tracker, r := createClientAndReconciler()
	tracker.AddReactor("get", "servicemeshcontrolplanes", test.ClientFails())

	assertReconcileFails(r, t)

	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func createClientAndReconciler(clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, *ControlPlaneReconciler) {
	cl, enhancedTracker := test.CreateClient(clientObjects...)
	fakeEventRecorder := &record.FakeRecorder{}

	dc := fake.FakeDiscovery{&enhancedTracker.Fake, test.DefaultKubeVersion}
	r := newReconciler(cl, scheme.Scheme, fakeEventRecorder, "istio-operator", cni.Config{Enabled: true}, &dc)
	r.instanceReconcilerFactory = NewFakeInstanceReconciler
	instanceReconciler = &fakeInstanceReconciler{}
	return cl, enhancedTracker, r
}

type fakeInstanceReconciler struct {
	reconcileInvoked       bool
	updateReadinessInvoked bool
	deleteInvoked          bool
	finished               bool
}

func NewFakeInstanceReconciler(_ common.ControllerResources, _ *maistrav2.ServiceMeshControlPlane, _ cni.Config) ControlPlaneInstanceReconciler {
	return instanceReconciler
}

func (r *fakeInstanceReconciler) Reconcile(ctx context.Context) (reconcile.Result, error) {
	r.reconcileInvoked = true
	return reconcile.Result{}, nil
}

func (r *fakeInstanceReconciler) UpdateReadiness(ctx context.Context) error {
	r.updateReadinessInvoked = true
	return nil
}

func (r *fakeInstanceReconciler) PatchAddons(ctx context.Context, _ *maistrav2.ControlPlaneSpec) (reconcile.Result, error) {
	r.updateReadinessInvoked = true
	return common.Reconciled()
}

func (r *fakeInstanceReconciler) Delete(ctx context.Context) error {
	r.deleteInvoked = true
	return nil
}

func (r *fakeInstanceReconciler) SetInstance(instance *maistrav2.ServiceMeshControlPlane) {
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

func newControlPlane() *maistrav2.ServiceMeshControlPlane {
	return &maistrav2.ServiceMeshControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:       controlPlaneName,
			Namespace:  controlPlaneNamespace,
			Finalizers: []string{common.FinalizerName},
			Generation: 1,
			UID:        controlPlaneUID,
		},
		Spec: maistrav2.ControlPlaneSpec{
			Version: versions.DefaultVersion.String(),
		},
		Status: maistrav2.ControlPlaneStatus{},
	}
}
