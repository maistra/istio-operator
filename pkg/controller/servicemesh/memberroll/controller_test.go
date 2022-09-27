package memberroll

import (
	"context"
	"fmt"
	"testing"
	"time"

	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
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

	memberUID = types.UID("3333")

	operatorVersion1_1     = "1.1.0"
	operatorVersionDefault = operatorVersion1_1

	cniNetwork1_1     = "v1-1-istio-cni"
	cniNetwork2_0     = "v2-0-istio-cni"
	cniNetwork2_1     = "v2-1-istio-cni"
	cniNetworkDefault = cniNetwork2_1
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

	cl, _, r, _ := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.DeepEquals(updatedRoll.GetFinalizers(), []string{common.FinalizerName}, "Unexpected finalizers in SMM", t)
}

func TestReconcileFailsIfItCannotAddFinalizer(t *testing.T) {
	roll := newDefaultMemberRoll()
	roll.Finalizers = []string{}

	_, tracker, r, _ := createClientAndReconciler(t, roll)
	tracker.AddReactor("update", "servicemeshmemberrolls", test.ClientFails())
	assertReconcileFails(r, t)
}

func TestReconcileDoesNothingIfMemberRollIsDeletedAndHasNoFinalizers(t *testing.T) {
	roll := newDefaultMemberRoll()
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
	tracker.AddReactor("get", "servicemeshmemberrolls", test.ClientFails())
	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileFailsWhenListControlPlanesFails(t *testing.T) {
	roll := newDefaultMemberRoll()
	_, tracker, r, _ := createClientAndReconciler(t, roll)
	tracker.AddReactor("list", "servicemeshcontrolplanes", test.ClientFails())

	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingIfControlPlaneMissing(t *testing.T) {
	roll := newDefaultMemberRoll()
	cl, tracker, r, _ := createClientAndReconciler(t, roll)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assertConditions(updatedRoll, []maistrav1.ServiceMeshMemberRollCondition{
		{
			Type:    maistrav1.ConditionTypeMemberRollReady,
			Status:  core.ConditionFalse,
			Reason:  maistrav1.ConditionReasonSMCPMissing,
			Message: "No ServiceMeshControlPlane exists in the namespace",
		},
	}, t)
}

func TestReconcileDoesNothingIfMultipleControlPlanesFound(t *testing.T) {
	roll := newDefaultMemberRoll()
	controlPlane1 := newControlPlane("")
	controlPlane2 := newControlPlane("")
	controlPlane2.Name = "my-mesh-2"
	cl, tracker, r, _ := createClientAndReconciler(t, roll, controlPlane1, controlPlane2)
	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assertConditions(updatedRoll, []maistrav1.ServiceMeshMemberRollCondition{
		{
			Type:    maistrav1.ConditionTypeMemberRollReady,
			Status:  core.ConditionFalse,
			Reason:  maistrav1.ConditionReasonMultipleSMCP,
			Message: "Multiple ServiceMeshControlPlane resources exist in the namespace",
		},
	}, t)
}

func TestReconcileFailsIfListingMembersFails(t *testing.T) {
	roll := newDefaultMemberRoll()
	controlPlane := newControlPlane("")
	markControlPlaneReconciled(controlPlane, operatorVersionDefault)

	_, tracker, r, _ := createClientAndReconciler(t, roll, controlPlane)
	tracker.AddReactor("list", "servicemeshmembers", test.ClientFails())

	assertReconcileFails(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileCreatesMember(t *testing.T) {
	cases := []struct {
		name                   string
		reactor                clienttesting.Reactor
		existingObjects        []runtime.Object
		specMembers            []string
		expectMembersCreated   []string
		expectedStatusMembers  []string
		expectedPendingMembers []string
	}{
		{
			name:                   "namespace-exists",
			existingObjects:        []runtime.Object{newNamespace(appNamespace)},
			specMembers:            []string{appNamespace},
			expectedStatusMembers:  []string{appNamespace},
			expectedPendingMembers: []string{appNamespace},
			expectMembersCreated:   []string{appNamespace},
		},
		{
			name:                   "namespace-not-exists",
			reactor:                test.On("create", "servicemeshmembers", test.ClientReturnsNotFound(maistrav1.APIGroup, "ServiceMeshMember", common.MemberName)),
			specMembers:            []string{appNamespace},
			expectedStatusMembers:  []string{appNamespace},
			expectedPendingMembers: []string{appNamespace},
			expectMembersCreated:   []string{},
		},
		{
			name:                   "multiple-members",
			existingObjects:        []runtime.Object{newNamespace(appNamespace), newNamespace(appNamespace2)},
			specMembers:            []string{appNamespace, appNamespace2},
			expectedStatusMembers:  []string{appNamespace, appNamespace2},
			expectedPendingMembers: []string{appNamespace, appNamespace2},
			expectMembersCreated:   []string{appNamespace, appNamespace2},
		},
		{
			name:                   "control-plane-as-member",
			existingObjects:        []runtime.Object{newNamespace(controlPlaneNamespace)},
			specMembers:            []string{controlPlaneNamespace},
			expectedStatusMembers:  []string{},
			expectedPendingMembers: []string{},
			expectMembersCreated:   []string{},
		},
		{
			name:                   "member-exists",
			existingObjects:        []runtime.Object{newNamespace(appNamespace), newMember()},
			specMembers:            []string{appNamespace},
			expectedStatusMembers:  []string{appNamespace},
			expectedPendingMembers: []string{appNamespace},
			expectMembersCreated:   []string{},
		},
		{
			name: "member-exists-but-points-to-different-control-plane",
			existingObjects: []runtime.Object{
				newNamespace(appNamespace),
				markMemberReconciled(
					newMemberWithRef("other-mesh", "other-mesh-namespace"),
					1, 1, 1, operatorVersionDefault)},
			specMembers:            []string{appNamespace},
			expectedStatusMembers:  []string{appNamespace},
			expectedPendingMembers: []string{appNamespace},
			expectMembersCreated:   []string{},
		},
		// TODO: add namespace that contains a different control plane as a member
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roll := newDefaultMemberRoll()
			roll.Spec.Members = tc.specMembers
			controlPlane := newControlPlane("")
			markControlPlaneReconciled(controlPlane, operatorVersionDefault)

			objects := []runtime.Object{roll, controlPlane}
			objects = append(objects, tc.existingObjects...)
			cl, _, r, _ := createClientAndReconciler(t, objects...)

			assertReconcileSucceeds(r, t)

			for _, ns := range tc.expectMembersCreated {
				assertMemberCreated(cl, t, ns, controlPlaneName, controlPlaneNamespace)
			}

			updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
			assertStatusMembers(updatedRoll, tc.expectedStatusMembers, tc.expectedPendingMembers, []string{}, []string{}, t)
			assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
			assert.Equals(updatedRoll.Status.ServiceMeshReconciledVersion, controlPlane.Status.GetReconciledVersion(), "Unexpected Status.ServiceMeshReconciledVersion in SMMR", t)
		})
	}
}

func TestReconcileCreatesMemberWhenAppNamespaceIsCreated(t *testing.T) {
	roll := newDefaultMemberRoll()
	roll.Spec.Members = []string{appNamespace}
	roll.ObjectMeta.Generation = 2
	roll.Status.ObservedGeneration = 2 // NOTE: generation 2 of the member roll has already been reconciled
	controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
	roll.Status.ServiceMeshGeneration = controlPlane.Status.ObservedGeneration
	namespace := newNamespace(appNamespace)

	cl, _, r, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, namespace)

	assertReconcileSucceeds(r, t)

	assertMemberCreated(cl, t, appNamespace, controlPlaneName, controlPlaneNamespace)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assertStatusMembers(updatedRoll, []string{appNamespace}, []string{appNamespace}, []string{}, []string{}, t)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)

	kialiReconciler.assertInvokedWith(t, appNamespace)
}

func TestReconcileDeletesMemberWhenRemovedFromSpecMembers(t *testing.T) {
	cases := []struct {
		name                       string
		reactor                    clienttesting.Reactor
		initMemberFunc             func(*maistrav1.ServiceMeshMember)
		memberNamespace            string
		expectedStatusMembers      []string
		expectedPendingMembers     []string
		expectedConfiguredMembers  []string
		expectedTerminatingMembers []string
		expectMemberDeleted        bool
	}{
		{
			name:            "created-by-controller",
			memberNamespace: appNamespace,
			initMemberFunc: func(member *maistrav1.ServiceMeshMember) {
				if member.Annotations == nil {
					member.Annotations = map[string]string{}
				}
				member.Annotations[common.CreatedByKey] = controllerName
			},
			expectMemberDeleted:        true,
			expectedStatusMembers:      []string{appNamespace},
			expectedConfiguredMembers:  []string{appNamespace},
			expectedPendingMembers:     []string{},
			expectedTerminatingMembers: []string{},
		},
		{
			name:                       "created-manually",
			memberNamespace:            appNamespace,
			expectMemberDeleted:        false,
			expectedStatusMembers:      []string{appNamespace},
			expectedConfiguredMembers:  []string{appNamespace},
			expectedPendingMembers:     []string{},
			expectedTerminatingMembers: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			roll := newDefaultMemberRoll()
			controlPlane := newControlPlane("")
			markControlPlaneReconciled(controlPlane, operatorVersionDefault)

			member := newMember()
			markMemberReconciled(member, 1, 1, controlPlane.Status.ObservedGeneration, controlPlane.Status.OperatorVersion)
			if tc.initMemberFunc != nil {
				tc.initMemberFunc(member)
			}

			cl, _, r, _ := createClientAndReconciler(t, member, roll, controlPlane)

			assertReconcileSucceeds(r, t)

			err := cl.Get(ctx, client.ObjectKey{Name: common.MemberName, Namespace: tc.memberNamespace}, &maistrav1.ServiceMeshMember{})
			memberExists := true
			if err != nil {
				if errors.IsNotFound(err) {
					memberExists = false
				} else {
					t.Fatalf("Unexpected error %v", err)
				}
			}

			if tc.expectMemberDeleted {
				if memberExists {
					t.Fatalf("Expected controller to delete ServiceMeshMember, but it didn't")
				}
			} else {
				if !memberExists {
					t.Fatalf("Expected controller to preserve ServiceMeshMember, but it has deleted it")
				}
			}

			updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
			assertStatusMembers(updatedRoll, tc.expectedStatusMembers, tc.expectedPendingMembers, tc.expectedConfiguredMembers, tc.expectedTerminatingMembers, t)
			assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
			assert.Equals(updatedRoll.Status.ServiceMeshReconciledVersion, controlPlane.Status.GetReconciledVersion(), "Unexpected Status.ServiceMeshReconciledVersion in SMMR", t)
		})
	}
}

func TestReconcileFailsIfMemberRollUpdateFails(t *testing.T) {
	roll := newMemberRoll(2, 1, 1, operatorVersionDefault)
	roll.Spec.Members = []string{appNamespace}
	controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
	namespace := newNamespace(appNamespace)

	_, tracker, r, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, namespace)
	tracker.AddReactor("patch", "servicemeshmemberrolls", test.ClientFails())

	assertReconcileFails(r, t)

	// assertNamespaceReconcilerInvoked(t, nsReconciler, appNamespace)
	kialiReconciler.assertInvokedWith(t, appNamespace)
}

func TestReconcileFailsIfKialiReconcileFails(t *testing.T) {
	roll := newMemberRoll(2, 1, 1, operatorVersionDefault)
	roll.Spec.Members = []string{appNamespace}
	controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
	namespace := newNamespace(appNamespace)

	_, _, r, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, namespace)
	kialiReconciler.errorToReturn = fmt.Errorf("error")

	assertReconcileFails(r, t)

	kialiReconciler.assertInvokedWith(t, appNamespace)
}

func TestReconcileUpdatesMembersInStatusWhenMemberIsDeleted(t *testing.T) {
	roll := newMemberRoll(1, 1, 1, operatorVersionDefault)
	roll.Spec.Members = []string{appNamespace}
	roll.Status.Members = []string{appNamespace}
	roll.Status.ConfiguredMembers = []string{appNamespace}

	controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
	namespace := newNamespace(appNamespace)

	cl, tracker, r, kialiReconciler := createClientAndReconciler(t, roll, controlPlane, namespace)
	tracker.AddReactor("create", "servicemeshmembers", test.ClientReturnsNotFound(maistrav1.APIGroup, "ServiceMeshMember", common.MemberName))

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assertStatusMembers(updatedRoll, []string{appNamespace}, []string{appNamespace}, []string{}, []string{}, t)
	assert.Equals(updatedRoll.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
	kialiReconciler.assertInvokedWith(t, appNamespace)
}

func TestReconcileClearsConfiguredMembersWhenSMCPDeleted(t *testing.T) {
	roll := newMemberRoll(1, 1, 1, operatorVersionDefault)
	roll.Spec.Members = []string{appNamespace}
	roll.Status.Members = []string{appNamespace}
	roll.Status.PendingMembers = []string{}
	roll.Status.ConfiguredMembers = []string{appNamespace}
	roll.Status.TerminatingMembers = []string{}
	roll.Status.MemberStatuses = []maistrav1.ServiceMeshMemberStatusSummary{
		{
			Namespace: appNamespace,
			Conditions: []maistrav1.ServiceMeshMemberCondition{
				{
					Type:   maistrav1.ConditionTypeMemberReconciled,
					Status: core.ConditionTrue,
				},
			},
		},
	}

	namespace := newNamespace(appNamespace)

	cl, _, r, _ := createClientAndReconciler(t, roll, namespace)

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assertStatusMembers(updatedRoll,
		[]string{appNamespace}, // members
		[]string{appNamespace}, // pending
		[]string{},             // configured
		[]string{},             // terminating
		t)
}

func TestReconcileAddsMembersToCorrectStatusField(t *testing.T) {
	smcp := newControlPlane()
	markControlPlaneReconciled(smcp)

	smmr := newMemberRoll(1)
	smmr.Spec.Members = []string{"unconfigured", "out-of-date", "up-to-date", "terminating"}

	unconfigured := newMemberWithNamespace("unconfigured")

	configuredButOutOfDate := newMemberWithNamespace("out-of-date")
	markMemberReconciled(configuredButOutOfDate, 1, 1, smcp.ObjectMeta.Generation-1, smcp.Status.OperatorVersion)

	configuredAndUpToDate := newMemberWithNamespace("up-to-date")
	markMemberReconciled(configuredAndUpToDate, 1, 1, smcp.ObjectMeta.Generation, smcp.Status.OperatorVersion)

	terminating := newMemberWithNamespace("terminating")
	markMemberReconciled(configuredAndUpToDate, 1, 1, smcp.ObjectMeta.Generation, smcp.Status.OperatorVersion)
	terminating.ObjectMeta.DeletionTimestamp = &oneMinuteAgo

	cl, _, r, _ := createClientAndReconciler(smcp, smmr, unconfigured, configuredButOutOfDate, configuredAndUpToDate, terminating,
		newNamespace("unconfigured"), newNamespace("out-of-date"), newNamespace("up-to-date"), newNamespace("terminating"))

	assertReconcileSucceeds(r, t)

	updatedRoll := test.GetUpdatedObject(ctx, cl, smmr.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assertStatusMembers(updatedRoll,
		[]string{"out-of-date", "terminating", "unconfigured", "up-to-date"}, // members
		[]string{"out-of-date", "unconfigured"},                              // pending
		[]string{"out-of-date", "up-to-date"},                                // configured
		[]string{"terminating"},                                              // terminating
		t)
}

func TestReconcileRemovesFinalizerFromMemberRoll(t *testing.T) {
	roll := newDefaultMemberRoll()
	roll.DeletionTimestamp = &oneMinuteAgo

	cl, _, r, _ := createClientAndReconciler(t, roll)

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
		noKiali                   bool
	}{
		{
			name:                      "normal-deletion",
			specMembers:               []string{appNamespace},
			configuredMembers:         []string{appNamespace},
			expectedRemovedNamespaces: []string{appNamespace},
		},
		{
			name:                      "normal-deletion-no-kiali",
			specMembers:               []string{appNamespace},
			configuredMembers:         []string{appNamespace},
			expectedRemovedNamespaces: []string{appNamespace},
			noKiali:                   true,
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
			controlPlane := newControlPlane("")
			if tc.noKiali {
				controlPlane.Status.AppliedSpec.Addons = nil
			}
			markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)

			roll := newDefaultMemberRoll()
			roll.Spec.Members = tc.specMembers
			roll.Status.ConfiguredMembers = tc.configuredMembers
			if !tc.noKiali {
				if roll.Status.Annotations == nil {
					roll.Status.Annotations = map[string]string{}
				}
				roll.Status.Annotations[statusAnnotationKialiName] = "kiali"
			}
			roll.DeletionTimestamp = &oneMinuteAgo

			initObjects := []runtime.Object{roll, controlPlane}
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

			cl, _, r, kialiReconciler := createClientAndReconciler(t, initObjects...)

			assertReconcileSucceeds(r, t)

			updatedRoll := test.GetUpdatedObject(ctx, cl, roll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
			assert.StringArrayEmpty(updatedRoll.Finalizers, "Expected finalizers list in SMMR to be empty, but it wasn't", t)

			// assertNamespaceRemoveInvoked(t, nsReconciler, tc.expectedRemovedNamespaces...)
			if !tc.noKiali {
				kialiReconciler.assertInvokedWith(t /* no namespaces */)
			}
		})
	}
}

func TestClientReturnsErrorWhenRemovingFinalizer(t *testing.T) {
	cases := []struct {
		name                 string
		reactor              clienttesting.Reactor
		successExpected      bool
		expectedWriteActions int
	}{
		{
			name:                 "get-memberroll-returns-notfound",
			reactor:              test.On("get", "servicemeshmemberrolls", test.ClientReturnsNotFound(maistrav1.APIGroup, "ServiceMeshMemberRoll", memberRollName)),
			successExpected:      true,
			expectedWriteActions: 0,
		},
		{
			name:                 "get-memberroll-fails",
			reactor:              test.On("get", "servicemeshmemberrolls", test.ClientFails()),
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
			controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)
			roll := newDefaultMemberRoll()
			roll.DeletionTimestamp = &oneMinuteAgo

			_, tracker, r, _ := createClientAndReconciler(t, roll, controlPlane)
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

func newMemberWithNamespace(ns string) *maistrav1.ServiceMeshMember {
	member := newMemberWithRef(controlPlaneName, controlPlaneNamespace)
	member.Namespace = ns
	return member
}

func newMember() *maistrav1.ServiceMeshMember {
	return newMemberWithRef(controlPlaneName, controlPlaneNamespace)
}

func newMemberWithRef(controlPlaneName, controlPlaneNamespace string) *maistrav1.ServiceMeshMember {
	return &maistrav1.ServiceMeshMember{
		ObjectMeta: meta.ObjectMeta{
			Name:       common.MemberName,
			Namespace:  appNamespace,
			Finalizers: []string{common.FinalizerName},
		},
		Spec: maistrav1.ServiceMeshMemberSpec{
			ControlPlaneRef: maistrav1.ServiceMeshControlPlaneRef{
				Name:      controlPlaneName,
				Namespace: controlPlaneNamespace,
			},
		},
	}
}

func markMemberReconciled(member *maistrav1.ServiceMeshMember, generation int64, observedGeneration int64, observedMeshGeneration int64, operatorVersion string) *maistrav1.ServiceMeshMember {
	member.Finalizers = []string{common.FinalizerName}
	member.Generation = generation
	member.UID = memberUID
	member.Status.ObservedGeneration = observedGeneration
	member.Status.ServiceMeshGeneration = observedMeshGeneration
	member.Status.ServiceMeshReconciledVersion = status.ComposeReconciledVersion(operatorVersion, observedMeshGeneration)

	member.Status.SetCondition(maistrav1.ServiceMeshMemberCondition{
		Type:    maistrav1.ConditionTypeMemberReconciled,
		Status:  common.BoolToConditionStatus(true),
		Reason:  "",
		Message: "",
	})
	member.Status.SetCondition(maistrav1.ServiceMeshMemberCondition{
		Type:    maistrav1.ConditionTypeMemberReady,
		Status:  common.BoolToConditionStatus(true),
		Reason:  "",
		Message: "",
	})
	return member
}

func createClientAndReconciler(t *testing.T, clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, *MemberRollReconciler, *fakeKialiReconciler) {
	cl, enhancedTracker := test.CreateClient(clientObjects...)

	fakeEventRecorder := &record.FakeRecorder{}
	kialiReconciler := &fakeKialiReconciler{}

	r := newReconciler(cl, scheme.Scheme, fakeEventRecorder, kialiReconciler)
	return cl, enhancedTracker, r, kialiReconciler
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

func assertReconcileSucceedsWithRequeue(r *MemberRollReconciler, t *testing.T) {
	res, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if !res.Requeue {
		t.Error("Expected reconcile to requeue the request, but it didn't")
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
			ServiceMeshReconciledVersion: status.ComposeReconciledVersion(operatorVersion, observedMeshGeneration),
		},
	}
}

func newControlPlane(version string) *maistrav2.ServiceMeshControlPlane {
	enabled := true
	return &maistrav2.ServiceMeshControlPlane{
		ObjectMeta: meta.ObjectMeta{
			Name:       controlPlaneName,
			Namespace:  controlPlaneNamespace,
			UID:        controlPlaneUID,
			Generation: 1,
		},
		Spec: maistrav2.ControlPlaneSpec{
			Version: version,
			Addons: &maistrav2.AddonsConfig{
				Kiali: &maistrav2.KialiAddonConfig{
					Enablement: maistrav2.Enablement{
						Enabled: &enabled,
					},
					Name: "kiali",
				},
			},
		},
	}
}

func markControlPlaneReconciled(controlPlane *maistrav2.ServiceMeshControlPlane, operatorVersion string) *maistrav2.ServiceMeshControlPlane {
	controlPlane.Status.Conditions = []status.Condition{
		{
			Type:   status.ConditionTypeReconciled,
			Status: status.ConditionStatusTrue,
		},
	}
	controlPlane.Status.ObservedGeneration = controlPlane.GetGeneration()
	controlPlane.Status.OperatorVersion = operatorVersion
	controlPlane.Spec.DeepCopyInto(&controlPlane.Status.AppliedSpec)
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

type fakeKialiReconciler struct {
	reconcileKialiInvoked  bool
	kialiConfiguredMembers []string
	errorToReturn          error
	delegate               func(ctx context.Context, kialiCRName, kialiCRNamespace string, configuredMembers []string) error
}

func (f *fakeKialiReconciler) reconcileKiali(ctx context.Context, kialiCRName, kialiCRNamespace string, configuredMembers []string) error {
	f.reconcileKialiInvoked = true
	f.kialiConfiguredMembers = append([]string{}, configuredMembers...)
	if f.errorToReturn != nil {
		return f.errorToReturn
	}
	if f.delegate != nil {
		return f.delegate(ctx, kialiCRName, kialiCRNamespace, configuredMembers)
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
	t.Helper()
	assert.True(f.reconcileKialiInvoked, "Expected reconcileKiali to be invoked, but it wasn't", t)
	if len(namespaces) != 0 || len(f.kialiConfiguredMembers) != 0 {
		assert.DeepEquals(f.kialiConfiguredMembers, namespaces, "reconcileKiali called with unexpected member list", t)
	}
}

func (f *fakeKialiReconciler) assertNotInvoked(t *testing.T) {
	assert.False(f.reconcileKialiInvoked, "Expected reconcileKiali not to be invoked, but it was", t)
}

func assertConditions(roll *maistrav1.ServiceMeshMemberRoll, expected []maistrav1.ServiceMeshMemberRollCondition, t *testing.T) {
	assert.DeepEquals(removeTimestamps(roll.Status.Conditions), expected, "Unexpected Status.Conditions in SMMR", t)
}

func removeTimestamps(conditions []maistrav1.ServiceMeshMemberRollCondition) []maistrav1.ServiceMeshMemberRollCondition {
	copies := make([]maistrav1.ServiceMeshMemberRollCondition, len(conditions))
	for i, cond := range conditions {
		condCopy := cond.DeepCopy()
		condCopy.LastTransitionTime = meta.Time{}
		copies[i] = *condCopy
	}
	return copies
}

func assertStatusMembers(roll *maistrav1.ServiceMeshMemberRoll, members, pending, configured, terminating []string, t *testing.T) {
	t.Helper()
	assert.DeepEquals(roll.Status.Members, members, "Unexpected Status.Members in SMMR", t)
	assert.DeepEquals(roll.Status.PendingMembers, pending, "Unexpected Status.PendingMembers in SMMR", t)
	assert.DeepEquals(roll.Status.ConfiguredMembers, configured, "Unexpected Status.ConfiguredMembers in SMMR", t)
	assert.DeepEquals(roll.Status.TerminatingMembers, terminating, "Unexpected Status.TerminatingMembers in SMMR", t)
	for _, member := range members {
		found := false
		for _, memberStatus := range roll.Status.MemberStatuses {
			if memberStatus.Namespace == member {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("No entry for namespace %s found in ServiceMeshMemberRoll.Status.MemberStatuses: %v", member, roll.Status.MemberStatuses)
		}
	}
}

func assertMemberCreated(cl client.Client, t *testing.T, namespace string, controlPlaneName string, controlPlaneNamespace string) {
	t.Helper()
	member := maistrav1.ServiceMeshMember{}
	err := cl.Get(ctx, client.ObjectKey{Name: common.MemberName, Namespace: namespace}, &member)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	assert.DeepEquals(member.Spec.ControlPlaneRef, maistrav1.ServiceMeshControlPlaneRef{Name: controlPlaneName, Namespace: controlPlaneNamespace}, "Unexpected controlPlaneRef in ServiceMeshMember", t)
	assert.Equals(member.ObjectMeta.Annotations[common.CreatedByKey], controllerName, "Unexpected created-by annotation  in ServiceMeshMember", t)
}
