package controlplane

import (
	"testing"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	. "github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

// Number of network polocies in our charts
const numberOfNetworkPolicies = 8

func TestNetworkPolicy(t *testing.T) {
	var testCases []IntegrationTestCase
	for _, v := range versions.TestedVersions {
		testCases = append(testCases,
			IntegrationTestCase{
				name: "np.enabled." + v.String(),
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
					Version: v.String(),
					Security: &v2.SecurityConfig{
						ManageNetworkPolicy: ptrTrue,
					},
				}),
				create: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("create").On("networkpolicies").In(controlPlaneNamespace).SeenCountIs(numberOfNetworkPolicies),
					},
				},
				delete: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("delete").On("networkpolicies").In(controlPlaneNamespace).SeenCountIs(numberOfNetworkPolicies),
					},
				},
			},
			IntegrationTestCase{
				name: "np.missing." + v.String(), // Same behavior as the above case, since the default value is `enabled`.
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
					Version: v.String(),
				}),
				create: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("create").On("networkpolicies").In(controlPlaneNamespace).SeenCountIs(numberOfNetworkPolicies),
					},
				},
				delete: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("delete").On("networkpolicies").In(controlPlaneNamespace).SeenCountIs(numberOfNetworkPolicies),
					},
				},
			},
			IntegrationTestCase{
				name: "np.disabled." + v.String(),
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
					Version: v.String(),
					Security: &v2.SecurityConfig{
						ManageNetworkPolicy: ptrFalse,
					},
				}),
				create: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("create").On("networkpolicies").In(controlPlaneNamespace).IsNotSeen(),
					},
				},
				delete: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("delete").On("networkpolicies").In(controlPlaneNamespace).IsNotSeen(),
					},
				},
			},
		)
	}
	RunSimpleInstallTests(t, testCases)
}
