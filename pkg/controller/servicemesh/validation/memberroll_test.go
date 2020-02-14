package validation

import (
	"fmt"
	"net/http"
	"testing"

	authorization "k8s.io/api/authorization/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	webhookadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maistra "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

var smcp = &maistra.ServiceMeshControlPlane{
	ObjectMeta: meta.ObjectMeta{
		Name:      "my-smcp",
		Namespace: "istio-system",
	},
}

func TestDeletedMemberRollIsAlwaysAllowed(t *testing.T) {
	roll := newMemberRoll("not-default", "istio-system")
	roll.DeletionTimestamp = now()

	validator, _, _ := createMemberRollValidatorTestFixture(smcp)
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.True(response.Response.Allowed, "Expected validator to allow deleted ServiceMeshMemberRoll", t)
}

func TestMemberRollOutsideWatchedNamespaceIsAlwaysAllowed(t *testing.T) {
	roll := newMemberRoll("not-default", "not-watched")
	watchNamespace = "watched-namespace"
	defer func() { watchNamespace = "" }()
	validator, _, _ := createMemberRollValidatorTestFixture(smcp)
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.True(response.Response.Allowed, "Expected validator to allow ServiceMeshMemberRoll whose namespace isn't watched", t)
}

func TestMemberRollWithWrongNameIsRejected(t *testing.T) {
	roll := newMemberRoll("not-default", "istio-system")

	validator, _, _ := createMemberRollValidatorTestFixture(smcp)
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshMemberRoll with wrong name", t)
}

func TestMemberRollIsRejectedWhenNoControlPlaneInNamespace(t *testing.T) {
	roll := newMemberRoll("default", "istio-system")

	validator, _, _ := createMemberRollValidatorTestFixture() // NOTE: no SMCP
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshMemberRoll because no SMCP exists in the namespace", t)
}

func TestMemberRollWithConflictingNamespaceIsRejected(t *testing.T) {
	otherRoll := newMemberRoll("default", "istio-system2", "already-in-another-roll")
	validator, _, _ := createMemberRollValidatorTestFixture(smcp, otherRoll)

	roll := newMemberRoll("default", "istio-system", "already-in-another-roll")
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshMemberRoll containing member contained in another ServiceMeshMemberRoll", t)
}

func TestMemberRollWithControlPlaneNamespaceIsRejected(t *testing.T) {
	validator, _, _ := createMemberRollValidatorTestFixture(smcp)

	roll := newMemberRoll("default", "istio-system", "istio-system")
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshMemberRoll containing control plane namespace as member", t)
}

func TestMemberRollWithFailedSubjectAccessReview(t *testing.T) {
	validator, _, tracker := createMemberRollValidatorTestFixture(smcp)
	tracker.AddReactor("create", "subjectaccessreviews", createSubjectAccessReviewReactor(false, false, nil))

	roll := newMemberRoll("default", "istio-system", "app-namespace")
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshMemberRoll due to failed SubjectAccessReview check", t)
}

func TestValidMemberRoll(t *testing.T) {
	validator, _, tracker := createMemberRollValidatorTestFixture(smcp)
	tracker.AddReactor("create", "subjectaccessreviews", createSubjectAccessReviewReactor(true, true, nil))

	roll := newMemberRoll("default", "istio-system", "app-namespace")
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.True(response.Response.Allowed, "Expected validator to allow ServiceMeshMemberRoll", t)
}

func TestClusterScopedSARCheckSuffices(t *testing.T) {
	sarCheckCount := 0
	validator, _, tracker := createMemberRollValidatorTestFixture(smcp)
	tracker.AddReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		sarCheckCount++
		if sarCheckCount > 1 {
			t.Fatalf("More than one SAR check was performed")
		}

		createAction := action.(clienttesting.CreateAction)
		sar := createAction.GetObject().(*authorization.SubjectAccessReview)

		assert.Equals(sar.Spec.ResourceAttributes.Namespace, "", "Unexpected namespace in SAR check", t)
		sar.Status.Allowed = true
		return true, sar.DeepCopy(), nil
	})

	roll := newMemberRoll("default", "istio-system", "app-namespace")
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.True(response.Response.Allowed, "Expected validator to allow ServiceMeshMemberRoll", t)
}

func TestNamespaceScopedSARCheckPerformedWhenClusterScopedReturnsFalse(t *testing.T) {
	validator, _, tracker := createMemberRollValidatorTestFixture(smcp)
	tracker.AddReactor("create", "subjectaccessreviews", createSubjectAccessReviewReactor(false, true, nil))

	roll := newMemberRoll("default", "istio-system", "app-namespace")
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.True(response.Response.Allowed, "Expected validator to allow ServiceMeshMemberRoll", t)
}

func TestMemberRollValidatorRejectsRequestWhenSARCheckErrors(t *testing.T) {
	validator, _, tracker := createMemberRollValidatorTestFixture(smcp)
	tracker.AddReactor("create", "subjectaccessreviews", createSubjectAccessReviewReactor(false, false, fmt.Errorf("SAR check error")))

	roll := newMemberRoll("default", "istio-system", "app-namespace")
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshMemberRoll due to SAR check error", t)
	assert.Equals(response.Response.Result.Code, int32(http.StatusInternalServerError), "Unexpected result code", t)
}

func TestSARCheckOnlyPerformedForNewlyAddedNamespacesOnUpdate(t *testing.T) {
	oldRoll := newMemberRoll("default", "istio-system", "app-namespace1")
	validator, _, tracker := createMemberRollValidatorTestFixture(smcp, oldRoll)
	sarCheckNumber := 0
	tracker.AddReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		sarCheckNumber++
		if sarCheckNumber > 2 {
			t.Fatalf("More than two SAR checks were performed")
		}

		createAction := action.(clienttesting.CreateAction)
		sar := createAction.GetObject().(*authorization.SubjectAccessReview)
		if sar.Spec.ResourceAttributes.Namespace == "" {
			sar.Status.Allowed = false
			return true, sar.DeepCopy(), nil
		}

		assert.Equals(sar.Spec.ResourceAttributes.Namespace, "app-namespace2", "Unexpected namespace in SAR check", t)
		sar.Status.Allowed = true
		return true, sar.DeepCopy(), nil
	})

	newRoll := oldRoll.DeepCopy()
	newRoll.Spec.Members = append(newRoll.Spec.Members, "app-namespace2")

	response := validator.Handle(ctx, createUpdateRequest(oldRoll, newRoll))
	assert.True(response.Response.Allowed, "Expected validator to accept ServiceMeshMemberRoll update", t)
}

func TestMemberRollValidatorSubmitsCorrectSubjectAccessReview(t *testing.T) {
	validator, _, tracker := createMemberRollValidatorTestFixture(smcp)
	tracker.AddReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(clienttesting.CreateAction)
		sar := createAction.GetObject().(*authorizationv1.SubjectAccessReview)
		assert.Equals(sar.Spec.User, userInfo.Username, "Unexpected User in SAR check", t)
		assert.Equals(sar.Spec.UID, userInfo.UID, "Unexpected UID in SAR check", t)
		assert.DeepEquals(sar.Spec.Groups, userInfo.Groups, "Unexpected Groups in SAR check", t)
		assert.DeepEquals(sar.Spec.Extra, convertUserInfoExtra(userInfo.Extra), "Unexpected Extra in SAR check", t)
		assert.Equals(sar.Spec.ResourceAttributes.Verb, "update", "Unexpected Verb in SAR check", t)
		assert.Equals(sar.Spec.ResourceAttributes.Group, "", "Unexpected resource Group in SAR check", t)
		assert.Equals(sar.Spec.ResourceAttributes.Resource, "pods", "Unexpected Resource in SAR check", t)
		assert.Equals(sar.Spec.ResourceAttributes.Name, "", "Unexpected resource Name in SAR check", t)
		assert.Equals(sar.Spec.ResourceAttributes.Namespace, "", "Unexpected resource Namespace in SAR check", t)
		sar.Status.Allowed = true
		return true, sar.DeepCopy(), nil
	})

	roll := newMemberRoll("default", "istio-system", "app-namespace")
	_ = validator.Handle(ctx, createCreateRequest(roll))
}

func createMemberRollValidatorTestFixture(clientObjects ...runtime.Object) (memberRollValidator, client.Client, *test.EnhancedTracker) {
	cl, tracker := test.CreateClient(clientObjects...)
	decoder, err := webhookadmission.NewDecoder(test.GetScheme())
	if err != nil {
		panic(fmt.Sprintf("Could not create decoder: %s", err))
	}
	validator := memberRollValidator{}

	err = validator.InjectClient(cl)
	if err != nil {
		panic(fmt.Sprintf("Could not inject client: %s", err))
	}

	err = validator.InjectDecoder(decoder)
	if err != nil {
		panic(fmt.Sprintf("Could not inject decoder: %s", err))
	}

	return validator, cl, tracker
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
