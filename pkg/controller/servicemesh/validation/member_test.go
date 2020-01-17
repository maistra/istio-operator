package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	admission "k8s.io/api/admission/v1beta1"
	authorization "k8s.io/api/authorization/v1"
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
	member := &maistra.ServiceMeshMember{
		ObjectMeta: meta.ObjectMeta{
			Name:              "not-default",
			DeletionTimestamp: now(),
		},
	}

	response := invokeValidator(createCreateRequest(member))
	assert.True(response.Response.Allowed, "Expected validator to allow deleted ServiceMeshMember", t)
}

func TestMemberWithWrongNameIsRejected(t *testing.T) {
	member := &maistra.ServiceMeshMember{
		ObjectMeta: meta.ObjectMeta{
			Name: "not-default",
		},
	}

	response := invokeValidator(createCreateRequest(member))
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

			response := invokeValidator(createUpdateRequest(oldMember, newMember))
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

	validator, _, tracker := createTestFixture()
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

	validator, _, tracker := createTestFixture()
	tracker.AddReactor(createSubjectAccessReviewReactor(true))

	response := validator.Handle(context.TODO(), createCreateRequest(member))
	assert.True(response.Response.Allowed, "Expected validator to allow ServiceMeshMember", t)
}

func createSubjectAccessReviewReactor(allowed bool) func(action clienttesting.Action) (handled bool, err error) {
	return func(action clienttesting.Action) (handled bool, err error) {
		if action.Matches("create", "subjectaccessreviews") {
			createAction := action.(clienttesting.CreateAction)
			sar := createAction.GetObject().(*authorization.SubjectAccessReview)
			sar.Status.Allowed = allowed
			return true, nil
		}
		return false, nil
	}
}

func createCreateRequest(member *maistra.ServiceMeshMember) atypes.Request {
	request := atypes.Request{
		AdmissionRequest: &admission.AdmissionRequest{
			Operation: admission.Create,
			Object:    toRawExtension(member),
		},
	}
	return request
}

func createUpdateRequest(oldMember *maistra.ServiceMeshMember, newMember *maistra.ServiceMeshMember) atypes.Request {
	request := atypes.Request{
		AdmissionRequest: &admission.AdmissionRequest{
			Operation: admission.Update,
			Object:    toRawExtension(newMember),
			OldObject: toRawExtension(oldMember),
		},
	}
	return request
}

func createDeleteRequest(newMember *maistra.ServiceMeshMember) atypes.Request {
	request := atypes.Request{
		AdmissionRequest: &admission.AdmissionRequest{
			Operation: admission.Delete,
			Object:    toRawExtension(newMember),
		},
	}
	return request
}

func invokeValidator(request atypes.Request) atypes.Response {
	validator, _, _ := createTestFixture()
	response := validator.Handle(context.TODO(), request)
	return response
}

func createTestFixture() (memberValidator, client.Client, *test.EnhancedTracker) {
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

func toRawExtension(member *maistra.ServiceMeshMember) runtime.RawExtension {
	memberJson, err := json.Marshal(member)
	if err != nil {
		panic(fmt.Sprintf("Could not marshal ServiceMeshMember: %s", err))
	}

	return runtime.RawExtension{
		Raw: memberJson,
	}
}

func now() *meta.Time {
	now := meta.Now()
	return &now
}
