package member

import (
	"context"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

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
	memberName            = "default"
	memberUID         = types.UID("3333")

	memberRollName        = "default"
	memberRollUID         = types.UID("1111")

	appNamespace          = "app-namespace"
	controlPlaneName      = "my-mesh"
	controlPlaneNamespace = "cp-namespace"
	controlPlaneUID       = types.UID("2222")

	operatorVersion1_0     = "1.0.0"
	operatorVersion1_1     = "1.1.0"
	operatorVersion2_0     = "2.0.0"
	operatorVersionDefault = operatorVersion2_0

	cniNetwork1_0     = "istio-cni"
	cniNetwork1_1     = "v1-1-istio-cni"
	cniNetwork2_0     = "v2-0-istio-cni"
	cniNetworkDefault = cniNetwork2_0
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
	logf.SetLogger(logf.ZapLogger(true))
}

func TestReconcileAddsFinalizer(t *testing.T) {
	member := newMember()
	member.Finalizers = []string{}

	cl, tracker, r := createClientAndReconciler(t, member)

	assertReconcileSucceeds(r, t)

	updatedMember := test.GetUpdatedObject(ctx, cl, member.ObjectMeta, &maistrav1.ServiceMeshMember{}).(*maistrav1.ServiceMeshMember)

	assert.DeepEquals(updatedMember.GetFinalizers(), []string{common.FinalizerName}, "Invalid finalizers in SMM", t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)
}

func TestReconcileCreatesMemberRollIfNeeded(t *testing.T) {
	member := newMember()
	controlPlane := newControlPlane(versions.DefaultVersion.String())
	markControlPlaneReconciled(controlPlane, operatorVersionDefault)
	cl, _, r := createClientAndReconciler(t, member, controlPlane)

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

	_, tracker, r := createClientAndReconciler(t, member)

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

	cl, _, r := createClientAndReconciler(t, member)

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

	cl, _, r := createClientAndReconciler(t, member)

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

	cl, _, r := createClientAndReconciler(t, member, memberRoll)

	assertReconcileSucceeds(r, t)
	test.AssertNotFound(ctx, cl, types.NamespacedName{controlPlaneNamespace, common.MemberRollName}, &maistrav1.ServiceMeshMemberRoll{}, "Expected reconcile to delete the SMMR, but it didn't", t)
}

func TestReconcilePreservesMemberRollCreatedByItButModifiedByUser(t *testing.T) {
	member := newMember()
	member.DeletionTimestamp = &oneMinuteAgo
	memberRoll := newMemberRoll()
	memberRoll.Annotations = map[string]string{
		common.CreatedByKey: controllerName,
	}
	memberRoll.Spec.Members = []string{"other-ns-1"}

	cl, _, r := createClientAndReconciler(t, member, memberRoll)

	assertReconcileSucceeds(r, t)

	updatedMemberRoll := test.GetUpdatedObject(ctx, cl, memberRoll.ObjectMeta, &maistrav1.ServiceMeshMemberRoll{}).(*maistrav1.ServiceMeshMemberRoll)
	assert.DeepEquals(updatedMemberRoll.Spec.Members, []string{"other-ns-1"}, "Unexpected members in SMMR", t)
}

func TestReconcileSucceedsIfControlPlaneAndMembersRollDoNotExistWhenDeletingMember(t *testing.T) {
	member := newMember()
	member.DeletionTimestamp = &oneMinuteAgo

	cl, _, r := createClientAndReconciler(t, member)

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

	_, tracker, r := createClientAndReconciler(t, member, memberRoll)
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

	_, tracker, r := createClientAndReconciler(t, member)

	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingWhenMemberIsNotFound(t *testing.T) {
	_, tracker, r := createClientAndReconciler(t)
	assertReconcileSucceeds(r, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileReturnsErrorIfItCanNotReadMember(t *testing.T) {
	_, tracker, r := createClientAndReconciler(t)
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

			_, tracker, r := createClientAndReconciler(t, objects...)
			tracker.AddReaction(tc.reactor)

			assertReconcileFails(r, t)
		})
	}
}

func TestReconcileReconcilesAfterOperatorUpgradeFromV1_0(t *testing.T) {
	member := newMember()
	markMemberReconciled(member, 1, 1, 1, operatorVersion1_0)
	controlPlane := markControlPlaneReconciled(newControlPlane(versions.V1_0.String()), operatorVersionDefault)
	namespace := newNamespace(appNamespace)
	common.SetLabel(namespace, common.MemberOfKey, controlPlaneNamespace)
	meshRoleBinding := newMeshRoleBinding()
	appRoleBinding := newMeshRoleBinding()
	appRoleBinding.SetNamespace(appNamespace)
	common.SetLabel(appRoleBinding, common.MemberOfKey, controlPlaneNamespace)
	nad := createNAD(cniNetwork1_0, appNamespace, controlPlaneNamespace)

	cl, tracker, r := createClientAndReconciler(t, member, controlPlane, namespace, meshRoleBinding, appRoleBinding, nad)
	tracker.AddReactor("delete", multusv1.SchemeGroupVersion.WithResource("networkattachmentdefinitions").String(), assertNADNotDeleted(t))
	tracker.AddReactor("create", rbac.SchemeGroupVersion.WithResource("rolebindings").String(), assertRBNotCreated(t))

	assert.Equals(member.Status.ServiceMeshReconciledVersion != controlPlane.Status.GetReconciledVersion(), true, "Unexpected Status.ServiceMeshReconciledVersion in SMMR already matches SMCP reconciled version", t)

	assertReconcileSucceeds(r, t)

	updatedMember := test.GetUpdatedObject(ctx, cl, member.ObjectMeta, &maistrav1.ServiceMeshMember{}).(*maistrav1.ServiceMeshMember)
	assert.Equals(updatedMember.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)
	assert.Equals(updatedMember.Status.ServiceMeshReconciledVersion, controlPlane.Status.GetReconciledVersion(), "Unexpected Status.ServiceMeshReconciledVersion in SMMR", t)

	meshNetAttachDefName := cniNetwork1_0
	assertNamespaceReconciled(t, cl, appNamespace, controlPlaneNamespace, meshNetAttachDefName, []rbac.RoleBinding{*meshRoleBinding})
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
			member := newMember()
			markMemberReconciled(member, 2, 1, 1, tc.operatorVersion)

			controlPlane := markControlPlaneReconciled(newControlPlane(tc.meshVersion), tc.operatorVersion)
			if tc.upgradedOperator {
				// need to reset the ServiceMeshReconciledVersion
				member.Status.ServiceMeshReconciledVersion = status.ComposeReconciledVersion(operatorVersion1_0, controlPlane.GetGeneration())
			}
			member.Status.ServiceMeshGeneration = controlPlane.Status.ObservedGeneration
			namespace := newNamespace(appNamespace)
			meshRoleBinding := newMeshRoleBinding()

			cl, _, r := createClientAndReconciler(t, member, controlPlane, namespace)

			assertReconcileSucceeds(r, t)

			updatedMember := test.GetUpdatedObject(ctx, cl, member.ObjectMeta, &maistrav1.ServiceMeshMember{}).(*maistrav1.ServiceMeshMember)
			assert.Equals(updatedMember.Status.ServiceMeshGeneration, controlPlane.Status.ObservedGeneration, "Unexpected Status.ServiceMeshGeneration in SMMR", t)

			assertNamespaceReconciled(t, cl, appNamespace, controlPlaneNamespace, tc.expectedNetworkName, []rbac.RoleBinding{*meshRoleBinding})
		})
	}
}

func TestReconciliationOfTerminatingNamespace(t *testing.T) {
	cases := []struct {
		name                string
		configureMember func(member *maistrav1.ServiceMeshMember)
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

			controlPlane := markControlPlaneReconciled(newControlPlane(""), operatorVersionDefault)

			namespace := newNamespace(appNamespace)
			namespace.DeletionTimestamp = &oneMinuteAgo
			if tc.configureNamespace != nil {
				tc.configureNamespace(namespace)
			}

			cl, _, r := createClientAndReconciler(t, member, controlPlane, namespace)
			assertReconcileSucceeds(r, t)

			updatedMember := test.GetUpdatedObject(ctx, cl, member.ObjectMeta, &maistrav1.ServiceMeshMember{}).(*maistrav1.ServiceMeshMember)
			assert.Equals(updatedMember.Status.GetCondition(maistrav1.ConditionTypeMemberReady).Status, corev1.ConditionFalse, "Unexpected Ready condition status", t)
			assert.Equals(updatedMember.Status.GetCondition(maistrav1.ConditionTypeMemberReconciled).Status, corev1.ConditionFalse, "Unexpected Reconciled condition status", t)
		})
	}
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
	netAttachDef := &multusv1.NetworkAttachmentDefinition{}
	netAttachDef.SetNamespace(appNamespace)
	netAttachDef.SetName(name)
	common.SetLabel(netAttachDef, common.MemberOfKey, cpNamespace)
	return netAttachDef
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

func markMemberReconciled(member *maistrav1.ServiceMeshMember, generation int64, observedGeneration int64, observedMeshGeneration int64, operatorVersion string) *maistrav1.ServiceMeshMember {
	member.Finalizers = []string{common.FinalizerName}
	member.Generation = generation
	member.UID = memberUID
	member.Status.ObservedGeneration = observedGeneration
	member.Status.ServiceMeshGeneration = observedMeshGeneration
	member.Status.ServiceMeshReconciledVersion = status.ComposeReconciledVersion(operatorVersion, observedMeshGeneration)
	return member
}

func createClientAndReconciler(t *testing.T, clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, *MemberReconciler) {
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
	res, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if res.Requeue {
		t.Error("Reconcile requeued the request, but it shouldn't have")
	}
}

func assertReconcileFails(r *MemberReconciler, t *testing.T) {
	t.Helper()
	_, err := r.Reconcile(request)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}
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

func newNamespace(name string) *corev1.Namespace {
	namespace := &corev1.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name:   name,
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

func newAppNamespaceRoleBinding() *rbac.RoleBinding {
	roleBinding := newRoleBinding(appNamespace, "role-binding")
	roleBinding.Labels = map[string]string{}
	roleBinding.Labels[common.OwnerKey] = controlPlaneNamespace
	roleBinding.Labels[common.MemberOfKey] = controlPlaneNamespace
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

func assertNamespaceReconcilerInvoked(t *testing.T, nsReconciler *fakeNamespaceReconciler, namespaces ...string) {
	assert.DeepEquals(nsReconciler.reconciledNamespaces, namespaces, "Expected namespace reconciler to be invoked, but it wasn't or it wasn't invoked properly", t)
}

func assertNamespaceRemoveInvoked(t *testing.T, nsReconciler *fakeNamespaceReconciler, namespaces ...string) {
	assert.DeepEquals(nsReconciler.removedNamespaces, namespaces, "Expected removal to be invoked for namespace, but it wasn't or it wasn't invoked properly", t)
}

func assertNamespaceReconciled(t *testing.T, cl client.Client, namespace, meshNamespace string, meshNetAttachDefName string, meshRoleBindings []rbac.RoleBinding) {
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
