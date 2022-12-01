package versions

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	maistrav2 "github.com/maistra/istio-operator/apis/maistra/v2"
)

const controlPlaneNamespace = "cp-namespace"

func TestValidateGateways(t *testing.T) {
	testCases := []struct {
		name        string
		gateways    *maistrav2.GatewaysConfig
		expectError bool
	}{
		{
			name:        "no-gateways",
			gateways:    nil,
			expectError: false,
		},
		{
			name: "additional-ingress-reserved-name",
			gateways: &maistrav2.GatewaysConfig{
				IngressGateways: map[string]*maistrav2.IngressGatewayConfig{
					"istio-ingressgateway": newIngressGatewayConfig(controlPlaneNamespace, nil),
				},
			},
			expectError: true,
		},

		{
			name: "additional-egress-reserved-name",
			gateways: &maistrav2.GatewaysConfig{
				EgressGateways: map[string]*maistrav2.EgressGatewayConfig{
					"istio-egressgateway": newEgressGatewayConfig(controlPlaneNamespace, nil),
				},
			},
			expectError: true,
		},
		{
			name: "duplicate-gateway-name",
			gateways: &maistrav2.GatewaysConfig{
				IngressGateways: map[string]*maistrav2.IngressGatewayConfig{
					"duplicate-name": newIngressGatewayConfig(controlPlaneNamespace, nil),
				},
				EgressGateways: map[string]*maistrav2.EgressGatewayConfig{
					"duplicate-name": newEgressGatewayConfig(controlPlaneNamespace, nil),
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			smcp := &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "minimal",
					Namespace: controlPlaneNamespace,
				},
				Spec: maistrav2.ControlPlaneSpec{
					Gateways: tc.gateways,
				},
			}

			allErrors := validateGatewaysInternal(&smcp.ObjectMeta, &smcp.Spec, []error{})
			if tc.expectError {
				if len(allErrors) == 0 {
					t.Fatal("Expected errors, but none were returned")
				}
			} else {
				if len(allErrors) > 0 {
					t.Fatalf("Unexpected errors: %v", allErrors)
				}
			}
		})
	}
}

func newEgressGatewayConfig(namespace string, enabled *bool) *maistrav2.EgressGatewayConfig {
	return &maistrav2.EgressGatewayConfig{
		GatewayConfig: *newGatewayConfig(namespace, enabled),
	}
}

func newIngressGatewayConfig(namespace string, enabled *bool) *maistrav2.IngressGatewayConfig {
	return &maistrav2.IngressGatewayConfig{
		GatewayConfig: *newGatewayConfig(namespace, enabled),
	}
}

func newGatewayConfig(namespace string, enabled *bool) *maistrav2.GatewayConfig {
	return &maistrav2.GatewayConfig{
		Namespace: namespace,
		Enablement: maistrav2.Enablement{
			Enabled: enabled,
		},
	}
}
