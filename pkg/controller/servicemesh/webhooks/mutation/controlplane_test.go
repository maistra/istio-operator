package mutation

import (
	"fmt"
	"testing"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func TestDeletedV1ControlPlaneIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlaneV1("my-smcp", "istio-system")
	controlPlane.Spec.Version = ""
	controlPlane.Spec.Template = ""
	controlPlane.DeletionTimestamp = now()

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept deleted ServiceMeshControlPlane", t)
}

func TestDeletedV2ControlPlaneIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlaneV2("my-smcp", "istio-system")
	controlPlane.Spec.Version = ""
	controlPlane.Spec.Template = ""
	controlPlane.DeletionTimestamp = now()

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept deleted ServiceMeshControlPlane", t)
}

func TestV1ControlPlaneOutsideWatchedNamespaceIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlaneV1("my-smcp", "not-watched")
	controlPlane.Spec.Version = ""
	controlPlane.Spec.Template = ""
	mutator, _, _ := createControlPlaneMutatorTestFixture()
	mutator.namespaceFilter = "watched-namespace"
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept ServiceMeshControlPlane whose namespace isn't watched", t)
}

func TestV2ControlPlaneOutsideWatchedNamespaceIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlaneV2("my-smcp", "not-watched")
	controlPlane.Spec.Version = ""
	controlPlane.Spec.Template = ""
	mutator, _, _ := createControlPlaneMutatorTestFixture()
	mutator.namespaceFilter = "watched-namespace"
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accept ServiceMeshControlPlane whose namespace isn't watched", t)
}

func TestV1ControlPlaneNoMutation(t *testing.T) {
	controlPlane := newControlPlaneV1("my-smcp", "istio-system")
	controlPlane.Spec.Version = versions.DefaultVersion.String()
	controlPlane.Spec.Template = maistrav1.DefaultTemplate

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accepet ServiceMeshControlPlane with no changes", t)
}

func TestV2ControlPlaneNoMutation(t *testing.T) {
	controlPlane := newControlPlaneV2("my-smcp", "istio-system")
	controlPlane.Spec.Version = versions.DefaultVersion.String()
	controlPlane.Spec.Template = maistrav1.DefaultTemplate

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	assert.DeepEquals(response, acceptWithNoMutation, "Expected mutator to accepet ServiceMeshControlPlane with no changes", t)
}

func TestV1VersionIsDefaultedToCurrentMaistraVersionOnCreate(t *testing.T) {
	controlPlane := newControlPlaneV1("my-smcp", "istio-system")
	controlPlane.Spec.Version = ""

	mutatedControlPlane := controlPlane.DeepCopy()
	mutatedControlPlane.Spec.Version = versions.DefaultVersion.String()

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	expectedResponse := PatchResponse(toRawExtension(controlPlane), mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the version on create", t)
}

// Test if the webhook defaults Version to the existing Version on an update
func TestV2VersionIsDefaultedToCurrentMaistraVersionOnCreate(t *testing.T) {
	controlPlane := newControlPlaneV2("my-smcp", "istio-system")
	controlPlane.Spec.Version = ""

	mutatedControlPlane := controlPlane.DeepCopy()
	mutatedControlPlane.Spec.Version = versions.DefaultVersion.String()

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	expectedResponse := PatchResponse(toRawExtension(controlPlane), mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the version on create", t)
}

func TestV1VersionIsDefaultedToOldSMCPVersionOnUpdate(t *testing.T) {
	cases := []struct {
		name    string
		version string
	}{
		{
			name:    "legacy-default",
			version: "",
		},
		{
			name:    "v1.0",
			version: "v1.0",
		},
		{
			name:    "v1.1",
			version: "v1.1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controlPlane := newControlPlaneV1("my-smcp", "istio-system")
			controlPlane.Spec.Version = tc.version

			updatedControlPlane := controlPlane.DeepCopy()
			updatedControlPlane.Spec.Version = ""
			updatedControlPlane.Labels = map[string]string{"newLabel": "newValue"}

			mutatedControlPlane := updatedControlPlane.DeepCopy()
			mutatedControlPlane.Spec.Version = controlPlane.Spec.Version

			mutator, _, _ := createControlPlaneMutatorTestFixture(controlPlane)
			response := mutator.Handle(ctx, newUpdateRequest(controlPlane, updatedControlPlane))
			expectedResponse := PatchResponse(toRawExtension(updatedControlPlane), mutatedControlPlane)
			if len(expectedResponse.Patches) == 0 {
				// PatchResponse() always creates a Patches array, so set it to nil if it's empty
				expectedResponse.Patches = nil
			}
			assert.DeepEquals(response, expectedResponse, "Expected the response to set the version to previously AppliedVersion on update", t)
		})
	}
}

func TestV2VersionIsDefaultedToOldSMCPVersionOnUpdate(t *testing.T) {
	cases := []struct {
		name    string
		version string
	}{
		{
			name:    "legacy-default",
			version: "",
		},
		{
			name:    "v1.0",
			version: "v1.0",
		},
		{
			name:    "v1.1",
			version: "v1.1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controlPlane := newControlPlaneV2("my-smcp", "istio-system")
			controlPlane.Spec.Version = tc.version

			updatedControlPlane := controlPlane.DeepCopy()
			updatedControlPlane.Spec.Version = ""
			updatedControlPlane.Labels = map[string]string{"newLabel": "newValue"}

			mutatedControlPlane := updatedControlPlane.DeepCopy()
			mutatedControlPlane.Spec.Version = controlPlane.Spec.Version

			mutator, _, _ := createControlPlaneMutatorTestFixture(controlPlane)
			response := mutator.Handle(ctx, newUpdateRequest(controlPlane, updatedControlPlane))
			expectedResponse := PatchResponse(toRawExtension(updatedControlPlane), mutatedControlPlane)
			if len(expectedResponse.Patches) == 0 {
				// PatchResponse() always creates a Patches array, so set it to nil if it's empty
				expectedResponse.Patches = nil
			}
			assert.DeepEquals(response, expectedResponse, "Expected the response to set the version to previously AppliedVersion on update", t)
		})
	}
}

func TestV1TemplateIsDefaultedOnCreate(t *testing.T) {
	controlPlane := newControlPlaneV1("my-smcp", "istio-system")
	controlPlane.Spec.Template = ""

	mutatedControlPlane := controlPlane.DeepCopy()
	mutatedControlPlane.Spec.Profiles = []string{maistrav1.DefaultTemplate}

	mutator, _, _ := createControlPlaneMutatorTestFixture()

	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	expectedResponse := PatchResponse(toRawExtension(controlPlane), mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the template on create", t)
}

func TestV2TemplateIsDefaultedOnCreate(t *testing.T) {
	controlPlane := newControlPlaneV2("my-smcp", "istio-system")
	controlPlane.Spec.Template = ""

	mutatedControlPlane := controlPlane.DeepCopy()
	mutatedControlPlane.Spec.Profiles = []string{maistrav1.DefaultTemplate}

	mutator, _, _ := createControlPlaneMutatorTestFixture()

	response := mutator.Handle(ctx, newCreateRequest(controlPlane))
	expectedResponse := PatchResponse(toRawExtension(controlPlane), mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the template on create", t)
}

func TestV1TemplateIsDefaultedOnUpdate(t *testing.T) {
	origControlPlane := newControlPlaneV1("my-smcp", "istio-system")
	origControlPlane.Spec.Template = ""

	updatedControlPlane := origControlPlane.DeepCopy()
	updatedControlPlane.Labels = map[string]string{"newLabel": "newValue"}

	mutatedControlPlane := updatedControlPlane.DeepCopy()
	mutatedControlPlane.Spec.Profiles = []string{maistrav1.DefaultTemplate}

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newUpdateRequest(origControlPlane, updatedControlPlane))
	expectedResponse := PatchResponse(toRawExtension(updatedControlPlane), mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the template on update", t)
}

func TestV2TemplateIsDefaultedOnUpdate(t *testing.T) {
	origControlPlane := newControlPlaneV2("my-smcp", "istio-system")
	origControlPlane.Spec.Template = ""

	updatedControlPlane := origControlPlane.DeepCopy()
	updatedControlPlane.Labels = map[string]string{"newLabel": "newValue"}

	mutatedControlPlane := updatedControlPlane.DeepCopy()
	mutatedControlPlane.Spec.Profiles = []string{maistrav1.DefaultTemplate}

	mutator, _, _ := createControlPlaneMutatorTestFixture()
	response := mutator.Handle(ctx, newUpdateRequest(origControlPlane, updatedControlPlane))
	expectedResponse := PatchResponse(toRawExtension(updatedControlPlane), mutatedControlPlane)
	assert.DeepEquals(response, expectedResponse, "Expected the response to set the template on update", t)
}

func createControlPlaneMutatorTestFixture(clientObjects ...runtime.Object) (*ControlPlaneMutator, client.Client, *test.EnhancedTracker) {
	cl, tracker := test.CreateClient(clientObjects...)
	decoder, err := admission.NewDecoder(test.GetScheme())
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

func newControlPlaneV1(name, namespace string) *maistrav1.ServiceMeshControlPlane {
	return &maistrav1.ServiceMeshControlPlane{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: maistrav1.ControlPlaneSpec{
			Version:  versions.DefaultVersion.String(),
			Template: maistrav1.DefaultTemplate,
		},
	}
}

func newControlPlaneV2(name, namespace string) *maistrav2.ServiceMeshControlPlane {
	return &maistrav2.ServiceMeshControlPlane{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: maistrav2.ControlPlaneSpec{
			Version:  versions.DefaultVersion.String(),
			Template: maistrav1.DefaultTemplate,
		},
	}
}
