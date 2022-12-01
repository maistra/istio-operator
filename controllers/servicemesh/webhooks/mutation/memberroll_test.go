package mutation

import (
	"fmt"
	"testing"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maistra "github.com/maistra/istio-operator/apis/maistra/v1"
	"github.com/maistra/istio-operator/controllers/common/test"
	"github.com/maistra/istio-operator/controllers/common/test/assert"
)

func TestDeletedMemberRollIsAlwaysAllowed(t *testing.T) {
	roll := newMemberRoll("not-default", "istio-system")
	roll.DeletionTimestamp = now()

	mutator := createMemberRollMutatorFixture()
	response := mutator.Handle(ctx, newCreateRequest(roll))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept deleted ServiceMeshMemberRoll", t)
}

func TestMemberRollOutsideWatchedNamespaceIsAlwaysAllowed(t *testing.T) {
	roll := newMemberRoll("not-default", "not-watched")
	mutator := createMemberRollMutatorFixture()
	mutator.namespaceFilter = "watched-namespace"
	response := mutator.Handle(ctx, newCreateRequest(roll))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept ServiceMeshMemberRoll whose namespace isn't watched", t)
}

func TestMemberRollNoMutation(t *testing.T) {
	roll := newMemberRoll("default", "istio-system", "bookinfo", "hipster-shop")
	mutator := createMemberRollMutatorFixture()
	response := mutator.Handle(ctx, newCreateRequest(roll))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept ServiceMeshMemberRoll with no changes", t)
}

func TestControlPlaneNamespaceIsRemovedFromMembersList(t *testing.T) {
	roll := newMemberRoll("default", "istio-system")
	roll.Spec.Members = []string{"bookinfo", "istio-system", "istio-system", "hipster-shop", "istio-system"}

	mutatedRoll := roll.DeepCopy()
	mutatedRoll.Spec.Members = []string{"bookinfo", "hipster-shop"}

	mutator := createMemberRollMutatorFixture()

	// check create
	response := mutator.Handle(ctx, newCreateRequest(roll))
	expectedResponse := PatchResponse(toRawExtension(roll), mutatedRoll)
	assert.DeepEquals(response, expectedResponse, "Unexpected response on create", t)

	// check update
	updatedRoll := roll.DeepCopy()
	updatedRoll.Spec.Members = []string{"istio-system", "bookinfo", "istio-system", "hipster-shop", "istio-system"}

	response = mutator.Handle(ctx, newUpdateRequest(roll, updatedRoll))
	expectedResponse = PatchResponse(toRawExtension(updatedRoll), mutatedRoll)
	assert.DeepEquals(response, expectedResponse, "Unexpected response on update", t)
}

func createMemberRollMutatorFixture() *MemberRollMutator {
	cl, _ := test.CreateClient()
	decoder, err := admission.NewDecoder(test.GetScheme())
	if err != nil {
		panic(fmt.Sprintf("Could not create decoder: %s", err))
	}
	validator := NewMemberRollMutator("")

	err = validator.InjectClient(cl)
	if err != nil {
		panic(fmt.Sprintf("Could not inject client: %s", err))
	}

	err = validator.InjectDecoder(decoder)
	if err != nil {
		panic(fmt.Sprintf("Could not inject decoder: %s", err))
	}

	return validator
}

func newMemberRoll(name, namespace string, members ...string) *maistra.ServiceMeshMemberRoll {
	return &maistra.ServiceMeshMemberRoll{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: maistra.ServiceMeshMemberRollSpec{
			Members: members,
		},
	}
}
