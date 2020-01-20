package validation

import (
	"context"
	"fmt"
	"testing"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	webhookadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"

	maistra "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func TestDeletedMemberIsAlwaysAllowed(t *testing.T) {
	member := &maistra.ServiceMeshMember{
		ObjectMeta: meta.ObjectMeta{
			Name:              "not-default",
			DeletionTimestamp: now(),
		},
	}

	response := invokeMemberValidator(createCreateRequest(member))
	assert.True(response.Response.Allowed, "Expected validator to allow deleted ServiceMeshMember", t)
}

func TestMemberWithWrongNameIsRejected(t *testing.T) {
	member := &maistra.ServiceMeshMember{
		ObjectMeta: meta.ObjectMeta{
			Name: "not-default",
		},
	}

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
			oldMember := &maistra.ServiceMeshMember{
				ObjectMeta: meta.ObjectMeta{
					Name: "default",
				},
				Spec: maistra.ServiceMeshMemberSpec{
					ControlPlaneRef: maistra.ServiceMeshControlPlaneRef{
						Name:      "my-smcp",
						Namespace: "istio-system",
					},
				},
			}
			newMember := oldMember.DeepCopy()
			tc.mutateMember(newMember)

			response := invokeMemberValidator(createUpdateRequest(oldMember, newMember))
			assert.False(response.Response.Allowed, "Expected validator to reject mutation of ServiceMeshMember.spec.controlPlaneRef", t)
		})
	}
}

func TestMemberWithFailedSubjectAccessReview(t *testing.T) {
	member := &maistra.ServiceMeshMember{
		ObjectMeta: meta.ObjectMeta{
			Name: "default",
		},
	}

	validator, _, tracker := createMemberValidatorTestFixture()
	tracker.AddReactor(createSubjectAccessReviewReactor(false))

	response := validator.Handle(context.TODO(), createCreateRequest(member))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshMember due to failed SubjectAccessReview check", t)
}

func TestValidMember(t *testing.T) {
	member := &maistra.ServiceMeshMember{
		ObjectMeta: meta.ObjectMeta{
			Name: "default",
		},
	}

	validator, _, tracker := createMemberValidatorTestFixture()
	tracker.AddReactor(createSubjectAccessReviewReactor(true))

	response := validator.Handle(context.TODO(), createCreateRequest(member))
	assert.True(response.Response.Allowed, "Expected validator to allow ServiceMeshMember", t)
}

func invokeMemberValidator(request atypes.Request) atypes.Response {
	validator, _, _ := createMemberValidatorTestFixture()
	response := validator.Handle(context.TODO(), request)
	return response
}

func createMemberValidatorTestFixture() (memberValidator, client.Client, *test.EnhancedTracker) {
	cl, tracker := test.CreateClient()
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
