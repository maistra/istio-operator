package memberroll

import (
	"context"
	"fmt"
	"testing"
	"time"

	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

const (
	memberRollName        = "default"
	memberRollUID         = types.UID("1111")
	appNamespace          = "app-namespace"
	appNamespace2         = "app-namespace-2"
	controlPlaneName      = "my-mesh"
	controlPlaneNamespace = "cp-namespace"
	controlPlaneUID       = types.UID("2222")

	operatorVersion1_0     = "1.0.0"
	operatorVersion1_1     = "1.1.0"
	operatorVersionDefault = operatorVersion1_1

	cniNetwork1_0     = "istio-cni"
	cniNetwork1_1     = "v1-1-istio-cni"
	cniNetworkDefault = cniNetwork1_1
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
	roll := newDefaultMemberRoll()
	roll.Finalizers = []string{}

	cl, _, r, _, _ := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.DeepEquals(updatedRoll.GetFinalizers(), []string{common.FinalizerName}, "Unexpected finalizers in SMM", t)
}

func TestReconcileFailsIfItCannotAddFinalizer(t *testing.T) {
	roll := newDefaultMemberRoll()
	roll.Finalizers = []string{}

	_, tracker, r, _, _ := createClientAndReconciler(t, roll)
	tracker.AddReactor("update", "servicemeshmemberrolls", test.ClientFails())
	assertReconcileFails(r, t)
}

func TestReconcileDoesNothingIfMemberRollIsDeletedAndHasNoFinalizers(t *testing.T) {
	roll := newDefaultMemberRoll()
	roll.DeletionTimestamp = &oneMinuteAgo
	roll.Finalizers = nil

	_, tracker, r, _, _ := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingWhenMemberRollIsNotFound(t *testing.T) {
	_, tracker, r, _, _ := createClientAndReconciler(t)
	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileFailsWhenGetMemberRollFails(t *testing.T) {
	_, tracker, r, _, _ := createClientAndReconciler(t)
	tracker.AddReactor("get", "servicemeshmemberrolls", test.ClientFails())
	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileFailsWhenListControlPlanesFails(t *testing.T) {
	roll := newDefaultMemberRoll()
	_, tracker, r, _, _ := createClientAndReconciler(t, roll)
	tracker.AddReactor("list", "servicemeshcontrolplanes", test.ClientFails())

	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingIfControlPlaneMissing(t *testing.T) {
	roll := newDefaultMemberRoll()
	_, tracker, r, _, _ := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingIfMultipleControlPlanesFound(t *testing.T) {
	roll := newDefaultMemberRoll()
	controlPlane1 := newControlPlane("")
	controlPlane2 := newControlPlane("")
	controlPlane2.Name = "my-mesh-2"
	_, tracker, r, _, _ := createClientAndReconciler(t, roll, controlPlane1, controlPlane2)
	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileAddsOwnerReference(t *testing.T) {
	roll := newDefaultMemberRoll()
	roll.OwnerReferences = []meta.OwnerReference{}
	controlPlane := newControlPlane("")

	cl, _, r, _, _ := createClientAndReconciler(t, roll, controlPlane)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.Equals(len(updatedRoll.OwnerReferences), 1, "Expected SMMR to contain exactly one ownerReference", t)

	expectedOwnerRef := meta.OwnerReference{
		APIVersion: maistrav1.SchemeGroupVersion.String(),
		Kind:       "ServiceMeshControlPlane",
		Name:       controlPlaneName,
		UID:        controlPlaneUID,
	}
	assert.DeepEquals(updatedRoll.OwnerReferences[0], expectedOwnerRef, "Unexpected OwnerReference in SMMR", t)
}

func TestReconcileFailsIfAddingOwnerReferenceFails(t *testing.T) {
	roll := newDefaultMemberRoll()
	roll.OwnerReferences = []meta.OwnerReference{}
	controlPlane := newControlPlane("")

	_, tracker, r, _, _ := createClientAndReconciler(t, roll, controlPlane)
	tracker.AddReactor("update", "servicemeshmemberrolls", test.ClientFails())

	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 1) // we expect only the update that fails
}

func TestReconcileDoesNothingIfControlPlaneNotReconciledAtLeastOnce(t *testing.T) {
	roll := newDefaultMemberRoll()
	addOwnerReference(roll)
	controlPlane := newControlPlane("")
	controlPlane.Status.ObservedGeneration = 0

	_, tracker, r, _, _ := createClientAndReconciler(t, roll, controlPlane)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingIfControlPlaneReconciledConditionIsNotTrue(t *testing.T) {
	roll := newDefaultMemberRoll()
	addOwnerReference(roll)
	controlPlane := newControlPlane("")
	controlPlane.Status.ObservedGeneration = 1
	controlPlane.Status.Conditions = []maistrav1.Condition{
		{
			Type:   maistrav1.ConditionTypeReconciled,
			Status: maistrav1.ConditionStatusFalse,
		},
	}

	_, tracker, r, _, _ := createClientAndReconciler(t, roll, controlPlane)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileFailsIfListingNamespacesFails(t *testing.T) {
	roll := newDefaultMemberRoll()
	addOwnerReference(roll)
	controlPlane := newControlPlane("")
	markControlPlaneReconciled(controlPlane, operatorVersionDefault)

	_, tracker, r, _, _ := createClientAndReconciler(t, roll, controlPlane)
	tracker.AddReactor("list", "namespaces", test.ClientFails())

	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileReconcilesAfterOperatorUpgradeFromV1_0(t *testing.T) {
	roll := newMemberRoll(1, 1, 1, operatorVersion1_0)
	addOwnerReference(roll)
	roll.Spec.Members = []string{appNamespace}
	roll.Status.ConfiguredMembers = []string{appNamespace}
	controlPlane := markControlPlaneReconciled(newControlPlane(versions.V1_0.String()), operatorVersionDefault)
	namespace := newNamespace(appNamespace)
	common.SetLabel(namespace, common.MemberOfKey, controlPlaneNamespace)
	meshRoleBinding := newMeshRoleBinding()
	appRoleBinding := newMeshRoleBinding()
	appRoleBinding.SetNamespace(appNamespace)
	common.SetLabel(appRoleBinding, common.MemberOfKey, controlPlaneNamespace)
	nad := createNAD(cniNetwork1_0, appNamespace, controlPlaneNamespace)

	cl, tracker, r, _, _ := createClientAndReconciler(t, roll, controlPlane, namespace, meshRoleBinding, appRoleBinding, nad)
	tracker.AddReactor("delete", "k8s.cni.cncf.io/v1, Resource=networkattachmentdefinitions", assertNADNotDeleted(t))
	tracker.AddReactor("create", rbac.SchemeGroupVersion.WithResource("rolebindings").String(), assertRBNotCreated(t))

	assert.Equals(roll.Status.ServiceMeshReconciledVersion != controlPlane.Status.GetReconciledVersion(), true, "Unexpected Status.ServiceMeshReconciledVersion in SMMR already matches SMCP reconciled version", t)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.DeepEquals(updatedRoll.Status.ConfiguredMembers, []string{appNamespace}, "Unexpected Status.ConfiguredMembers in SMMR", t)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
	assert.Equals(updatedRoll.Status.ServiceMeshReconciledVersion, controlPlane.Status.GetReconciledVersion(), "Unexpected Status.ServiceMeshReconciledVersion in SMMR", t)

	meshNetAttachDefName := cniNetwork1_0
	assertNamespaceReconciled(t, cl, appNamespace, controlPlaneNamespace, meshNetAttachDefName, []rbac.RoleBinding{*meshRoleBinding})
}

func TestReconcileReconcilesAddedMember(t *testing.T) {
	cases := []struct {
		name                string
		operatorVersion     string
		meshVersion         string
		expectedNetworkName string
		upgradedOperator    bool
	}{
		{
			// tests a namespace add being processed before the mesh is upgraded
			name:                "v1.0-before-mesh-upgrade",
			operatorVersion:     operatorVersion1_0,
			meshVersion:         "",
			expectedNetworkName: cniNetwork1_0,
		},
		{
			// tests a namespace add being processed after the mesh is upgraded,
			// but before roll has been synced, i.e. simulates a mesh upgrade
			// _and_ a roll update hitting at the same time
			name:                "v1.0-after-mesh-upgrade",
			operatorVersion:     operatorVersion1_1,
			meshVersion:         "",
			expectedNetworkName: cniNetwork1_0,
			upgradedOperator:    true,
		},
		{
			name:                "v1.0-installed-with-v1.1",
			operatorVersion:     operatorVersion1_1,
			meshVersion:         versions.V1_0.String(),
			expectedNetworkName: cniNetwork1_0,
		},
		{
			name:                "v1.1",
			operatorVersion:     operatorVersion1_1,
			meshVersion:         versions.V1_1.String(),
			expectedNetworkName: cniNetwork1_1,
		},
		{
			name:                "default",
			operatorVersion:     operatorVersionDefault,
			meshVersion:         "",
			expectedNetworkName: cniNetwork1_0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roll := newMemberRoll(2, 1, 1, tc.operatorVersion)
			addOwnerReference(roll)
			roll.Spec.Members = []string{appNamespace}
			controlPlane := markControlPlaneReconciled(newControlPlane(tc.meshVersion), tc.operatorVersion)
			if tc.upgradedOperator {
				// need to reset the ServiceMeshReconciledVersion
				roll.Status.ServiceMeshReconciledVersion = maistrav1.ComposeReconciledVersion(operatorVersion1_0, controlPlane.GetGeneration())
			}
			roll.Status.ServiceMeshGeneration = controlPlane.Status.ObservedGeneration
			namespace := newNamespace(appNamespace)
			meshRoleBinding := newMeshRoleBinding()

			cl, _, r, nsReconciler, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, namespace)

			assertReconcileSucceeds(r, t)

			updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
			assert.DeepEquals(updatedRoll.Status.ConfiguredMembers, []string{appNamespace}, "Unexpected Status.ConfiguredMembers in SMMR", t)
			assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)

			assertNamespaceReconciled(t, cl, appNamespace, controlPlaneNamespace, tc.expectedNetworkName, []rbac.RoleBinding{*meshRoleBinding})
			assertNamespaceReconcilerInvoked(t, nsReconciler, appNamespace)
			kialiReconciler.assertInvokedWith(t, appNamespace)
		})
	}

}

func TestReconcileFailsIfMemberRollUpdateFails(t *testing.T) {
	roll := newMemberRoll(2, 1, 1, operatorVersionDefault)
	addOwnerReference(roll)
	roll.Spec.Members = []string{appNamespace}
	controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
	namespace := newNamespace(appNamespace)

	_, tracker, r, nsReconciler, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, namespace)
	tracker.AddReactor("update", "servicemeshmemberrolls", test.ClientFails())

	assertReconcileFails(r, t)

	assertNamespaceReconcilerInvoked(t, nsReconciler, appNamespace)
	kialiReconciler.assertInvokedWith(t, appNamespace)
}

func TestReconcileFailsIfKialiReconcileFails(t *testing.T) {
	roll := newMemberRoll(2, 1, 1, operatorVersionDefault)
	addOwnerReference(roll)
	roll.Spec.Members = []string{appNamespace}
	controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
	namespace := newNamespace(appNamespace)

	_, _, r, nsReconciler, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, namespace)
	kialiReconciler.errorToReturn = fmt.Errorf("error")

	assertReconcileFails(r, t)

	assertNamespaceReconcilerInvoked(t, nsReconciler, appNamespace)
	kialiReconciler.assertInvokedWith(t, appNamespace)
}

func TestReconcileReconcilesMemberIfNamespaceIsCreatedLater(t *testing.T) {
	roll := newDefaultMemberRoll()
	addOwnerReference(roll)
	roll.Spec.Members = []string{appNamespace}
	roll.ObjectMeta.Generation = 2
	roll.Status.ObservedGeneration = 2 // NOTE: generation 2 of the member roll has already been reconciled
	controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
	roll.Status.ServiceMeshGeneration = controlPlane.Status.ObservedGeneration
	meshRoleBinding := newMeshRoleBinding()
	namespace := newNamespace(appNamespace)

	cl, _, r, nsReconciler, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, namespace, meshRoleBinding)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.DeepEquals(updatedRoll.Status.ConfiguredMembers, []string{appNamespace}, "Unexpected Status.ConfiguredMembers in SMMR", t)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)

	assertNamespaceReconcilerInvoked(t, nsReconciler, appNamespace)
	meshNetAttachDefName, _ := cni.GetNetworkName(versions.LegacyVersion)
	assertNamespaceReconciled(t, cl, appNamespace, controlPlaneNamespace, meshNetAttachDefName, []rbac.RoleBinding{*meshRoleBinding})

	// invoke reconcile again to check if the Status.ServiceMeshGeneration field is updated
	assertReconcileSucceeds(r, t)
	updatedRoll = test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
	kialiReconciler.assertInvokedWith(t, appNamespace)
}

func TestReconcileUpdatesMemberListWhenNamespaceIsDeleted(t *testing.T) {
	roll := newMemberRoll(1, 1, 1, operatorVersionDefault)
	addOwnerReference(roll)
	roll.Spec.Members = []string{controlPlaneNamespace, appNamespace, appNamespace2}
	roll.Status.ConfiguredMembers = []string{appNamespace, appNamespace2}
	controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
	namespace := newNamespace(appNamespace)

	cl, _, r, _, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, namespace) // NOTE: no appNamespace2

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.DeepEquals(updatedRoll.Status.ConfiguredMembers, []string{appNamespace}, "Unexpected Status.ConfiguredMembers in SMMR", t)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
	kialiReconciler.assertInvokedWith(t, appNamespace)
}

func TestReconcileDoesNotUpdateMemberRollWhenNothingToReconcile(t *testing.T) {
	roll := newMemberRoll(2, 2, 1, operatorVersionDefault)
	addOwnerReference(roll)
	roll.Spec.Members = []string{appNamespace}
	roll.Status.ConfiguredMembers = []string{appNamespace}

	controlPlane := newControlPlane(versions.DefaultVersion.String())
	controlPlane.SetGeneration(2)
	markControlPlaneReconciled(controlPlane, operatorVersionDefault)

	namespace := newNamespace(appNamespace)
	common.SetLabel(namespace, common.MemberOfKey, controlPlaneNamespace)

	kialiCR := createKialiResource(controlPlaneNamespace, appNamespace)

	nad := createNAD(cniNetworkDefault, appNamespace, controlPlaneNamespace)

	_, tracker, r, _, _ := createClientAndReconciler(t, roll, controlPlane, namespace, nad, kialiCR)

	assertReconcileSucceeds(r, t)

	actions := tracker.Actions()
	test.AssertNumberOfWriteActions(t, actions, 1)
	if updatedObj, err := tracker.Get(maistrav1.SchemeBuilder.GroupVersion.WithResource("servicemeshmemberrolls"), controlPlaneNamespace, "default"); err != nil {
		t.Errorf("Unexpected error retrieving updated ServiceMeshMemberRoll: %v", err)
	} else if updatedRoll, ok := updatedObj.(*maistrav1.ServiceMeshMemberRoll); !ok {
		t.Errorf("Unexpected error casting runtime.Object to ServiceMeshMemberRoll: %v", updatedObj)
	} else if updatedRoll.Status.ServiceMeshReconciledVersion != controlPlane.Status.ReconciledVersion {
		t.Errorf("ServiceMeshMemberRoll was not updated")
	}
}

func TestReconcileNamespacesIgnoresControlPlaneNamespace(t *testing.T) {
	namespace := newNamespace(appNamespace)

	_, _, r, nsReconciler, _ := createClientAndReconciler(t, namespace)

	reqLogger := logf.Log.WithName("testLog").WithValues("ServiceMeshMemberRoll", request)
	ctx := common.NewContextWithLog(ctx, reqLogger)

	namespaces := sets.NewString(controlPlaneNamespace, appNamespace)
	configuredMembers, err, nsErrors := r.reconcileNamespaces(ctx, namespaces, namespaces, controlPlaneNamespace, versions.DefaultVersion)
	if err != nil {
		t.Fatalf("reconcileNamespaces failed: %v", err)
	}
	if len(nsErrors) > 0 {
		t.Fatalf("reconcileNamespaces returned unexpected nsErrors: %v", nsErrors)
	}

	assertNamespaceReconcilerInvoked(t, nsReconciler, appNamespace) // NOTE: no controlPlaneNamespace
	assertNamespaceRemoveInvoked(t, nsReconciler, appNamespace)     // NOTE: no controlPlaneNamespace
	assert.DeepEquals(configuredMembers, []string{appNamespace}, "reconcileNamespaces returned an unexpected configuredMembers list", t)
}

func TestReconcileWorksWithMultipleNamespaces(t *testing.T) {
	controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
	roll := newDefaultMemberRoll()
	addOwnerReference(roll)
	roll.Spec.Members = []string{appNamespace, appNamespace2}
	roll.ObjectMeta.Generation = 2
	roll.Status.ObservedGeneration = 1
	roll.Status.ServiceMeshGeneration = controlPlane.Status.ObservedGeneration

	cl, _, r, _, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, newNamespace(appNamespace))
	assertReconcileSucceeds(r, t)
	test.PanicOnError(cl.Create(context.TODO(), newNamespace(appNamespace2)))
	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.StringArrayContains(updatedRoll.Status.ConfiguredMembers, appNamespace, "Expected Status.ConfiguredMembers to contain "+appNamespace, t)
	assert.StringArrayContains(updatedRoll.Status.ConfiguredMembers, appNamespace2, "Expected Status.ConfiguredMembers to contain "+appNamespace2, t)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
	kialiReconciler.assertInvokedWith(t, appNamespace, appNamespace2)
}

func assertNamespaceReconcilerInvoked(t *testing.T, nsReconciler *fakeNamespaceReconciler, namespaces ...string) {
	assert.DeepEquals(nsReconciler.reconciledNamespaces, namespaces, "Expected namespace reconciler to be invoked, but it wasn't invoked or wasn't invoked properly", t)
}

func assertNamespaceRemoveInvoked(t *testing.T, nsReconciler *fakeNamespaceReconciler, namespaces ...string) {
	assert.DeepEquals(nsReconciler.removedNamespaces, namespaces, "Expected removal to be invoked for namespace, but it wasn't or wasn't invoked properly", t)
}

// TODO: add test that checks if one namespace is missing, but another is present, the latter gets reconciled and reconcile does NOT return an error

func TestReconcileDoesNotAddControlPlaneNamespaceToMembers(t *testing.T) {
	roll := newDefaultMemberRoll()
	addOwnerReference(roll)
	roll.Spec.Members = []string{controlPlaneNamespace}
	roll.ObjectMeta.Generation = 2
	roll.Status.ObservedGeneration = 1
	controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
	roll.Status.ServiceMeshGeneration = controlPlane.Status.ObservedGeneration
	namespace := &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: controlPlaneNamespace,
		},
	}

	cl, _, r, _, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, namespace)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.StringArrayEmpty(updatedRoll.Status.ConfiguredMembers, "Expected Status.ConfiguredMembers in SMMR to be empty, but it wasn't.", t)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
	kialiReconciler.assertInvokedWith(t /* no namespaces */)
}

func TestReconcileRemovesFinalizerFromMemberRoll(t *testing.T) {
	roll := newDefaultMemberRoll()
	roll.DeletionTimestamp = &oneMinuteAgo

	cl, _, r, _, _ := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.StringArrayEmpty(updatedRoll.Finalizers, "Expected finalizers list in SMMR to be empty, but it wasn't", t)
}

func TestReconcileHandlesDeletionProperly(t *testing.T) {
	cases := []struct {
		name                      string
		specMembers               []string
		configuredMembers         []string
		expectedRemovedNamespaces []string
	}{
		{
			name:                      "normal-deletion",
			specMembers:               []string{appNamespace},
			configuredMembers:         []string{appNamespace},
			expectedRemovedNamespaces: []string{appNamespace},
		},
		{
			name:                      "ns-removed-from-members-list-and-smmr-deleted-immediately",
			specMembers:               []string{}, // appNamespace was removed, but then the SMMR was deleted immediately. The controller is reconciling both actions at once.
			configuredMembers:         []string{appNamespace},
			expectedRemovedNamespaces: []string{appNamespace},
		},
		// TODO: add a member, it gets configured by namespace reconciler, but then the SMMR update fails (configuredMembers doesn't include the namespace). Then the SMMR is deleted. Does the namespace get cleaned up?
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roll := newDefaultMemberRoll()
			roll.Spec.Members = tc.specMembers
			roll.Status.ConfiguredMembers = tc.configuredMembers
			roll.DeletionTimestamp = &oneMinuteAgo

			initObjects := []runtime.Object{roll}
			for _, ns := range tc.configuredMembers {
				initObjects = append(initObjects, &core.Namespace{
					ObjectMeta: meta.ObjectMeta{
						Name: ns,
						Labels: map[string]string{
							common.MemberOfKey: controlPlaneNamespace,
						},
					},
				})
			}

			cl, _, r, nsReconciler, kialiReconciler := createClientAndReconciler(t, initObjects...)

			assertReconcileSucceeds(r, t)

			updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
			assert.StringArrayEmpty(updatedRoll.Finalizers, "Expected finalizers list in SMMR to be empty, but it wasn't", t)

			assertNamespaceRemoveInvoked(t, nsReconciler, tc.expectedRemovedNamespaces...)
			kialiReconciler.assertInvokedWith(t /* no namespaces */)
		})
	}
}

// TODO: removal of namespace from SMMR.spec.members - does it get cleaned up?

// TODO: test reconcileNamespaces() - including cases where namespace is NotFound or Gone (shouldn't be an error)

func TestClientReturnsErrorWhenRemovingFinalizer(t *testing.T) {
	cases := []struct {
		name                 string
		reactor              clienttesting.Reactor
		successExpected      bool
		expectedWriteActions int
	}{
		{
			name:                 "get-memberroll-returns-notfound",
			reactor:              test.On("get", "servicemeshmemberrolls", test.AttemptNumber(2, test.ClientReturnsNotFound(maistrav1.APIGroup, "ServiceMeshMemberRoll", memberRollName))),
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
			reactor:              test.On("update", "servicemeshmemberrolls", test.ClientReturnsNotFound(maistrav1.APIGroup, "ServiceMeshMemberRoll", memberRollName)),
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
			roll := newDefaultMemberRoll()
			roll.DeletionTimestamp = &oneMinuteAgo

			_, tracker, r, _, _ := createClientAndReconciler(t, roll)
			tracker.AddReaction(tc.reactor)

			if tc.successExpected {
				assertReconcileSucceeds(r, t)
			} else {
				assertReconcileFails(r, t)
			}
			test.AssertNumberOfWriteActions(t, tracker.Actions(), tc.expectedWriteActions)
		})
	}
}

func createClientAndReconciler(t *testing.T, clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, *MemberRollReconciler, *fakeNamespaceReconciler, *fakeKialiReconciler) {

	cl, enhancedTracker := test.CreateClient(clientObjects...)

	rf := fakeNamespaceReconcilerFactory{
		reconciler: &fakeNamespaceReconciler{},
	}

	fakeEventRecorder := &record.FakeRecorder{}
	kialiReconciler := &fakeKialiReconciler{}
	cniConfig := cni.Config{Enabled: true}

	r := newReconciler(cl, scheme.Scheme, fakeEventRecorder, rf.newReconciler, kialiReconciler, cniConfig)

	return cl, enhancedTracker, r, rf.reconciler, kialiReconciler
}

type fakeNamespaceReconcilerFactory struct {
	reconciler *fakeNamespaceReconciler
}

func (rf *fakeNamespaceReconcilerFactory) newReconciler(ctx context.Context, cl client.Client, meshNamespace string, meshVersion versions.Version, isCNIEnabled bool) (NamespaceReconciler, error) {
	delegate, err := newNamespaceReconciler(ctx, cl, meshNamespace, meshVersion, isCNIEnabled)
	rf.reconciler.delegate = delegate
	return rf.reconciler, err
}

type fakeNamespaceReconciler struct {
	reconciledNamespaces []string
	removedNamespaces    []string
	delegate             NamespaceReconciler
}

func (r *fakeNamespaceReconciler) reconcileNamespaceInMesh(ctx context.Context, namespace string) error {
	r.reconciledNamespaces = append(r.reconciledNamespaces, namespace)
	return r.delegate.reconcileNamespaceInMesh(ctx, namespace)
}

func (r *fakeNamespaceReconciler) removeNamespaceFromMesh(ctx context.Context, namespace string) error {
	r.removedNamespaces = append(r.removedNamespaces, namespace)
	return r.delegate.removeNamespaceFromMesh(ctx, namespace)
}

func assertReconcileSucceeds(r *MemberRollReconciler, t *testing.T) {
	res, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if res.Requeue {
		t.Error("Reconcile requeued the request, but it shouldn't have")
	}
}

func assertReconcileFails(r *MemberRollReconciler, t *testing.T) {
	_, err := r.Reconcile(request)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}
}

func newDefaultMemberRoll() *maistrav1.ServiceMeshMemberRoll {
	return newMemberRoll(1, 1, 1, operatorVersionDefault)
}

func newMemberRoll(generation int64, observedGeneration int64, observedMeshGeneration int64, operatorVersion string) *maistrav1.ServiceMeshMemberRoll {
	return &maistrav1.ServiceMeshMemberRoll{
		ObjectMeta: meta.ObjectMeta{
			Name:       memberRollName,
			Namespace:  controlPlaneNamespace,
			Finalizers: []string{common.FinalizerName},
			Generation: generation,
			UID:        memberRollUID,
		},
		Spec: maistrav1.ServiceMeshMemberRollSpec{
			Members: []string{},
		},
		Status: maistrav1.ServiceMeshMemberRollStatus{
			ObservedGeneration:           observedGeneration,
			ServiceMeshGeneration:        observedMeshGeneration,
			ServiceMeshReconciledVersion: maistrav1.ComposeReconciledVersion(operatorVersion, observedMeshGeneration),
		},
	}
}

func addOwnerReference(roll *maistrav1.ServiceMeshMemberRoll) *maistrav1.ServiceMeshMemberRoll {
	roll.OwnerReferences = []meta.OwnerReference{
		{
			APIVersion: maistrav1.SchemeGroupVersion.String(),
			Kind:       "ServiceMeshControlPlane",
			Name:       controlPlaneName,
			UID:        controlPlaneUID,
		},
	}
	return roll
}

func newControlPlane(version string) *maistrav1.ServiceMeshControlPlane {
	return &maistrav1.ServiceMeshControlPlane{
		ObjectMeta: meta.ObjectMeta{
			Name:       controlPlaneName,
			Namespace:  controlPlaneNamespace,
			UID:        controlPlaneUID,
			Generation: 1,
		},
		Spec: maistrav1.ControlPlaneSpec{
			Version: version,
		},
	}
}

func markControlPlaneReconciled(controlPlane *maistrav1.ServiceMeshControlPlane, operatorVersion string) *maistrav1.ServiceMeshControlPlane {
	controlPlane.Status.ObservedGeneration = controlPlane.GetGeneration()
	controlPlane.Status.Conditions = []maistrav1.Condition{
		{
			Type:   maistrav1.ConditionTypeReconciled,
			Status: maistrav1.ConditionStatusTrue,
		},
	}
	controlPlane.Status.ReconciledVersion = maistrav1.ComposeReconciledVersion(operatorVersion, controlPlane.GetGeneration())
	return controlPlane
}

func newNamespace(name string) *core.Namespace {
	namespace := &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
		},
	}
	return namespace
}

func newRoleBinding(namespace, name string) *rbac.RoleBinding {
	return &rbac.RoleBinding{
		ObjectMeta: meta.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		RoleRef: rbac.RoleRef{APIGroup: rbac.GroupName},
	}
}

func newMeshRoleBinding() *rbac.RoleBinding {
	roleBinding := newRoleBinding(controlPlaneNamespace, "role-binding")
	roleBinding.Labels = map[string]string{}
	roleBinding.Labels[common.OwnerKey] = controlPlaneNamespace
	return roleBinding
}

func newAppNamespaceRoleBinding() *rbac.RoleBinding {
	roleBinding := newRoleBinding(appNamespace, "role-binding")
	roleBinding.Labels = map[string]string{}
	roleBinding.Labels[common.OwnerKey] = controlPlaneNamespace
	roleBinding.Labels[common.MemberOfKey] = controlPlaneNamespace
	return roleBinding
}

func assertNamespaceReconciled(t *testing.T, cl client.Client, namespace, meshNamespace string, meshNetAttachDefName string, meshRoleBindings []rbac.RoleBinding) {
	// check if namespace has member-of label
	ns := &core.Namespace{}
	test.GetObject(ctx, cl, types.NamespacedName{Name: namespace}, ns)
	assert.Equals(ns.Labels[common.MemberOfKey], meshNamespace, "Unexpected or missing member-of label in namespace", t)

	// check if net-attach-def exists
	netAttachDef := &unstructured.Unstructured{}
	netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})
	err := cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: meshNetAttachDefName}, netAttachDef)
	if err != nil {
		t.Fatalf("Couldn't get NetworkAttachmentDefinition from client: %v", err)
	}
}

type fakeKialiReconciler struct {
	reconcileKialiInvoked  bool
	kialiConfiguredMembers []string
	errorToReturn          error
	delegate               func(ctx context.Context, kialiCRNamespace string, configuredMembers []string) error
}

func (f *fakeKialiReconciler) reconcileKiali(ctx context.Context, kialiCRNamespace string, configuredMembers []string) error {
	f.reconcileKialiInvoked = true
	f.kialiConfiguredMembers = append([]string{}, configuredMembers...)
	if f.errorToReturn != nil {
		return f.errorToReturn
	}
	if f.delegate != nil {
		return f.delegate(ctx, kialiCRNamespace, configuredMembers)
	}
	return nil
}

func createKialiResource(controlPlaneNamespace string, members ...string) runtime.Object {
	kialiCRName := "kiali"
	kialiCR := &unstructured.Unstructured{}
	kialiCR.SetAPIVersion("kiali.io/v1alpha1")
	kialiCR.SetKind("Kiali")
	kialiCR.SetNamespace(controlPlaneNamespace)
	kialiCR.SetName(kialiCRName)
	unstructured.SetNestedStringSlice(kialiCR.UnstructuredContent(), members, "spec", "deployment", "accessible_namespaces")
	return kialiCR
}

func (f *fakeKialiReconciler) assertInvokedWith(t *testing.T, namespaces ...string) {
	assert.True(f.reconcileKialiInvoked, "Expected reconcileKiali to be invoked, but it wasn't", t)
	if len(namespaces) != 0 || len(f.kialiConfiguredMembers) != 0 {
		assert.DeepEquals(f.kialiConfiguredMembers, namespaces, "reconcileKiali called with unexpected member list", t)
	}
}

func (f *fakeKialiReconciler) assertNotInvoked(t *testing.T) {
	assert.False(f.reconcileKialiInvoked, "Expected reconcileKiali not to be invoked, but it was", t)
}

func assertRBNotCreated(t *testing.T) clienttesting.ReactionFunc {
	return func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Errorf("Unexpected creation of RoleBinding")
		return false, nil, nil
	}
}

func assertNADNotDeleted(t *testing.T) clienttesting.ReactionFunc {
	return func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		t.Errorf("Unexpected deletion of CNI NetworkAttachmentDefinition")
		return false, nil, nil
	}
}

func createNAD(name, appNamespace, cpNamespace string) runtime.Object {
	netAttachDef := &unstructured.Unstructured{}
	netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})
	netAttachDef.SetNamespace(appNamespace)
	netAttachDef.SetName(name)
	common.SetLabel(netAttachDef, common.MemberOfKey, cpNamespace)
	return netAttachDef
}
