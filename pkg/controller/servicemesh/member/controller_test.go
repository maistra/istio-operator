package member

import (
	"context"
	"fmt"
	"testing"
	"time"

	multusv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var ctx = common.NewContextWithLog(context.Background(), logf.Log)

const (
	memberName = "default"
	memberUID  = types.UID("3333")

	memberRollName = "default"

	appNamespace          = "app-namespace"
	controlPlaneName      = "my-mesh"
	controlPlaneNamespace = "cp-namespace"
	controlPlaneUID       = types.UID("2222")

	operatorVersion1_1     = "1.1.0"
	operatorVersion2_0     = "2.0.0"
	operatorVersionDefault = operatorVersion2_0

	cniNetwork1_1 = "v1-1-istio-cni"
)

var (
	request = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      memberName,
			Namespace: appNamespace,
		},
	}

	oneMinuteAgo = meta.NewTime(time.Now().Add(-time.Minute))
)

func init() {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))
}

func TestReconcileAddsFinalizer(t *testing.T) {
	member := newMember()
	member.Finalizers = []string{}

	cl, tracker, r := createClientAndReconciler(member)

	assertReconcileSucceeds(r, t)

	updatedMember := test.GetUpdatedObject(ctx, cl, member.ObjectMeta, &maistrav1.ServiceMeshMember{}).(*maistrav1.ServiceMeshMember)

	assert.DeepEquals(updatedMember.GetFinalizers(), []string{common.FinalizerName}, "Invalid finalizers in SMM", t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)
}

func TestReconcileMemberWithInvalidName(t *testing.T) {
	member := newMember()
	member.Name = "not-default"
	cl, _, r := createClientAndReconciler(member)

	assertReconcileWithRequestSucceeds(r, reconcile.Request{NamespacedName: common.ToNamespacedName(member)}, t)

	updatedMember := test.GetUpdatedObject(ctx, cl, member.ObjectMeta, &maistrav1.ServiceMeshMember{}).(*maistrav1.ServiceMeshMember)
	expectedMessage := fmt.Sprintf("the ServiceMeshMember name is invalid; must be %q", common.MemberName)

	readyCondition := updatedMember.Status.GetCondition(maistrav1.ConditionTypeMemberReady)
	assert.Equals(readyCondition.Status, corev1.ConditionFalse, "unexpected condition status", t)
	assert.Equals(readyCondition.Reason, maistrav1.ConditionReasonMemberNameInvalid, "unexpected condition reason", t)
	assert.Equals(readyCondition.Message, expectedMessage, "unexpected condition message", t)

	reconciledCondition := updatedMember.Status.GetCondition(maistrav1.ConditionTypeMemberReconciled)
	assert.Equals(reconciledCondition.Status, corev1.ConditionFalse, "unexpected condition status", t)
	assert.Equals(reconciledCondition.Reason, maistrav1.ConditionReasonMemberNameInvalid, "unexpected condition reason", t)
	assert.Equals(reconciledCondition.Message, expectedMessage, "unexpected condition message", t)
}

func TestReconcileCreatesMemberRollIfNeeded(t *testing.T) {
	member := newMember()
	controlPlane := newControlPlane(versions.DefaultVersion.String())
	markControlPlaneReconciled(controlPlane, operatorVersionDefault)
	cl, _, r := createClientAndReconciler(member, controlPlane)

	assertReconcileSucceeds(r, t)

	memberRollKey := types.NamespacedName{Namespace: controlPlaneNamespace, Name: common.MemberRollName}
	test.AssertObjectExists(ctx, cl, memberRollKey, &maistrav1.ServiceMeshMemberRoll{}, "Expected reconcile to create the SMMR, but it didn't", t)

	memberRoll := test.GetObject(ctx, cl, memberRollKey, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)

	createdByAnnotation, annotationFound := memberRoll.Annotations[common.CreatedByKey]
	if !annotationFound {
		t.Fatalf("Expected reconcile to create the SMMR with the annotation %s, but the annotation was missing.", common.CreatedByKey)
	}
	assert.DeepEquals(createdByAnnotation, controllerName, "Wrong annotation value", t)
}

func TestReconcileDoesNothingIfReferencedControlPlaneNamespaceDoesNotExist(t *testing.T) {
	member := newMember()
	member.Spec.ControlPlaneRef.Namespace = "nonexistent-ns"

	_, tracker, r := createClientAndReconciler(member)

	tracker.AddReactor("get", "namespace", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetNamespace() == "nonexistent-ns" {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{
				Group:    "",
				Resource: "Namespace",
			}, action.GetNamespace())
		}
		return false, nil, nil
	})

	assertReconcileSucceeds(r, t)
}

func TestReconcileCreatesMemberRollWhenReferencedControlPlaneIsCreated(t *testing.T) {
	member := newMember()

	cl, _, r := createClientAndReconciler(member)

	// reconcile while SMCP does not exist
	assertReconcileSucceeds(r, t)

	// create the SMCP
	controlPlane := newControlPlane(versions.DefaultVersion.String())
	markControlPlaneReconciled(controlPlane, operatorVersionDefault)
	test.PanicOnError(cl.Create(ctx, controlPlane))

	// check if the SMMR is created now that the SMCP exists
	assertReconcileSucceeds(r, t)

	memberRollKey := types.NamespacedName{Namespace: controlPlaneNamespace, Name: common.MemberRollName}
	test.AssertObjectExists(ctx, cl, memberRollKey, &maistrav1.ServiceMeshMemberRoll{}, "Expected reconcile to create the SMMR, but it didn't", t)
}

func TestReconcileRemovesFinalizerFromMemberWhenMemberDeleted(t *testing.T) {
	member := newMember()
	member.DeletionTimestamp = &oneMinuteAgo

	cl, _, r := createClientAndReconciler(member)

	assertReconcileSucceeds(r, t)

	updatedMember := test.GetUpdatedObject(ctx, cl, member.ObjectMeta, &maistrav1.ServiceMeshMember{}).(*maistrav1.ServiceMeshMember)
	assert.StringArrayEmpty(updatedMember.Finalizers, "Expected finalizers list in SMM to be empty", t)
}

func TestReconcileDeletesMemberRollCreatedByItWhenMemberDeleted(t *testing.T) {
	member := newMember()
	member.DeletionTimestamp = &oneMinuteAgo
	memberRoll := newMemberRoll()
	memberRoll.Annotations = map[string]string{
		common.CreatedByKey: controllerName,
	}

	cl, _, r := createClientAndReconciler(member, memberRoll)

	assertReconcileSucceeds(r, t)
	test.AssertNotFound(ctx, cl, types.NamespacedName{Namespace: controlPlaneNamespace, Name: common.MemberRollName}, &maistrav1.ServiceMeshMemberRoll{},
		"Expected reconcile to delete the SMMR, but it didn't", t)
}

func TestReconcilePreservesMemberRollCreatedByItButModifiedByUser(t *testing.T) {
	member := newMember()
	member.DeletionTimestamp = &oneMinuteAgo
	memberRoll := newMemberRoll()
	memberRoll.Annotations = map[string]string{
		common.CreatedByKey: controllerName,
	}
	memberRoll.Spec.Members = []string{"other-ns-1"}

	cl, _, r := createClientAndReconciler(member, memberRoll)

	assertReconcileSucceeds(r, t)

	updatedMemberRoll := test.GetUpdatedObject(ctx, cl, memberRoll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.DeepEquals(updatedMemberRoll.Spec.Members, []string{"other-ns-1"}, "Unexpected members in SMMR", t)
}

func TestReconcileSucceedsIfControlPlaneAndMembersRollDoNotExistWhenDeletingMember(t *testing.T) {
	member := newMember()
	member.DeletionTimestamp = &oneMinuteAgo

	cl, _, r := createClientAndReconciler(member)

	assertReconcileSucceeds(r, t)

	updatedMember := test.GetUpdatedObject(ctx, cl, member.ObjectMeta, &maistrav1.ServiceMeshMember{}).(*maistrav1.ServiceMeshMember)
	assert.StringArrayEmpty(updatedMember.Finalizers, "Expected finalizers list in SMM to be empty", t)
}

func TestReconcileSucceedsIfMembersRollIsDeletedExternallyWhenRemovingMember(t *testing.T) {
	member := newMember()
	member.DeletionTimestamp = &oneMinuteAgo
	memberRoll := newMemberRoll()
	memberRoll.Annotations = map[string]string{
		common.CreatedByKey: controllerName,
	}
	memberRoll.Spec.Members = []string{appNamespace}

	_, tracker, r := createClientAndReconciler(member, memberRoll)
	tracker.AddReactor("delete", "servicemeshmemberrolls", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{
			Group:    maistrav1.APIGroup,
			Resource: "ServiceMeshMemberRoll",
		}, memberRollName)
	})

	assertReconcileSucceeds(r, t)
}

func TestReconcileDoesNothingIfMemberIsDeletedAndHasNoFinalizers(t *testing.T) {
	member := newMember()
	member.DeletionTimestamp = &oneMinuteAgo
	member.Finalizers = nil

	_, tracker, r := createClientAndReconciler(member)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingWhenMemberIsNotFound(t *testing.T) {
	_, tracker, r := createClientAndReconciler()
	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileReturnsErrorIfItCanNotReadMember(t *testing.T) {
	_, tracker, r := createClientAndReconciler()
	tracker.AddReactor("get", "servicemeshmembers", test.ClientFails())
	assertReconcileFails(r, t)
}

func TestReconcileReturnsErrorIfClientOperationFails(t *testing.T) {
	cases := []struct {
		name                      string
		controlPlaneExists        bool
		memberRollExists          bool
		memberRollCreatedManually bool
		memberInit                func(*maistrav1.ServiceMeshMember)
		reactor                   clienttesting.Reactor
	}{
		{
			name:    "get-member-fails",
			reactor: test.On("get", "servicemeshmembers", test.ClientFails()),
		},
		{
			name:    "update-member-fails",
			reactor: test.On("patch", "servicemeshmembers", test.ClientFails()),
		},
		{
			name:       "add-finalizer-fails",
			memberInit: func(member *maistrav1.ServiceMeshMember) { member.Finalizers = []string{} },
			reactor:    test.On("update", "servicemeshmembers", test.ClientFails()),
		},
		{
			name:    "patch-member-status-fails",
			reactor: test.On("patch", "servicemeshmembers", test.ClientFails()),
		},
		{
			name:    "get-control-plane-fails",
			reactor: test.On("get", "servicemeshcontrolplanes", test.ClientFails()),
		},
		{
			name:               "get-member-roll-fails",
			reactor:            test.On("get", "servicemeshmemberrolls", test.ClientFails()),
			controlPlaneExists: true,
		},
		{
			name:               "create-member-roll-fails",
			reactor:            test.On("create", "servicemeshmemberrolls", test.ClientFails()),
			controlPlaneExists: true,
		},
		{
			name:       "get-member-roll-fails-during-delete",
			memberInit: func(member *maistrav1.ServiceMeshMember) { member.DeletionTimestamp = &oneMinuteAgo },
			reactor:    test.On("get", "servicemeshmemberrolls", test.ClientFails()),
		},
		{
			name:             "delete-member-roll-fails-during-delete",
			memberRollExists: true,
			memberInit:       func(member *maistrav1.ServiceMeshMember) { member.DeletionTimestamp = &oneMinuteAgo },
			reactor:          test.On("delete", "servicemeshmemberrolls", test.ClientFails()),
		},
		{
			name:             "update-member-fails-during-delete",
			memberRollExists: true,
			memberInit:       func(member *maistrav1.ServiceMeshMember) { member.DeletionTimestamp = &oneMinuteAgo },
			reactor:          test.On("update", "servicemeshmembers", test.ClientFails()),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var objects []runtime.Object

			member := newMember()
			if tc.memberInit != nil {
				tc.memberInit(member)
			}
			objects = append(objects, member)

			if tc.controlPlaneExists {
				controlPlane := newControlPlane(versions.DefaultVersion.String())
				objects = append(objects, controlPlane)
			}
			if tc.memberRollExists {
				memberRoll := newMemberRoll()
				if !tc.memberRollCreatedManually {
					memberRoll.Annotations = map[string]string{
						common.CreatedByKey: controllerName,
					}
				}
				objects = append(objects, memberRoll)
			}

			_, tracker, r := createClientAndReconciler(objects...)
			tracker.AddReaction(tc.reactor)

			assertReconcileFails(r, t)
		})
	}
}

func TestReconcileAfterOperatorUpgrade(t *testing.T) {
	cases := []struct {
		name                string
		operatorVersion     string
		meshVersion         string
		expectedNetworkName string
		upgradedOperator    bool
	}{
		{
			// tests a namespace add being processed before the mesh is upgraded
			name:                "v1.1-before-mesh-upgrade",
			operatorVersion:     operatorVersion1_1,
			meshVersion:         "v1.1",
			expectedNetworkName: cniNetwork1_1,
		},
		{
			// tests a namespace add being processed after the mesh is upgraded,
			// but before roll has been synced, i.e. simulates a mesh upgrade
			// _and_ a roll update hitting at the same time
			name:                "v1.1-after-mesh-upgrade",
			operatorVersion:     operatorVersion2_0,
			meshVersion:         "v1.1",
			expectedNetworkName: cniNetwork1_1,
			upgradedOperator:    true,
		},
		{
			name:                "v1.1-installed-with-v2.0",
			operatorVersion:     operatorVersion2_0,
			meshVersion:         versions.V1_1.String(),
			expectedNetworkName: cniNetwork1_1,
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
			meshVersion:         "v1.1",
			expectedNetworkName: cniNetwork1_1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			member := newMember()
			markMemberReconciled(member, 2, 1, 1, tc.operatorVersion)

			controlPlane := markControlPlaneReconciled(newControlPlane(tc.meshVersion), tc.operatorVersion)
			if tc.upgradedOperator {
				// need to reset the ServiceMeshReconciledVersion
				member.Status.ServiceMeshReconciledVersion = status.ComposeReconciledVersion(operatorVersion1_1, controlPlane.GetGeneration())
			}
			member.Status.ServiceMeshGeneration = controlPlane.Status.ObservedGeneration
			namespace := newAppNamespace()

			cl, _, r := createClientAndReconciler(member, controlPlane, namespace)

			assertReconcileSucceeds(r, t)

			updatedMember := test.GetUpdatedObject(ctx, cl, member.ObjectMeta, &maistrav1.ServiceMeshMember{}).(*maistrav1.ServiceMeshMember)
			assert.Equals(updatedMember.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)

			assertNamespaceReconciled(t, cl, appNamespace, controlPlaneNamespace, tc.expectedNetworkName)
		})
	}
}

func TestReconciliationOfTerminatingNamespace(t *testing.T) {
	cases := []struct {
		name               string
		configureMember    func(member *maistrav1.ServiceMeshMember)
		configureNamespace func(ns *corev1.Namespace)
	}{
		{
			name: "creation",
		},
		{
			name: "deletion",
			configureMember: func(member *maistrav1.ServiceMeshMember) {
				member.DeletionTimestamp = &oneMinuteAgo
			},
			configureNamespace: func(ns *corev1.Namespace) {
				common.SetLabel(ns, common.MemberOfKey, controlPlaneNamespace)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			member := newMember()
			if tc.configureMember != nil {
				tc.configureMember(member)
			}

			controlPlane := markControlPlaneReconciled(newControlPlane(versions.DefaultVersion.String()), operatorVersionDefault)

			namespace := newAppNamespace()
			namespace.DeletionTimestamp = &oneMinuteAgo
			if tc.configureNamespace != nil {
				tc.configureNamespace(namespace)
			}

			cl, _, r := createClientAndReconciler(member, controlPlane, namespace)
			assertReconcileSucceeds(r, t)

			updatedMember := test.GetUpdatedObject(ctx, cl, member.ObjectMeta, &maistrav1.ServiceMeshMember{}).(*maistrav1.ServiceMeshMember)
			readyStatus := updatedMember.Status.GetCondition(maistrav1.ConditionTypeMemberReady).Status
			assert.Equals(readyStatus, corev1.ConditionFalse, "Unexpected Ready condition status", t)
			reconciledStatus := updatedMember.Status.GetCondition(maistrav1.ConditionTypeMemberReconciled).Status
			assert.Equals(reconciledStatus, corev1.ConditionFalse, "Unexpected Reconciled condition status", t)
		})
	}
}

func TestReconcileReturnsConflictError(t *testing.T) {
	cases := []struct{ verb, resource string }{
		{"patch", "servicemeshmembers"},
		{"create", "servicemeshmemberrolls"},
		{"create", "rolebindings"},
		{"create", "networkattachmentdefinitions"},
		{"update", "namespaces"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s %s", tc.verb, tc.resource), func(t *testing.T) {
			member := newMember()
			appNs := newAppNamespace()
			roleBinding := newMeshRoleBinding()
			smcp := newControlPlane(versions.DefaultVersion.String())

			_, tracker, r := createClientAndReconciler(smcp, member, appNs, roleBinding)
			tracker.AddReactor(tc.verb, tc.resource, func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, apierrors.NewConflict(schema.GroupResource{
					Group:    "v1",
					Resource: "DoesntMatter",
				}, "doesnt-matter", fmt.Errorf("simulated conflict"))
			})

			_, err := r.Reconcile(context.TODO(), request)
			if !common.IsConflict(err) {
				t.Fatalf("Expected Conflict error, but got: %v", err)
			}
		})
	}
}

func newMember() *maistrav1.ServiceMeshMember {
	return &maistrav1.ServiceMeshMember{
		ObjectMeta: meta.ObjectMeta{
			Name:       memberName,
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

func newMemberRoll() *maistrav1.ServiceMeshMemberRoll {
	return &maistrav1.ServiceMeshMemberRoll{
		ObjectMeta: meta.ObjectMeta{
			Name:      memberRollName,
			Namespace: controlPlaneNamespace,
		},
		Spec: maistrav1.ServiceMeshMemberRollSpec{
			Members: []string{},
		},
	}
}

func markMemberReconciled(member *maistrav1.ServiceMeshMember, generation, observedGeneration, observedMeshGeneration int64,
	operatorVersion string,
) *maistrav1.ServiceMeshMember {
	member.Finalizers = []string{common.FinalizerName}
	member.Generation = generation
	member.UID = memberUID
	member.Status.ObservedGeneration = observedGeneration
	member.Status.ServiceMeshGeneration = observedMeshGeneration
	member.Status.ServiceMeshReconciledVersion = status.ComposeReconciledVersion(operatorVersion, observedMeshGeneration)
	return member
}

func createClientAndReconciler(clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, *MemberReconciler) {
	cl, enhancedTracker := test.CreateClient(clientObjects...)
	fakeEventRecorder := &record.FakeRecorder{}

	rf := fakeNamespaceReconcilerFactory{
		reconciler: &fakeNamespaceReconciler{},
	}
	cniConfig := cni.Config{Enabled: true}

	r := newReconciler(cl, scheme.Scheme, fakeEventRecorder, rf.newReconciler, cniConfig)
	return cl, enhancedTracker, r
}

func assertReconcileSucceeds(r *MemberReconciler, t *testing.T) {
	assertReconcileWithRequestSucceeds(r, request, t)
}

func assertReconcileWithRequestSucceeds(r *MemberReconciler, request reconcile.Request, t *testing.T) {
	res, err := r.Reconcile(context.TODO(), request)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if res.Requeue {
		t.Error("Reconcile requeued the request, but it shouldn't have")
	}
}

func assertReconcileFails(r *MemberReconciler, t *testing.T) {
	t.Helper()
	_, err := r.Reconcile(context.TODO(), request)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}
}

type fakeNamespaceReconcilerFactory struct {
	reconciler *fakeNamespaceReconciler
}

func (rf *fakeNamespaceReconcilerFactory) newReconciler(ctx context.Context, cl client.Client,
	meshNamespace string, meshVersion versions.Version, isCNIEnabled bool,
) (NamespaceReconciler, error) {
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

func newAppNamespace() *corev1.Namespace {
	namespace := &corev1.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name:   appNamespace,
			Labels: map[string]string{},
		},
	}
	return namespace
}

func newMeshRoleBinding() *rbac.RoleBinding {
	roleBinding := newRoleBinding(controlPlaneNamespace, "role-binding")
	roleBinding.Labels = map[string]string{}
	roleBinding.Labels[common.OwnerKey] = controlPlaneNamespace
	roleBinding.Labels[common.OwnerNameKey] = controlPlaneName
	roleBinding.Labels[common.KubernetesAppVersionKey] = "0"
	return roleBinding
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
	controlPlane.Status.ObservedGeneration = controlPlane.GetGeneration()
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

func assertNamespaceReconciled(t *testing.T, cl client.Client, namespace, meshNamespace, meshNetAttachDefName string) {
	// check if namespace has member-of label
	ns := &corev1.Namespace{}
	test.GetObject(ctx, cl, types.NamespacedName{Name: namespace}, ns)
	assert.Equals(ns.Labels[common.MemberOfKey], meshNamespace, "Unexpected or missing member-of label in namespace", t)

	// check if net-attach-def exists
	netAttachDef := &multusv1.NetworkAttachmentDefinition{}
	err := cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: meshNetAttachDefName}, netAttachDef)
	if err != nil {
		t.Fatalf("Couldn't get NetworkAttachmentDefinition from client: %v", err)
	}
}
