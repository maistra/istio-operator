package validation

import (
	"fmt"
	"net/http"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	webhookadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"

	maistra "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func TestDeletedMemberIsAlwaysAllowed(t *testing.T) {
	member := newMember("not-default", "app-namespace", "my-smcp", "istio-system")
	member.DeletionTimestamp = now()

	response := invokeMemberValidator(createCreateRequest(member))
	assert.True(response.Response.Allowed, "Expected validator to allow deleted ServiceMeshMember", t)
}

func newMember(name, namespace, smcpName, smcpNamespace string) *maistra.ServiceMeshMember {
	return &maistra.ServiceMeshMember{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: maistra.ServiceMeshMemberSpec{
			ControlPlaneRef: maistra.ServiceMeshControlPlaneRef{
				Name:      smcpName,
				Namespace: smcpNamespace,
			},
		},
	}
}

func TestMemberWithWrongNameIsRejected(t *testing.T) {
	member := newMember("not-default", "app-namespace", "my-smcp", "istio-system")

	response := invokeMemberValidator(createCreateRequest(member))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshMember with wrong name", t)
}

func TestMutationOfSpecControlPlaneRefIsRejected(t *testing.T) {
	cases := []struct {
		name         string
		mutateMember func(member *maistra.ServiceMeshMember)
	}{
		{
			name: "change-name",
			mutateMember: func(member *maistra.ServiceMeshMember) {
				member.Spec.ControlPlaneRef.Name = "my-smcp2"
			},
		},
		{
			name: "change-namespace",
			mutateMember: func(member *maistra.ServiceMeshMember) {
				member.Spec.ControlPlaneRef.Namespace = "istio-system2"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oldMember := newMember("default", "app-namespace", "my-smcp", "istio-system")
			newMember := oldMember.DeepCopy()
			tc.mutateMember(newMember)

			response := invokeMemberValidator(createUpdateRequest(oldMember, newMember))
			assert.False(response.Response.Allowed, "Expected validator to reject mutation of ServiceMeshMember.spec.controlPlaneRef", t)
		})
	}
}

func TestMemberWithFailedSubjectAccessReview(t *testing.T) {
	validator, _, tracker := createMemberValidatorTestFixture()
	tracker.AddReactor("create", "subjectaccessreviews", createSubjectAccessReviewReactor(false, false, nil))

	member := newMember("default", "app-namespace", "my-smcp", "istio-system")
	response := validator.Handle(ctx, createCreateRequest(member))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshMember due to failed SubjectAccessReview check", t)
}

func TestValidMemberCreation(t *testing.T) {
	validator, _, tracker := createMemberValidatorTestFixture()
	tracker.AddReactor("create", "subjectaccessreviews", createSubjectAccessReviewReactor(true, true, nil))

	member := newMember("default", "app-namespace", "my-smcp", "istio-system")
	response := validator.Handle(ctx, createCreateRequest(member))
	assert.True(response.Response.Allowed, "Expected validator to allow ServiceMeshMember", t)
}

func TestValidMemberUpdate(t *testing.T) {
	oldMember := newMember("default", "app-namespace", "my-smcp", "istio-system")
	validator, _, tracker := createMemberValidatorTestFixture()
	tracker.AddReactor("create", "subjectaccessreviews", createSubjectAccessReviewReactor(true, true, nil))

	newMember := oldMember.DeepCopy()
	newMember.Labels = map[string]string{
		"some-label": "some-label-value",
	}

	response := validator.Handle(ctx, createUpdateRequest(oldMember, newMember))
	assert.True(response.Response.Allowed, "Expected validator to accept ServiceMeshMember update", t)
}

func TestMemberValidatorRejectsRequestWhenSARCheckErrors(t *testing.T) {
	validator, _, tracker := createMemberValidatorTestFixture()
	tracker.AddReactor("create", "subjectaccessreviews", createSubjectAccessReviewReactor(true, true, fmt.Errorf("SAR check error")))

	roll := newMember("default", "app-namespace", "my-smcp", "istio-system")
	response := validator.Handle(ctx, createCreateRequest(roll))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshMember due to SAR check error", t)
	assert.Equals(response.Response.Result.Code, int32(http.StatusInternalServerError), "Unexpected result code", t)
}

func TestMemberValidatorSubmitsCorrectSubjectAccessReview(t *testing.T) {
	validator, _, tracker := createMemberValidatorTestFixture()
	tracker.AddReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(clienttesting.CreateAction)
		sar := createAction.GetObject().(*authorizationv1.SubjectAccessReview)
		assert.Equals(sar.Spec.User, userInfo.Username, "Unexpected User in SAR check", t)
		assert.Equals(sar.Spec.UID, userInfo.UID, "Unexpected UID in SAR check", t)
		assert.DeepEquals(sar.Spec.Groups, userInfo.Groups, "Unexpected Groups in SAR check", t)
		assert.DeepEquals(sar.Spec.Extra, convertUserInfoExtra(userInfo.Extra), "Unexpected Extra in SAR check", t)
		assert.Equals(sar.Spec.ResourceAttributes.Verb, "use", "Unexpected Verb in SAR check", t)
		assert.Equals(sar.Spec.ResourceAttributes.Group, "maistra.io", "Unexpected resource Group in SAR check", t)
		assert.Equals(sar.Spec.ResourceAttributes.Resource, "servicemeshcontrolplanes", "Unexpected Resource in SAR check", t)
		assert.Equals(sar.Spec.ResourceAttributes.Name, "my-smcp", "Unexpected Namespace in SAR check", t)
		assert.Equals(sar.Spec.ResourceAttributes.Namespace, "istio-system", "Unexpected Namespace in SAR check", t)
		sar.Status.Allowed = true
		return true, sar.DeepCopy(), nil
	})

	roll := newMember("default", "app-namespace", "my-smcp", "istio-system")
	_ = validator.Handle(ctx, createCreateRequest(roll))
}

func invokeMemberValidator(request atypes.Request) atypes.Response {
	validator, _, tracker := createMemberValidatorTestFixture()
	tracker.AddReactor("create", "subjectaccessreviews", createSubjectAccessReviewReactor(true, true, nil))
	response := validator.Handle(ctx, request)
	return response
}

func createMemberValidatorTestFixture(clientObjects ...runtime.Object) (memberValidator, client.Client, *test.EnhancedTracker) {
	cl, tracker := test.CreateClient(clientObjects...)
	decoder, err := webhookadmission.NewDecoder(test.GetScheme())
	if err != nil {
		panic(fmt.Sprintf("Could not create decoder: %s", err))
	}
	validator := memberValidator{}

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
