package mutation

import (
	"fmt"
	"testing"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	webhookadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/maistra/istio-operator/pkg/apis/maistra"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func TestDeletedControlPlaneIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "istio-system")
	controlPlane.Spec.Version = ""
	controlPlane.Spec.Template = ""
	controlPlane.DeletionTimestamp = now()

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept deleted ServiceMeshControlPlane", t)
}

func TestControlPlaneOutsideWatchedNamespaceIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "not-watched")
	controlPlane.Spec.Version = ""
	controlPlane.Spec.Template = ""
	mutator, _, _ := createControlPlaneMutatorTestFixture()
	mutator.namespaceFilter = "watched-namespace"
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept ServiceMeshControlPlane whose namespace isn't watched", t)
}

func TestControlPlaneNoMutation(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "istio-system")
	controlPlane.Spec.Version = maistra.DefaultVersion.String()
	controlPlane.Spec.Template = maistrav1.DefaultTemplate

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accepet ServiceMeshControlPlane with no changes", t)
}

func TestVersionIsDefaultedToCurrentMaistraVersionOnCreate(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "istio-system")
	controlPlane.Spec.Version = ""

	mutatedControlPlane := controlPlane.DeepCopy()
	mutatedControlPlane.Spec.Version = maistra.DefaultVersion.String()

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	expectedResponse := webhookadmission.PatchResponse(controlPlane, mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the version on create", t)
}

// Test if the webhook should default the version to the existing AppliedVersion on an update
func TestVersionIsDefaultedToAppliedVersionOnUpdate(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "istio-system")
	controlPlane.Spec.Version = ""
	controlPlane.Status.AppliedVersion = maistra.V1_1.String()

	updatedControlPlane := controlPlane.DeepCopy()
	updatedControlPlane.Labels = map[string]string{"newLabel": "newValue"}

	mutatedControlPlane := controlPlane.DeepCopy()
	mutatedControlPlane.Spec.Version = controlPlane.Status.AppliedVersion

	mutator, _, _ := createControlPlaneMutatorTestFixture(controlPlane)
	response := mutator.Handle(ctx, newUpdateRequest(controlPlane, updatedControlPlane))
	expectedResponse := webhookadmission.PatchResponse(controlPlane, mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the version to previously AppliedVersion on update", t)
}

func TestVersionIsDefaultedToLegacyVersionOnUpdate(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "istio-system")
	controlPlane.Spec.Version = ""

	updatedControlPlane := controlPlane.DeepCopy()
	updatedControlPlane.Labels = map[string]string{"newLabel": "newValue"}

	mutatedControlPlane := controlPlane.DeepCopy()
	mutatedControlPlane.Spec.Version = maistra.LegacyVersion.String()

	mutator, _, _ := createControlPlaneMutatorTestFixture(controlPlane)
	response := mutator.Handle(ctx, newUpdateRequest(controlPlane, updatedControlPlane))
	expectedResponse := webhookadmission.PatchResponse(controlPlane, mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the version to previously AppliedVersion on update", t)
}

func TestTemplateIsDefaultedOnCreate(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "istio-system")
	controlPlane.Spec.Template = ""

	mutatedControlPlane := controlPlane.DeepCopy()
	mutatedControlPlane.Spec.Template = maistrav1.DefaultTemplate

	mutator, _, _ := createControlPlaneMutatorTestFixture()

	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	expectedResponse := webhookadmission.PatchResponse(controlPlane, mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the template on create", t)
}

func TestTemplateIsDefaultedOnUpdate(t *testing.T) {
	origControlPlane := newControlPlane("my-smcp", "istio-system")
	origControlPlane.Spec.Template = ""

	updatedControlPlane := origControlPlane.DeepCopy()
	updatedControlPlane.Labels = map[string]string{"newLabel": "newValue"}

	mutatedControlPlane := updatedControlPlane.DeepCopy()
	mutatedControlPlane.Spec.Template = maistrav1.DefaultTemplate

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newUpdateRequest(origControlPlane, updatedControlPlane))
	expectedResponse := webhookadmission.PatchResponse(updatedControlPlane, mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the template on update", t)
}

func createControlPlaneMutatorTestFixture(clientObjects ...runtime.Object) (*ControlPlaneMutator, client.Client, *test.EnhancedTracker) {
	cl, tracker := test.CreateClient(clientObjects...)
	decoder, err := webhookadmission.NewDecoder(test.GetScheme())
	if err != nil {
		panic(fmt.Sprintf("Could not create decoder: %s", err))
	}
	validator := NewControlPlaneMutator("")

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

func newControlPlane(name, namespace string) *maistrav1.ServiceMeshControlPlane {
	return &maistrav1.ServiceMeshControlPlane{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: maistrav1.ControlPlaneSpec{
			Version:  maistra.DefaultVersion.String(),
			Template: maistrav1.DefaultTemplate,
		},
	}
}
