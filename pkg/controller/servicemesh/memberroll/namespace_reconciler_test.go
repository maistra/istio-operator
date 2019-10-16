package memberroll

import (
	"context"
	"testing"

	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func assertNamespaceReconciled(t *testing.T, cl client.Client, namespace, meshNamespace string, meshRoleBindings []rbac.RoleBinding) {
	// check if namespace has member-of label
	ns := &core.Namespace{}
	test.GetObject(cl, types.NamespacedName{Name: namespace}, ns)
	assert.Equals(ns.Labels[common.MemberOfKey], meshNamespace, "Unexpected or missing member-of label in namespace", t)

	// check if net-attach-def exists
	netAttachDef := &unstructured.Unstructured{}
	netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})
	err := cl.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: netAttachDefName}, netAttachDef)
	if err != nil {
		t.Fatalf("Couldn't get NetworkAttachmentDefinition from client: %v", err)
	}

	// check role bindings exist
	roleBindings := rbac.RoleBindingList{}
	err = cl.List(context.TODO(), client.InNamespace(namespace), &roleBindings)
	if err != nil {
		t.Fatalf("Couldn't list RoleBindings: %v", err)
	}

	expectedRoleBindings := []rbac.RoleBinding{}
	for _, meshRB := range meshRoleBindings {
		expectedRB := meshRB.DeepCopy()
		expectedRB.Namespace = namespace
		expectedRB.Labels[common.MemberOfKey] = meshNamespace
		expectedRoleBindings = append(expectedRoleBindings, *expectedRB)
	}

	assert.DeepEquals(roleBindings.Items, expectedRoleBindings, "Unexpected RoleBindings found in namespace", t)

	// TODO: check networking
}
