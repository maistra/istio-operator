package validation

import (
	"fmt"
	"testing"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	webhookadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maistra "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func TestDeletedControlPlaneIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "istio-system")
	controlPlane.DeletionTimestamp = now()

	validator, _, _ := createControlPlaneValidatorTestFixture()
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.True(response.Response.Allowed, "Expected validator to allow deleted ServiceMeshControlPlane", t)
}

func TestControlPlaneOutsideWatchedNamespaceIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "not-watched")
	validator, _, _ := createControlPlaneValidatorTestFixture()
	validator.namespaceFilter = "watched-namespace"
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.True(response.Response.Allowed, "Expected validator to allow ServiceMeshControlPlane whose namespace isn't watched", t)
}

func TestControlPlaneWithIncorrectVersionIsRejected(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "not-watched")
	controlPlane.Spec.Version = "0.0"
	validator, _, _ := createControlPlaneValidatorTestFixture()
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshControlPlane with bad version", t)
}

func TestOnlyOneControlPlaneIsAllowedPerNamespace(t *testing.T) {
	controlPlane1 := newControlPlane("my-smcp", "istio-system")
	validator, _, _ := createControlPlaneValidatorTestFixture(controlPlane1)
	controlPlane2 := newControlPlane("my-smcp2", "istio-system")
	response := validator.Handle(ctx, createCreateRequest(controlPlane2))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshControlPlane with bad version", t)
}

func TestValidControlPlane(t *testing.T) {
	cases := []struct {
		name         string
		controlPlane *maistra.ServiceMeshControlPlane
	}{
		{
			name:         "blank-version",
			controlPlane: newControlPlane("my-smcp", "istio-system"),
		},
		{
			name:         "version-1.0",
			controlPlane: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
		},
		{
			name:         "version-1.1",
			controlPlane: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.1"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validator, _, _ := createControlPlaneValidatorTestFixture()
			response := validator.Handle(ctx, createCreateRequest(tc.controlPlane))
			assert.True(response.Response.Allowed, "Expected validator to accept valid ServiceMeshControlPlane", t)
		})
	}
}

func TestUpdateOfValidControlPlane(t *testing.T) {
	oldControlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0")
	validator, _, _ := createControlPlaneValidatorTestFixture(oldControlPlane)

	controlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "v1.1")
	response := validator.Handle(ctx, createUpdateRequest(oldControlPlane, controlPlane))
	assert.True(response.Response.Allowed, "Expected validator to accept update of valid ServiceMeshControlPlane", t)
}

func createControlPlaneValidatorTestFixture(clientObjects ...runtime.Object) (*ControlPlaneValidator, client.Client, *test.EnhancedTracker) {
	cl, tracker := test.CreateClient(clientObjects...)
	decoder, err := webhookadmission.NewDecoder(test.GetScheme())
	if err != nil {
		panic(fmt.Sprintf("Could not create decoder: %s", err))
	}
	validator := NewControlPlaneValidator("")

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

func newControlPlaneWithVersion(name, namespace, version string) *maistra.ServiceMeshControlPlane {
	controlPlane := newControlPlane(name, namespace)
	controlPlane.Spec.Version = version
	return controlPlane
}

func newControlPlane(name, namespace string) *maistra.ServiceMeshControlPlane {
	return &maistra.ServiceMeshControlPlane{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: maistra.ControlPlaneSpec{},
	}
}
