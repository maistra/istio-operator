package member

import (
	"testing"

	networking "k8s.io/api/networking/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func TestMeshNetworkPolicyIsCopiedIntoAppNamespace(t *testing.T) {
	meshNetworkPolicy := newMeshNetworkPolicy()

	cl, _ := test.CreateClient(meshNetworkPolicy)
	strategy := createNetworkPolicyStrategy(cl, t)
	assert.Success(strategy.reconcileNamespaceInMesh(ctx, appNamespace), "reconcileNamespaceInMesh", t)

	nsNetworkPolicy := getNamespaceNetworkPolicy(cl, t)

	expectedNsNetworkPolicy := newMeshNetworkPolicy()
	// the NetworkPolicy must not contain the maistra.io/owner label or any
	// other labels of the NetworkPolicy in the control plane namespace so
	// that it won't be deleted by the pruner, but it must contain the
	// maistra.io/member-of label, so that it can be later deleted by the
	// SMM controller
	expectedNsNetworkPolicy.Labels = map[string]string{
		"my-label":         "foo",
		common.MemberOfKey: controlPlaneNamespace,
	}
	expectedNsNetworkPolicy.Namespace = appNamespace

	assert.DeepEquals(nsNetworkPolicy, expectedNsNetworkPolicy, "Unexpected state of app namespace NetworkPolicy", t)
}

func TestObsoleteMeshNetworkPolicyIsRemovedFromAppNamespace(t *testing.T) {
	meshNetworkPolicy := newMeshNetworkPolicy()

	cl, _ := test.CreateClient(meshNetworkPolicy)
	setupNetworkPolicyReconciledNamespace(t, cl, appNamespace)

	// a copy of the meshNetworkPolicy is now in the app namespace
	// we now delete the meshNetworkPolicy from the mesh namespace
	err := cl.Delete(ctx, meshNetworkPolicy)
	if err != nil {
		t.Fatalf("Couldn't delete NetworkPolicy: %v", err)
	}

	strategy := createNetworkPolicyStrategy(cl, t)
	// this should remove the NetworkPolicy from the namespace
	assert.Success(strategy.reconcileNamespaceInMesh(ctx, appNamespace), "reconcileNamespaceInMesh", t)

	nsNetworkPolicy := &networking.NetworkPolicy{}
	err = cl.Get(ctx, types.NamespacedName{Namespace: appNamespace, Name: "my-policy"}, nsNetworkPolicy)
	assertNotFound(err, "Expected NetworkPolicy to have been removed from app namespace, but it is still present", t)
}

func TestInternalMeshNetworkPolicyIsNotCopiedIntoAppNamespace(t *testing.T) {
	meshNetworkPolicy := newMeshNetworkPolicy()
	meshNetworkPolicy.Annotations[common.InternalKey] = "true"

	cl, _ := test.CreateClient(meshNetworkPolicy)
	strategy := createNetworkPolicyStrategy(cl, t)
	assert.Success(strategy.reconcileNamespaceInMesh(ctx, appNamespace), "reconcileNamespaceInMesh", t)

	nsNetworkPolicy := &networking.NetworkPolicy{}
	err := cl.Get(ctx, types.NamespacedName{Namespace: appNamespace, Name: "my-policy"}, nsNetworkPolicy)
	assertNotFound(err, "Expected NetworkPolicy to not be in the app namespace", t)
}

func TestRemoveNamespaceInMeshRemovesTheCorrectNetworkPolicies(t *testing.T) {
	meshNetworkPolicy := newMeshNetworkPolicy()
	nonMeshNetworkPolicy := &networking.NetworkPolicy{
		ObjectMeta: meta.ObjectMeta{
			Namespace: appNamespace,
			Name:      "policy-not-related-to-service-mesh",
		},
	}

	cl, _ := test.CreateClient(meshNetworkPolicy, nonMeshNetworkPolicy)
	setupNetworkPolicyReconciledNamespace(t, cl, appNamespace)

	// a copy of the mesh network policy is now in the app namespace

	strategy := createNetworkPolicyStrategy(cl, t)
	assert.Success(strategy.removeNamespaceFromMesh(ctx, appNamespace), "removeNamespaceFromMesh", t)

	nsNetworkPolicy := &networking.NetworkPolicy{}
	err := cl.Get(ctx, types.NamespacedName{Namespace: appNamespace, Name: "my-policy"}, nsNetworkPolicy)
	assertNotFound(err, "Expected NetworkPolicy to have been removed from app namespace, but it is still present", t)

	nonMeshNamespacedName := types.NamespacedName{Namespace: appNamespace, Name: "policy-not-related-to-service-mesh"}
	err = cl.Get(ctx, nonMeshNamespacedName, nsNetworkPolicy)
	assert.Success(err, "cl.Get", t)
	test.AssertObjectExists(ctx, cl, nonMeshNamespacedName, nsNetworkPolicy,
		"Expected policy not related to service mesh to still be present, but it was removed", t)
}

func getNamespaceNetworkPolicy(cl client.Client, t *testing.T) *networking.NetworkPolicy {
	nsNetworkPolicy := &networking.NetworkPolicy{}
	err := cl.Get(ctx, types.NamespacedName{Namespace: appNamespace, Name: "my-policy"}, nsNetworkPolicy)
	if err != nil {
		t.Fatalf("Error getting NetworkPolicy in app namespace: %v", err)
	}
	return nsNetworkPolicy
}

func setupNetworkPolicyReconciledNamespace(t *testing.T, cl client.Client, namespace string) {
	strategy := createNetworkPolicyStrategy(cl, t)
	assert.Success(strategy.reconcileNamespaceInMesh(ctx, namespace), "reconcileNamespaceInMesh", t)
}

func createNetworkPolicyStrategy(cl client.Client, t *testing.T) *networkPolicyStrategy {
	strategy, err := newNetworkPolicyStrategy(ctx, cl, controlPlaneNamespace)
	if err != nil {
		t.Fatalf("Error creating network strategy: %v", err)
	}
	return strategy
}

func newMeshNetworkPolicy() *networking.NetworkPolicy {
	meshNetworkPolicy := &networking.NetworkPolicy{
		ObjectMeta: meta.ObjectMeta{
			Namespace: controlPlaneNamespace,
			Name:      "my-policy",
			Labels: map[string]string{
				"my-label":      "foo",
				common.OwnerKey: controlPlaneNamespace,
			},
			Annotations: map[string]string{
				"my-annotation": "bar",
			},
		},
		Spec: networking.NetworkPolicySpec{
			PolicyTypes: []networking.PolicyType{networking.PolicyTypeIngress, networking.PolicyTypeEgress},
			PodSelector: meta.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
			Ingress: []networking.NetworkPolicyIngressRule{
				{
					Ports: []networking.NetworkPolicyPort{
						{
							Port: intOrStringFromInt(8080),
						},
					},
				},
			},
			Egress: []networking.NetworkPolicyEgressRule{
				{
					Ports: []networking.NetworkPolicyPort{
						{
							Port: intOrStringFromInt(9090),
						},
					},
				},
			},
		},
	}
	return meshNetworkPolicy
}

func intOrStringFromInt(num int) *intstr.IntOrString {
	v := intstr.FromInt(num)
	return &v
}
