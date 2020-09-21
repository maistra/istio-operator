package controlplane

import (
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	. "github.com/maistra/istio-operator/pkg/controller/common/test"
)

func TestWASMExtensionInstall(t *testing.T) {
	testCases := []IntegrationTestCase{
		{
			name: "wasm-extensions.enabled",
			smcp: New20SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"wasmExtensions": map[string]interface{}{
						"enabled": true,
					},
				}),
			}),
			create: IntegrationTestValidation{
				Verifier: ActionVerifier(
					Verify("create").On("deployments").Named("wasm-cacher-" + controlPlaneName).In(controlPlaneNamespace).IsSeen(),
				),
				Assertions: ActionAssertions{},
			},
			delete: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("delete").On("deployments").Named("wasm-cacher-" + controlPlaneName).In(controlPlaneNamespace).IsSeen(),
				},
			},
		},
	}
	RunSimpleInstallTest(t, testCases)
}
