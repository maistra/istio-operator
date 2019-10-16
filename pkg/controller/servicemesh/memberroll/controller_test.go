package memberroll

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/maistra/istio-operator/pkg/apis"
	maistra "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

const (
	memberRollName        = "default"
	memberRollUID         = types.UID("1111")
	appNamespace          = "app-namespace"
	appNamespace2         = "app-namespace-2"
	controlPlaneName      = "my-mesh"
	controlPlaneNamespace = "cp-namespace"
	controlPlaneUID       = types.UID("2222")
)

var (
	request = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      memberRollName,
			Namespace: controlPlaneNamespace,
		},
	}

	oneMinuteAgo = meta.NewTime(time.Now().Add(-time.Minute))
)

func init() {
	logf.SetLogger(logf.ZapLogger(true))
}

func TestReconcileAddsFinalizer(t *testing.T) {
	roll := newMemberRoll()
	roll.Finalizers = []string{}

	cl, _, r, _ := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(cl, roll.ObjectMeta, &maistra.ServiceMeshMemberRoll{}).(*maistra.ServiceMeshMemberRoll)
	assert.DeepEquals(updatedRoll.GetFinalizers(), []string{common.FinalizerName}, "Unexpected finalizers in SMM", t)
}

func TestReconcileDoesNothingIfMemberRollIsDeletedAndHasNoFinalizers(t *testing.T) {
	roll := newMemberRoll()
	roll.DeletionTimestamp = &oneMinuteAgo
	roll.Finalizers = nil

	_, tracker, r, _ := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingWhenMemberRollIsNotFound(t *testing.T) {
	_, tracker, r, _ := createClientAndReconciler(t)
	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileFailsWhenGetMemberRollFails(t *testing.T) {
	_, tracker, r, _ := createClientAndReconciler(t)
	tracker.AddReactor(test.ClientFailsOn("get", "servicemeshmemberrolls"))
	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileFailsWhenListControlPlanesFails(t *testing.T) {
	roll := newMemberRoll()
	_, tracker, r, _ := createClientAndReconciler(t, roll)
	tracker.AddReactor(test.ClientFailsOn("list", "servicemeshcontrolplanes"))

	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingIfControlPlaneMissing(t *testing.T) {
	roll := newMemberRoll()
	_, tracker, r, _ := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingIfMultipleControlPlanesFound(t *testing.T) {
	roll := newMemberRoll()
	controlPlane1 := newControlPlane()
	controlPlane2 := newControlPlane()
	controlPlane2.Name = "my-mesh-2"
	_, tracker, r, _ := createClientAndReconciler(t, roll, controlPlane1, controlPlane2)
	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileAddsOwnerReference(t *testing.T) {
	roll := newMemberRoll()
	roll.OwnerReferences = []meta.OwnerReference{}
	controlPlane := newControlPlane()

	cl, _, r, _ := createClientAndReconciler(t, roll, controlPlane)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(cl, roll.ObjectMeta, &maistra.ServiceMeshMemberRoll{}).(*maistra.ServiceMeshMemberRoll)
	assert.Equals(len(updatedRoll.OwnerReferences), 1, "Expected SMMR to contain exactly one ownerReference", t)

	expectedOwnerRef := meta.OwnerReference{
		APIVersion: maistra.SchemeGroupVersion.String(),
		Kind:       "ServiceMeshControlPlane",
		Name:       controlPlaneName,
		UID:        controlPlaneUID,
	}
	assert.DeepEquals(updatedRoll.OwnerReferences[0], expectedOwnerRef, "Unexpected OwnerReference in SMMR", t)
}

func TestReconcileFailsIfAddingOwnerReferenceFails(t *testing.T) {
	roll := newMemberRoll()
	roll.OwnerReferences = []meta.OwnerReference{}
	controlPlane := newControlPlane()

	_, tracker, r, _ := createClientAndReconciler(t, roll, controlPlane)
	tracker.AddReactor(test.ClientFailsOn("update", "servicemeshmemberrolls"))

	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 1) // we expect only the update that fails
}

func TestReconcileDoesNothingIfControlPlaneNotReconciledAtLeastOnce(t *testing.T) {
	roll := newMemberRoll()
	addOwnerReference(roll)
	controlPlane := newControlPlane()
	controlPlane.Status.ObservedGeneration = 0

	_, tracker, r, _ := createClientAndReconciler(t, roll, controlPlane)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingIfControlPlaneReconciledConditionIsNotTrue(t *testing.T) {
	roll := newMemberRoll()
	addOwnerReference(roll)
	controlPlane := newControlPlane()
	controlPlane.Status.ObservedGeneration = 1
	controlPlane.Status.Conditions = []maistra.Condition{
		{
			Type:   maistra.ConditionTypeReconciled,
			Status: maistra.ConditionStatusFalse,
		},
	}

	_, tracker, r, _ := createClientAndReconciler(t, roll, controlPlane)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileFailsIfListingNamespacesFails(t *testing.T) {
	roll := newMemberRoll()
	addOwnerReference(roll)
	controlPlane := newControlPlane()
	markControlPlaneReconciled(controlPlane)

	_, tracker, r, _ := createClientAndReconciler(t, roll, controlPlane)
	tracker.AddReactor(test.ClientFailsOn("list", "namespaces"))

	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileReconcilesAddedMember(t *testing.T) {
	roll := newMemberRoll()
	addOwnerReference(roll)
	roll.Spec.Members = []string{appNamespace}
	roll.ObjectMeta.Generation = 2
	roll.Status.ObservedGeneration = 1
	controlPlane := markControlPlaneReconciled(newControlPlane())
	namespace := newAppNamespace()
	meshRoleBinding := newMeshRoleBinding()

	cl, _, r, nsReconciler := createClientAndReconciler(t, roll, controlPlane, namespace, meshRoleBinding)
	common.IsCNIEnabled = true // TODO: this is a global variable; we should get rid of it, because we can't parallelize tests because of it

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(cl, roll.ObjectMeta, &maistra.ServiceMeshMemberRoll{}).(*maistra.ServiceMeshMemberRoll)
	assert.DeepEquals(updatedRoll.Status.ConfiguredMembers, []string{appNamespace}, "Unexpected Status.ConfiguredMembers in SMMR", t)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)

	assertNamespaceReconcilerInvoked(t, nsReconciler, appNamespace)
}

func TestReconcileReconcilesMemberIfNamespaceIsCreatedLater(t *testing.T) {
	roll := newMemberRoll()
	addOwnerReference(roll)
	roll.Spec.Members = []string{appNamespace}
	roll.ObjectMeta.Generation = 2
	roll.Status.ObservedGeneration = 2 // NOTE: generation 2 of the member roll has already been reconciled
	controlPlane := markControlPlaneReconciled(newControlPlane())
	namespace := newAppNamespace()
	meshRoleBinding := newMeshRoleBinding()

	cl, _, r, nsReconciler := createClientAndReconciler(t, roll, controlPlane, namespace, meshRoleBinding)
	common.IsCNIEnabled = true // TODO: this is a global variable; we should get rid of it, because we can't parallelize tests because of it

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(cl, roll.ObjectMeta, &maistra.ServiceMeshMemberRoll{}).(*maistra.ServiceMeshMemberRoll)
	assert.DeepEquals(updatedRoll.Status.ConfiguredMembers, []string{appNamespace}, "Unexpected Status.ConfiguredMembers in SMMR", t)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, int64(0), "Unexpected Status.ServiceMeshGeneration in SMMR", t)

	assertNamespaceReconcilerInvoked(t, nsReconciler, appNamespace)

	// invoke reconcile again to check if the Status.ServiceMeshGeneration field is updated
	assertReconcileSucceeds(r, t)
	updatedRoll = test.GetUpdatedObject(cl, roll.ObjectMeta, &maistra.ServiceMeshMemberRoll{}).(*maistra.ServiceMeshMemberRoll)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
}

func TestReconcileReconcilesMembersWhenControlPlaneUpdated(t *testing.T) {
	roll := newMemberRoll()
	addOwnerReference(roll)
	roll.Spec.Members = []string{appNamespace}
	roll.ObjectMeta.Generation = 2
	roll.Status.ObservedGeneration = 2 // NOTE: generation 2 of the member roll has already been reconciled
	roll.Status.ConfiguredMembers = []string{appNamespace}
	controlPlane := markControlPlaneReconciled(newControlPlane())
	namespace := newAppNamespace()
	meshRoleBinding := newMeshRoleBinding()
	nsRoleBinding := newAppNamespaceRoleBinding()

	cl, _, r, nsReconciler := createClientAndReconciler(t, roll, controlPlane, namespace, meshRoleBinding, nsRoleBinding)
	common.IsCNIEnabled = true // TODO: this is a global variable; we should get rid of it, because we can't parallelize tests because of it

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(cl, roll.ObjectMeta, &maistra.ServiceMeshMemberRoll{}).(*maistra.ServiceMeshMemberRoll)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)

	assertNamespaceReconcilerInvoked(t, nsReconciler, appNamespace)
}

func assertNamespaceReconcilerInvoked(t *testing.T, nsReconciler *fakeNamespaceReconciler, ns string) {
	assert.DeepEquals(nsReconciler.reconciledNamespaces, []string{ns}, "Expected namespace reconciler to be invoked, but it wasn't invoked or wasn't invoked properly", t)
}

// TODO: add test that checks if one namespace is missing, but another is present, the latter gets reconciled and reconcile does NOT return an error

func TestReconcileDoesNotAddControlPlaneNamespaceToMembers(t *testing.T) {
	roll := newMemberRoll()
	addOwnerReference(roll)
	roll.Spec.Members = []string{controlPlaneNamespace}
	roll.ObjectMeta.Generation = 2
	roll.Status.ObservedGeneration = 1
	controlPlane := markControlPlaneReconciled(newControlPlane())
	namespace := &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: controlPlaneNamespace,
		},
	}

	cl, _, r, _ := createClientAndReconciler(t, roll, controlPlane, namespace)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(cl, roll.ObjectMeta, &maistra.ServiceMeshMemberRoll{}).(*maistra.ServiceMeshMemberRoll)
	assert.StringArrayEmpty(updatedRoll.Status.ConfiguredMembers, "Expected Status.ConfiguredMembers in SMMR to be empty, but it wasn't.", t)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
}

func TestReconcileRemovesFinalizerFromMemberRoll(t *testing.T) {
	roll := newMemberRoll()
	roll.DeletionTimestamp = &oneMinuteAgo

	cl, _, r, _ := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(cl, roll.ObjectMeta, &maistra.ServiceMeshMemberRoll{}).(*maistra.ServiceMeshMemberRoll)
	assert.StringArrayEmpty(updatedRoll.Finalizers, "Expected finalizers list in SMMR to be empty, but it wasn't", t)
}

func TestClientReturnsErrorWhenRemovingFinalizer(t *testing.T) {
	cases := []struct {
		name                 string
		reactor              test.ReactFunc
		successExpected      bool
		expectedWriteActions int
	}{
		{
			name:                 "get-memberroll-returns-notfound",
			reactor:              test.On("get", "servicemeshmemberrolls", test.AttemptNumber(2, test.ClientReturnsNotFound(maistra.APIGroup, "ServiceMeshMemberRoll", memberRollName))),
			successExpected:      true,
			expectedWriteActions: 0,
		},
		{
			name:                 "get-memberroll-fails",
			reactor:              test.On("get", "servicemeshmemberrolls", test.AttemptNumber(2, test.ClientFails())),
			successExpected:      false,
			expectedWriteActions: 0,
		},
		{
			name:                 "update-memberroll-returns-notfound",
			reactor:              test.On("update", "servicemeshmemberrolls", test.ClientReturnsNotFound(maistra.APIGroup, "ServiceMeshMemberRoll", memberRollName)),
			successExpected:      true,
			expectedWriteActions: 1,
		},
		{
			name:                 "update-memberroll-fails",
			reactor:              test.On("update", "servicemeshmemberrolls", test.ClientFails()),
			successExpected:      false,
			expectedWriteActions: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roll := newMemberRoll()
			roll.DeletionTimestamp = &oneMinuteAgo

			_, tracker, r, _ := createClientAndReconciler(t, roll)
			tracker.AddReactor(tc.reactor)

			if tc.successExpected {
				assertReconcileSucceeds(r, t)
			} else {
				assertReconcileFails(r, t)
			}
			test.AssertNumberOfWriteActions(t, tracker.Actions(), tc.expectedWriteActions)
		})
	}
}

func createClientAndReconciler(t *testing.T, clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, *ReconcileMemberList, *fakeNamespaceReconciler) {
	s := scheme.Scheme // scheme must be initialized before creating the client below
	if err := rbac.AddToScheme(s); err != nil {
		t.Fatalf("Could not add to scheme: %v", err)
	}
	if err := apis.AddToScheme(s); err != nil {
		t.Fatalf("Could not add to scheme: %v", err)
	}

	tracker := clienttesting.NewObjectTracker(scheme.Scheme, scheme.Codecs.UniversalDecoder())
	enhancedTracker := test.NewEnhancedTracker(tracker)
	cl := fake.NewFakeClientWithSchemeAndTracker(scheme.Scheme, &enhancedTracker, clientObjects...)

	rf := fakeNamespaceReconcilerFactory{
		reconciler: &fakeNamespaceReconciler{},
	}

	r := &ReconcileMemberList{
		ResourceManager:        common.ResourceManager{Client: cl, PatchFactory: common.NewPatchFactory(cl), Log: log},
		scheme:                 s,
		newNamespaceReconciler: rf.newReconciler,
	}
	return cl, &enhancedTracker, r, rf.reconciler
}

type fakeNamespaceReconcilerFactory struct {
	reconciler *fakeNamespaceReconciler
}

func (rf *fakeNamespaceReconcilerFactory) newReconciler(cl client.Client, logger logr.Logger, meshNamespace string, isCNIEnabled bool) (NamespaceReconciler, error) {
	return rf.reconciler, nil
}

type fakeNamespaceReconciler struct {
	reconciledNamespaces []string
	removedNamespaces    []string
}

func (r *fakeNamespaceReconciler) reconcileNamespaceInMesh(namespace string) error {
	r.reconciledNamespaces = append(r.reconciledNamespaces, namespace)
	return nil
}

func (r *fakeNamespaceReconciler) removeNamespaceFromMesh(namespace string) error {
	r.removedNamespaces = append(r.removedNamespaces, namespace)
	return nil
}

func assertReconcileSucceeds(r *ReconcileMemberList, t *testing.T) {
	res, err := r.Reconcile(request)
	if err != nil {
		log.Error(err, "Reconcile failed")
		t.Fatalf("Reconcile failed: %v", err)
	}
	if res.Requeue {
		t.Error("Reconcile requeued the request, but it shouldn't have")
	}
}

func assertReconcileFails(r *ReconcileMemberList, t *testing.T) {
	_, err := r.Reconcile(request)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}
}

func newMemberRoll() *maistra.ServiceMeshMemberRoll {
	return &maistra.ServiceMeshMemberRoll{
		ObjectMeta: meta.ObjectMeta{
			Name:       memberRollName,
			Namespace:  controlPlaneNamespace,
			Finalizers: []string{common.FinalizerName},
			Generation: 1,
			UID:        memberRollUID,
		},
		Spec: maistra.ServiceMeshMemberRollSpec{
			Members: []string{},
		},
		Status: maistra.ServiceMeshMemberRollStatus{
			ObservedGeneration: 1,
		},
	}
}

func addOwnerReference(roll *maistra.ServiceMeshMemberRoll) *maistra.ServiceMeshMemberRoll {
	roll.OwnerReferences = []meta.OwnerReference{
		{
			APIVersion: maistra.SchemeGroupVersion.String(),
			Kind:       "ServiceMeshControlPlane",
			Name:       controlPlaneName,
			UID:        controlPlaneUID,
		},
	}
	return roll
}

func newControlPlane() *maistra.ServiceMeshControlPlane {
	return &maistra.ServiceMeshControlPlane{
		ObjectMeta: meta.ObjectMeta{
			Name:      controlPlaneName,
			Namespace: controlPlaneNamespace,
			UID:       controlPlaneUID,
		},
		Spec: maistra.ControlPlaneSpec{},
	}
}

func markControlPlaneReconciled(controlPlane *maistra.ServiceMeshControlPlane) *maistra.ServiceMeshControlPlane {
	controlPlane.Status.ObservedGeneration = 1
	controlPlane.Status.Conditions = []maistra.Condition{
		{
			Type:   maistra.ConditionTypeReconciled,
			Status: maistra.ConditionStatusTrue,
		},
	}
	return controlPlane
}

func newAppNamespace() *core.Namespace {
	namespace := &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: appNamespace,
		},
	}
	return namespace
}

func newMeshRoleBinding() *rbac.RoleBinding {
	return &rbac.RoleBinding{
		ObjectMeta: meta.ObjectMeta{
			Namespace: controlPlaneNamespace,
			Name:      "role-binding",
			Labels: map[string]string{
				common.OwnerKey: controlPlaneNamespace,
			},
		},
	}
}

func newAppNamespaceRoleBinding() *rbac.RoleBinding {
	roleBinding := newMeshRoleBinding()
	roleBinding.Namespace = appNamespace
	roleBinding.Labels[common.MemberOfKey] = controlPlaneNamespace
	return roleBinding
}
