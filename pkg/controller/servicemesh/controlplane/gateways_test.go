package controlplane

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	. "github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func TestAdditionalIngressGatewayInstall(t *testing.T) {
	enabled := true
	disabled := false
	additionalGatewayName := "additional-gateway"
	appNamespace := "app-namespace"
	const gatewayLabel = "maistra.io/gateway"
	var testCases []IntegrationTestCase
	for _, v := range versions.TestedVersions {
		testCases = append(testCases,
			IntegrationTestCase{
				name: "no-namespace." + v.String(),
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
					Gateways: &v2.GatewaysConfig{
						ClusterIngress: &v2.ClusterIngressGatewayConfig{
							IngressGatewayConfig: v2.IngressGatewayConfig{
								GatewayConfig: v2.GatewayConfig{
									Enablement: v2.Enablement{
										Enabled: &enabled,
									},
								},
							},
						},
						IngressGateways: map[string]*v2.IngressGatewayConfig{
							additionalGatewayName: {
								GatewayConfig: v2.GatewayConfig{
									Enablement: v2.Enablement{
										Enabled: &enabled,
									},
								},
							},
						},
					},
					Version: v.String(),
				}),
				create: IntegrationTestValidation{
					Verifier: VerifyActions(
						Verify("create").On("deployments").
							Named("istio-ingressgateway").In(controlPlaneNamespace).
							Passes(ExpectedLabelGatewayCreate(gatewayLabel, "istio-ingressgateway."+controlPlaneNamespace)),
						Verify("create").On("deployments").
							Named(additionalGatewayName).In(controlPlaneNamespace).
							Passes(ExpectedLabelGatewayCreate(gatewayLabel, additionalGatewayName+"."+controlPlaneNamespace)),
					),
					Assertions: ActionAssertions{},
				},
				delete: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("delete").On("deployments").Named(additionalGatewayName).In(controlPlaneNamespace).IsSeen(),
					},
				},
			},
			IntegrationTestCase{
				name: "cp-namespace." + v.String(),
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
					Gateways: &v2.GatewaysConfig{
						IngressGateways: map[string]*v2.IngressGatewayConfig{
							additionalGatewayName: {
								GatewayConfig: v2.GatewayConfig{
									Enablement: v2.Enablement{
										Enabled: &enabled,
									},
									Namespace: controlPlaneNamespace,
								},
							},
						},
					},
					Version: v.String(),
				}),
				create: IntegrationTestValidation{
					Verifier: Verify("create").On("deployments").
						Named(additionalGatewayName).In(controlPlaneNamespace).
						Passes(ExpectedLabelGatewayCreate(gatewayLabel, additionalGatewayName+"."+controlPlaneNamespace)),
					Assertions: ActionAssertions{},
				},
				delete: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("delete").On("deployments").Named(additionalGatewayName).In(controlPlaneNamespace).IsSeen(),
					},
				},
			},
			IntegrationTestCase{
				name: "app-namespace." + v.String(),
				resources: []runtime.Object{
					&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: appNamespace}},
					&v1.ServiceMeshMemberRoll{
						ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: controlPlaneNamespace},
						Status: v1.ServiceMeshMemberRollStatus{
							ConfiguredMembers: []string{
								appNamespace,
							},
						},
					},
				},
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
					Gateways: &v2.GatewaysConfig{
						IngressGateways: map[string]*v2.IngressGatewayConfig{
							additionalGatewayName: {
								GatewayConfig: v2.GatewayConfig{
									Enablement: v2.Enablement{
										Enabled: &enabled,
									},
									Namespace: appNamespace,
								},
							},
						},
					},
					Version: v.String(),
				}),
				create: IntegrationTestValidation{
					Verifier: Verify("create").On("deployments").
						Named(additionalGatewayName).In(appNamespace).
						Passes(ExpectedExternalGatewayCreate),
					Assertions: ActionAssertions{},
				},
				delete: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("delete").On("deployments").Named(additionalGatewayName).In(appNamespace).IsSeen(),
					},
				},
			},
			IntegrationTestCase{
				name: "labels." + v.String(),
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
					Gateways: &v2.GatewaysConfig{
						IngressGateways: map[string]*v2.IngressGatewayConfig{
							additionalGatewayName: {
								GatewayConfig: v2.GatewayConfig{
									Enablement: v2.Enablement{
										Enabled: &enabled,
									},
									Service: v2.GatewayServiceConfig{
										Metadata: &v2.MetadataConfig{
											Labels: map[string]string{
												"test": "test",
											},
										},
									},
									Namespace: controlPlaneNamespace,
								},
							},
						},
					},
					Version: v.String(),
				}),
				create: IntegrationTestValidation{
					Verifier: VerifyActions(
						Verify("create").On("networkpolicies").Named("istio-ingressgateway").In(controlPlaneNamespace).Passes(
							ExpectedLabelMatchedByNetworkPolicy("istio", "ingressgateway"),
						),
						Verify("create").On("networkpolicies").Named(additionalGatewayName).In(controlPlaneNamespace).Passes(
							ExpectedLabelMatchedByNetworkPolicy("test", "test"),
						),
						Verify("create").On("deployments").Named(additionalGatewayName).In(controlPlaneNamespace).Passes(
							ExpectedLabelGatewayCreate("test", "test"),
							ExpectedLabelGatewayCreate(gatewayLabel, additionalGatewayName+"."+controlPlaneNamespace),
							ExpectedLabelGatewayCreate("app", additionalGatewayName),
						),
					),
					Assertions: ActionAssertions{},
				},
				delete: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("delete").On("deployments").Named(additionalGatewayName).In(controlPlaneNamespace).IsSeen(),
					},
				},
			},
			IntegrationTestCase{
				name: "cluster-ingress-route-enabled." + v.String(),
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
					Gateways: &v2.GatewaysConfig{
						ClusterIngress: &v2.ClusterIngressGatewayConfig{
							IngressGatewayConfig: v2.IngressGatewayConfig{
								RouteConfig: &v2.Enablement{
									Enabled: &enabled,
								},
							},
						},
					},
					Version: v.String(),
				}),
				create: IntegrationTestValidation{
					Verifier: VerifyActions(
						Verify("create").On("deployments").Named("istio-ingressgateway").In(controlPlaneNamespace).Passes(
							ExpectedLabelGatewayCreate(gatewayLabel, "istio-ingressgateway."+controlPlaneNamespace),
						),
					),
					Assertions: ActionAssertions{
						Assert("create").On("routes").Named("istio-ingressgateway").In(controlPlaneNamespace).IsSeen(),
					},
				},
				delete: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("delete").On("deployments").Named("istio-ingressgateway").In(controlPlaneNamespace).IsSeen(),
						Assert("delete").On("routes").Named("istio-ingressgateway").In(controlPlaneNamespace).IsSeen(),
					},
				},
			},
			IntegrationTestCase{
				name: "cluster-ingress-route-disabled." + v.String(),
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
					Gateways: &v2.GatewaysConfig{
						ClusterIngress: &v2.ClusterIngressGatewayConfig{
							IngressGatewayConfig: v2.IngressGatewayConfig{
								RouteConfig: &v2.Enablement{
									Enabled: &disabled,
								},
							},
						},
					},
					Version: v.String(),
				}),
				create: IntegrationTestValidation{
					Verifier: VerifyActions(
						Verify("create").On("deployments").Named("istio-ingressgateway").In(controlPlaneNamespace).Passes(
							ExpectedLabelGatewayCreate(gatewayLabel, "istio-ingressgateway."+controlPlaneNamespace),
						),
					),
					Assertions: ActionAssertions{
						Assert("create").On("routes").Named("istio-ingressgateway").In(controlPlaneNamespace).IsNotSeen(),
					},
				},
				delete: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("delete").On("deployments").Named("istio-ingressgateway").In(controlPlaneNamespace).IsSeen(),
						Assert("delete").On("routes").Named("istio-ingressgateway").In(controlPlaneNamespace).IsNotSeen(),
					},
				},
			},
			IntegrationTestCase{
				// creating a route should be enabled by default
				name: "cluster-ingress-route-undefined." + v.String(),
				smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
					Gateways: &v2.GatewaysConfig{
						ClusterIngress: &v2.ClusterIngressGatewayConfig{},
					},
					Version: v.String(),
				}),
				create: IntegrationTestValidation{
					Verifier: VerifyActions(
						Verify("create").On("deployments").Named("istio-ingressgateway").In(controlPlaneNamespace).Passes(
							ExpectedLabelGatewayCreate(gatewayLabel, "istio-ingressgateway."+controlPlaneNamespace),
						),
					),
					Assertions: ActionAssertions{
						Assert("create").On("routes").Named("istio-ingressgateway").In(controlPlaneNamespace).IsSeen(),
					},
				},
				delete: IntegrationTestValidation{
					Assertions: ActionAssertions{
						Assert("delete").On("deployments").Named("istio-ingressgateway").In(controlPlaneNamespace).IsSeen(),
						Assert("delete").On("routes").Named("istio-ingressgateway").In(controlPlaneNamespace).IsSeen(),
					},
				},
			})
	}
	RunSimpleInstallTests(t, testCases)
}

func ExpectedLabelGatewayCreate(labelName string, expectedValue string) func(action clienttesting.Action) error {
	return func(action clienttesting.Action) error {
		createAction := action.(clienttesting.CreateAction)
		obj := createAction.GetObject()
		gateway := obj.(*unstructured.Unstructured)
		if val, ok := common.GetLabel(gateway, labelName); ok {
			if val != expectedValue {
				return fmt.Errorf("expected %s label to be %s, got %s", labelName, expectedValue, val)
			}
		} else {
			return fmt.Errorf("gateway should have %s label defined", labelName)
		}
		return nil
	}
}

func ExpectedExternalGatewayCreate(action clienttesting.Action) error {
	createAction := action.(clienttesting.CreateAction)
	obj := createAction.GetObject()
	gateway := obj.(*unstructured.Unstructured)
	if len(gateway.GetOwnerReferences()) > 0 {
		return fmt.Errorf("external gateway should not have an owner reference")
	}
	return nil
}

func ExpectedLabelMatchedByNetworkPolicy(labelName string, expectedValue string) func(action clienttesting.Action) error {
	return func(action clienttesting.Action) error {
		createAction := action.(clienttesting.CreateAction)
		obj := createAction.GetObject()
		networkPolicy := obj.(*unstructured.Unstructured)
		if val, found, err := unstructured.NestedString(networkPolicy.Object, "spec", "podSelector", "matchLabels", labelName); err == nil {
			if !found || val != expectedValue {
				return fmt.Errorf("expected %s label to be matched against value %s, but didn't", labelName, expectedValue)
			}
		} else if err != nil {
			return err
		}

		return nil
	}
}
