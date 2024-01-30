package v2

import (
	"fmt"

	"sigs.k8s.io/yaml"

	"testing"
)

var isGatewayControllerTestCases = []struct {
	name                string
	smcpSpec            *ControlPlaneSpec
	isGatewayController bool
}{
	{
		name: "techPreview gateway controller - expected true",
		smcpSpec: newSmcpSpec(`
techPreview:
  gatewayAPI:
    controllerMode: true
`),
		isGatewayController: true,
	},
	{
		name: "custom gateway controller - expected true",
		smcpSpec: newSmcpSpec(`
runtime:
  components:
    pilot:
      container:
        env:
          PILOT_ENABLE_GATEWAY_CONTROLLER_MODE: "true"
`),
		isGatewayController: true,
	},
	{
		name: "techPreview custom gateway controller - expected true (environment variable has precedence over techPreview settings)",
		smcpSpec: newSmcpSpec(`
runtime:
  components:
    pilot:
      container:
        env:
          PILOT_ENABLE_GATEWAY_CONTROLLER_MODE: "true"
techPreview:
  gatewayAPI:
    controllerMode: false
`),
		isGatewayController: true,
	},
	{
		name: "techPreview gateway controller disabled - expected false",
		smcpSpec: newSmcpSpec(`
techPreview:
  gatewayAPI:
    controllerMode: false
`),
		isGatewayController: false,
	},
	{
		name: "techPreview gateway controller with invalid value - expected false",
		smcpSpec: newSmcpSpec(`
techPreview:
  gatewayAPI:
    controllerMode: a
`),
		isGatewayController: false,
	},
	{
		name: "custom gateway controller disabled - expected false",
		smcpSpec: newSmcpSpec(`
runtime:
  components:
    pilot:
      container:
        env:
          PILOT_ENABLE_GATEWAY_CONTROLLER_MODE: "false"
`),
		isGatewayController: false,
	},
	{
		name: "custom gateway controller with invalid value - expected false",
		smcpSpec: newSmcpSpec(`
runtime:
  components:
    pilot:
      container:
        env:
          PILOT_ENABLE_GATEWAY_CONTROLLER_MODE: "a"
`),
		isGatewayController: false,
	},
	{
		name: "custom gateway controller with invalid environment variable and enabled techPreview gateway controller - expected true",
		smcpSpec: newSmcpSpec(`
runtime:
  components:
    pilot:
      container:
        env:
          PILOT_ENABLE_GATEWAY_CONTROLLER_MODE: "a"
techPreview:
  gatewayAPI:
    controllerMode: true
`),
		isGatewayController: true,
	},
	{
		name: "custom gateway controller enabled and invalid techPreview gateway controller - expected true",
		smcpSpec: newSmcpSpec(`
runtime:
  components:
    pilot:
      container:
        env:
          PILOT_ENABLE_GATEWAY_CONTROLLER_MODE: "true"
techPreview:
  gatewayAPI:
    controllerMode: a
`),
		isGatewayController: true,
	},
}

func TestIsGatewayController(t *testing.T) {
	for _, tc := range isGatewayControllerTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.smcpSpec.IsGatewayController() != tc.isGatewayController {
				t.Errorf("exptected to get %t, got %t", tc.isGatewayController, tc.smcpSpec.IsGatewayController())
			}
		})
	}
}

func newSmcpSpec(specYaml string) *ControlPlaneSpec {
	smcp := &ControlPlaneSpec{}
	err := yaml.Unmarshal([]byte(specYaml), smcp)
	if err != nil {
		panic(fmt.Sprintf("unexpected error: %v", err))
	}
	return smcp
}
