package memberroll

import (
	"context"
	"testing"

	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileNamespaceInMesh(t *testing.T) {
	namespace := newNamespace(appNamespace)
	meshRoleBinding := newMeshRoleBinding()
	meshRoleBindings := []*rbac.RoleBinding{meshRoleBinding}
	cl, _ := test.CreateClient(namespace, meshRoleBinding)

	fakeNetworkStrategy := &fakeNetworkStrategy{}
	assertReconcileNamespaceSucceeds(t, cl, fakeNetworkStrategy)

	// check if namespace has member-of label
	ns := &core.Namespace{}
	test.GetObject(cl, types.NamespacedName{Name: appNamespace}, ns)
	assert.Equals(ns.Labels[common.MemberOfKey], controlPlaneNamespace, "Unexpected or missing member-of label in namespace", t)

	// check if net-attach-def exists
	netAttachDefName, _ := common.GetCNINetworkName(meshVersionDefault)
	netAttachDef := newNetworkAttachmentDefinition()
	err := cl.Get(context.TODO(), types.NamespacedName{Namespace: appNamespace, Name: netAttachDefName}, netAttachDef)
	if err != nil {
		t.Fatalf("Couldn't get NetworkAttachmentDefinition from client: %v", err)
	}

	// check role bindings exist
	roleBindings := rbac.RoleBindingList{}
	err = cl.List(context.TODO(), client.InNamespace(appNamespace), &roleBindings)
	if err != nil {
		t.Fatalf("Couldn't list RoleBindings: %v", err)
	}

	expectedRoleBindings := []rbac.RoleBinding{}
	for _, meshRB := range meshRoleBindings {
		expectedRB := meshRB.DeepCopy()
		expectedRB.Namespace = appNamespace
		expectedRB.Labels[common.MemberOfKey] = controlPlaneNamespace
		expectedRoleBindings = append(expectedRoleBindings, *expectedRB)
	}

	assert.DeepEquals(roleBindings.Items, expectedRoleBindings, "Unexpected RoleBindings found in namespace", t)
	assert.DeepEquals(fakeNetworkStrategy.reconciledNamespaces, []string{appNamespace}, "Expected reconcileNamespaceInMesh to invoke the networkStrategy with only the appNamespace, but it didn't", t)
}

func TestReconcileFailsIfNamespaceIsPartOfAnotherMesh(t *testing.T) {
	namespace := newNamespace(appNamespace)
	namespace.Labels = map[string]string{
		common.MemberOfKey: "other-control-plane",
	}
	cl, _ := test.CreateClient(namespace)

	assertReconcileNamespaceFails(t, cl)
}

func TestRemoveNamespaceFromMesh(t *testing.T) {
	namespace := newNamespace(appNamespace)
	meshRoleBinding := newMeshRoleBinding()
	cl, _ := test.CreateClient(namespace, meshRoleBinding)
	setupReconciledNamespace(t, cl, appNamespace)

	fakeNetworkStrategy := &fakeNetworkStrategy{}
	assertRemoveNamespaceSucceeds(t, cl, fakeNetworkStrategy)

	// check that namespace no longer has member-of label
	ns := &core.Namespace{}
	test.GetObject(cl, types.NamespacedName{Name: appNamespace}, ns)
	_, found := ns.Labels[common.MemberOfKey]
	assert.False(found, "Expected member-of label to be removed, but it is still present", t)

	// check that net-attach-def was removed
	netAttachDefName, _ := common.GetCNINetworkName(meshVersionDefault)
	netAttachDef := newNetworkAttachmentDefinition()
	err := cl.Get(context.TODO(), types.NamespacedName{Namespace: appNamespace, Name: netAttachDefName}, netAttachDef)
	assertNotFound(err, "Expected NetworkAttachmentDefinition to be deleted, but it is still present", t)

	// check that role binding was removed
	roleBinding := &rbac.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{Namespace: appNamespace, Name: meshRoleBinding.Name}, roleBinding)
	assertNotFound(err, "Expected RoleBinding to be deleted, but it is still present", t)

	assert.DeepEquals(fakeNetworkStrategy.removedNamespaces, []string{appNamespace}, "Expected removeNamespaceFromMesh to invoke the networkStrategy with only the appNamespace, but it didn't", t)
}

func TestReconcileUpdatesModifiedRoleBindings(t *testing.T) {
	t.Skip("Not implemented yet")
	namespace := newNamespace(appNamespace)
	meshRoleBinding := newMeshRoleBinding()
	cl, _ := test.CreateClient(namespace, meshRoleBinding)
	setupReconciledNamespace(t, cl, appNamespace)

	// update mesh role binding
	meshRoleBinding.Subjects = []rbac.Subject{
		{
			Kind: rbac.UserKind,
			Name: "alice",
		},
	}
	err := cl.Update(context.TODO(), meshRoleBinding)
	if err != nil {
		t.Fatalf("Couldn't update meshRoleBinding: %v", err)
	}

	fakeNetworkStrategy := &fakeNetworkStrategy{}
	assertReconcileNamespaceSucceeds(t, cl, fakeNetworkStrategy)

	// check role bindings exist
	roleBindings := rbac.RoleBindingList{}
	err = cl.List(context.TODO(), client.InNamespace(appNamespace), &roleBindings)
	if err != nil {
		t.Fatalf("Couldn't list RoleBindings: %v", err)
	}

	expectedRoleBindings := []rbac.RoleBinding{}

	expectedRB := meshRoleBinding.DeepCopy()
	expectedRB.Namespace = appNamespace
	expectedRB.Labels[common.MemberOfKey] = controlPlaneNamespace
	expectedRoleBindings = append(expectedRoleBindings, *expectedRB)

	assert.DeepEquals(roleBindings.Items, expectedRoleBindings, "Unexpected RoleBindings found in namespace", t)
	assert.DeepEquals(fakeNetworkStrategy.reconciledNamespaces, []string{appNamespace}, "Expected reconcileNamespace to invoke the networkStrategy with only the appNamespace, but it didn't", t)
}

func TestReconcileDeletesObsoleteRoleBindings(t *testing.T) {
	namespace := newNamespace(appNamespace)
	meshRoleBinding := newMeshRoleBinding()

	cl, _ := test.CreateClient(namespace, meshRoleBinding)
	setupReconciledNamespace(t, cl, appNamespace)

	err := cl.Delete(context.TODO(), meshRoleBinding)
	if err != nil {
		t.Fatalf("Couldn't update meshRoleBinding: %v", err)
	}

	fakeNetworkStrategy := &fakeNetworkStrategy{}
	assertReconcileNamespaceSucceeds(t, cl, fakeNetworkStrategy)

	// check that role binding in app namespace has also been deleted
	roleBindings := rbac.RoleBindingList{}
	err = cl.List(context.TODO(), client.InNamespace(appNamespace), &roleBindings)
	if err != nil {
		t.Fatalf("Couldn't list RoleBindings: %v", err)
	}

	assert.DeepEquals(roleBindings.Items, []rbac.RoleBinding{}, "Unexpected RoleBindings found in namespace", t)
}

func TestOtherResourcesArePreserved(t *testing.T) {
	otherLabelName := "other-label"
	otherLabelValue := "other-label-value"
	namespace := newNamespace(appNamespace)
	namespace.Labels[otherLabelName] = otherLabelValue
	meshRoleBinding := newMeshRoleBinding()

	otherNetAttachDefNamme := "some-other-net-attach-def"
	otherNetAttachDef := newNetworkAttachmentDefinition()
	otherNetAttachDef.SetNamespace(appNamespace)
	otherNetAttachDef.SetName(otherNetAttachDefNamme)

	otherRoleBindingName := "other-role-binding"
	otherRoleBinding := newRoleBinding(appNamespace, otherRoleBindingName)

	cl, _ := test.CreateClient(namespace, meshRoleBinding, otherNetAttachDef, otherRoleBinding)

	fakeNetworkStrategy := &fakeNetworkStrategy{}

	// 1. check if reconcileNamespaceInMesh preserves other resources
	assertReconcileNamespaceSucceeds(t, cl, fakeNetworkStrategy)

	// 1a. check if other namespace labels were preserved
	ns := &core.Namespace{}
	test.GetObject(cl, types.NamespacedName{Name: appNamespace}, ns)
	assert.Equals(ns.Labels[otherLabelName], otherLabelValue, "Expected reconcileNamespaceInMesh to preserve other namespace labels, but it didn't", t)

	// 1b. check if other NetworkAttachmentDefinitions were preserved
	nad := newNetworkAttachmentDefinition()
	err := cl.Get(context.TODO(), types.NamespacedName{Namespace: appNamespace, Name: otherNetAttachDefNamme}, nad)
	if errors.IsNotFound(err) {
		t.Fatalf("Expected reconcileNamespaceInMesh to preserve other NetworkAttachmentDefinition, but it deleted it")
	} else if err != nil {
		panic(err)
	}
	assert.DeepEquals(nad, otherNetAttachDef, "Expected reconcileNamespaceInMesh to preserve other NetworkAttachmentDefinition, but it modified it", t)

	// 1c. check if other RoleBindings were preserved
	rb := &rbac.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{Namespace: appNamespace, Name: otherRoleBindingName}, rb)
	if errors.IsNotFound(err) {
		t.Fatalf("Expected reconcileNamespaceInMesh to preserve other RoleBinding, but it deleted it")
	} else if err != nil {
		panic(err)
	}
	assert.DeepEquals(rb, otherRoleBinding, "Expected reconcileNamespaceInMesh to preserve other RoleBinding, but it modified it", t)

	// 2. check if removeNamespaceFromMesh preserves other resources
	assertRemoveNamespaceSucceeds(t, cl, fakeNetworkStrategy)

	// 2a. check if other namespace labels were preserved
	ns = &core.Namespace{}
	test.GetObject(cl, types.NamespacedName{Name: appNamespace}, ns)
	assert.Equals(ns.Labels[otherLabelName], otherLabelValue, "Expected removeNamespaceFromMesh to preserve other namespace labels, but it didn't", t)

	// 2b. check if other NetworkAttachmentDefinitions were preserved
	nad = newNetworkAttachmentDefinition()
	err = cl.Get(context.TODO(), types.NamespacedName{Namespace: appNamespace, Name: otherNetAttachDefNamme}, nad)
	if errors.IsNotFound(err) {
		t.Fatalf("Expected removeNamespaceFromMesh to preserve other NetworkAttachmentDefinition, but it deleted it")
	} else if err != nil {
		panic(err)
	}
	assert.DeepEquals(nad, otherNetAttachDef, "Expected removeNamespaceFromMesh to preserve other NetworkAttachmentDefinition, but it modified it", t)

	// 2c. check if other RoleBindings were preserved
	rb = &rbac.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{Namespace: appNamespace, Name: otherRoleBindingName}, rb)
	if errors.IsNotFound(err) {
		t.Fatalf("Expected removeNamespaceFromMesh to preserve other RoleBinding, but it deleted it")
	} else if err != nil {
		panic(err)
	}
	assert.DeepEquals(rb, otherRoleBinding, "Expected removeNamespaceFromMesh to preserve other RoleBinding, but it modified it", t)
}

func newNetworkAttachmentDefinition() *unstructured.Unstructured {
	netAttachDef := &unstructured.Unstructured{}
	netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})
	return netAttachDef
}

func setupReconciledNamespace(t *testing.T, cl client.Client, namespace string) {
	reconciler, err := newNamespaceReconciler(cl, logf.Log, controlPlaneNamespace, meshVersionDefault, true)
	if err != nil {
		t.Fatalf("Error creating namespace reconciler: %v", err)
	}
	err = reconciler.reconcileNamespaceInMesh(namespace)
	if err != nil {
		t.Fatalf("reconcileNamespaceInMesh returned an error: %v", err)
	}
}

func assertNotFound(err error, message string, t *testing.T) {
	if err == nil {
		t.Fatalf(message)
	} else if !errors.IsNotFound(err) {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func assertReconcileNamespaceSucceeds(t *testing.T, cl client.Client, networkStrategy NamespaceReconciler) {
	reconciler, err := newNamespaceReconciler(cl, logf.Log, controlPlaneNamespace, meshVersionDefault, true)
	if err != nil {
		t.Fatalf("Error creating namespace reconciler: %v", err)
	}

	// install fake network strategy
	(reconciler.(*namespaceReconciler)).networkingStrategy = networkStrategy

	err = reconciler.reconcileNamespaceInMesh(appNamespace)
	if err != nil {
		t.Fatalf("reconcileNamespaceInMesh returned an error: %v", err)
	}
}

func assertRemoveNamespaceSucceeds(t *testing.T, cl client.Client, networkStrategy NamespaceReconciler) {
	reconciler, err := newNamespaceReconciler(cl, logf.Log, controlPlaneNamespace, meshVersionDefault, true)
	if err != nil {
		t.Fatalf("Error creating namespace reconciler: %v", err)
	}

	// install fake network strategy
	(reconciler.(*namespaceReconciler)).networkingStrategy = networkStrategy

	err = reconciler.removeNamespaceFromMesh(appNamespace)
	if err != nil {
		t.Fatalf("removeNamespaceFromMesh returned an error: %v", err)
	}
}

func assertReconcileNamespaceFails(t *testing.T, cl client.Client) {
	reconciler, err := newNamespaceReconciler(cl, logf.Log, controlPlaneNamespace, meshVersionDefault, true)
	if err != nil {
		t.Fatalf("Error creating namespace reconciler: %v", err)
	}
	err = reconciler.reconcileNamespaceInMesh(appNamespace)
	if err == nil {
		t.Fatal("Expected reconcileNamespaceInMesh to fail, but it didn't.")
	}
}

type fakeNetworkStrategy struct {
	reconciledNamespaces []string
	removedNamespaces    []string
}

func (f *fakeNetworkStrategy) reconcileNamespaceInMesh(namespace string) error {
	f.reconciledNamespaces = append(f.reconciledNamespaces, namespace)
	return nil
}

func (f *fakeNetworkStrategy) removeNamespaceFromMesh(namespace string) error {
	f.removedNamespaces = append(f.removedNamespaces, namespace)
	return nil
}
