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
	testCases := []IntegrationTestCase{
		{
			name: "np.enabled",
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				Version: versions.V2_1.String(),
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
		{
			name: "np.missing", // Same behavior as the above case, since the default value is `enabled`.
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				Version: versions.V2_1.String(),
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
		{
			name: "np.disabled",
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				Version: versions.V2_1.String(),
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
	}
	RunSimpleInstallTest(t, testCases)
}
