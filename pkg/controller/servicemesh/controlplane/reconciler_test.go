package controlplane

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	falseVal = false
	ptrFalse = &falseVal
	trueVal  = true
	ptrTrue  = &trueVal
)

func TestInstallationErrorDoesNotUpdateLastTransitionTimeWhenNoStateTransitionOccurs(t *testing.T) {
	controlPlane := newControlPlane()
	controlPlane.Spec.Profiles = []string{"maistra"}
	controlPlane.Status.SetCondition(status.Condition{
		Type:               status.ConditionTypeReconciled,
		Status:             status.ConditionStatusFalse,
		Reason:             "",
		Message:            "",
		LastTransitionTime: oneMinuteAgo,
	})

	cl, tracker, r := newReconcilerTestFixture(controlPlane)

	// make installation fail
	tracker.AddReactor("create", "deployments", test.ClientFails())

	// run initial reconcile to update the SMCP status
	assertInstanceReconcilerFails(r, t)

	// run reconcile again to work around the problem where the condition reason "InstallError" gets changed to "ReconcileError" in 2nd reconcile
	assertInstanceReconcilerFails(r, t)

	// remember the SMCP status at this point
	updatedControlPlane := &maistrav2.ServiceMeshControlPlane{}
	test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(controlPlane), updatedControlPlane))
	initialStatus := updatedControlPlane.Status.DeepCopy()

	// the resolution of lastTransitionTime is one second, so we need to wait at least one second before
	// performing another reconciliation to ensure that if the lastTransitionTime field is reset, the new
	// value will actually be different from the previous one
	time.Sleep(1 * time.Second)

	// run reconcile again to check if the status is still the same
	assertInstanceReconcilerFails(r, t)

	test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(controlPlane), updatedControlPlane))
	newStatus := &updatedControlPlane.Status

	marshal, _ := json.Marshal(initialStatus)
	fmt.Println(string(marshal))
	marshal, _ = json.Marshal(newStatus)
	fmt.Println(string(marshal))

	assert.DeepEquals(newStatus, initialStatus, "didn't expect SMCP status to be updated", t)
}

type customSetup func(client.Client, *test.EnhancedTracker)

func TestManifestValidation(t *testing.T) {
	enabled := true
	testCases := []struct {
		name          string
		controlPlane  *maistrav2.ServiceMeshControlPlane
		memberRoll    *maistrav1.ServiceMeshMemberRoll
		setupFn       customSetup
		errorMessages map[versions.Version]string // expected error message for each version
		errorMessage  string                      // common error message (expected for all versions)
	}{
		{
			name: "error getting smmr",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: newControlPlane().ObjectMeta,
				Spec: maistrav2.ControlPlaneSpec{
					Profiles: []string{"maistra"},
					Gateways: &maistrav2.GatewaysConfig{
						IngressGateways: map[string]*maistrav2.IngressGatewayConfig{
							"another-ingress": {
								GatewayConfig: maistrav2.GatewayConfig{
									Enablement: maistrav2.Enablement{
										Enabled: &enabled,
									},
									Namespace: "somewhere",
									Service: maistrav2.GatewayServiceConfig{
										Metadata: &maistrav2.MetadataConfig{
											Labels: map[string]string{
												"app": "istio",
											},
										},
									},
								},
							},
						},
					},
				},
				Status: maistrav2.ControlPlaneStatus{},
			},
			memberRoll: &maistrav1.ServiceMeshMemberRoll{},
			setupFn: func(cl client.Client, tracker *test.EnhancedTracker) {
				tracker.AddReactor("get", "servicemeshmemberrolls", test.ClientFails())
			},
			errorMessage: "error on get",
		},
		{
			name: "gateways outside of mesh",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: newControlPlane().ObjectMeta,
				Spec: maistrav2.ControlPlaneSpec{
					Profiles: []string{"maistra"},
					Gateways: &maistrav2.GatewaysConfig{
						IngressGateways: map[string]*maistrav2.IngressGatewayConfig{
							"another-ingress": {
								GatewayConfig: maistrav2.GatewayConfig{
									Enablement: maistrav2.Enablement{
										Enabled: &enabled,
									},
									Namespace: "b",
									Service: maistrav2.GatewayServiceConfig{
										Metadata: &maistrav2.MetadataConfig{
											Labels: map[string]string{
												"app": "istio",
											},
										},
									},
								},
							},
						},
						EgressGateways: map[string]*maistrav2.EgressGatewayConfig{
							"another-egress": {
								GatewayConfig: maistrav2.GatewayConfig{
									Enablement: maistrav2.Enablement{
										Enabled: &enabled,
									},
									Namespace: "d",
									Service: maistrav2.GatewayServiceConfig{
										Metadata: &maistrav2.MetadataConfig{
											Labels: map[string]string{
												"app": "istio",
											},
										},
									},
								},
							},
						},
					},
				},
				Status: maistrav2.ControlPlaneStatus{},
			},
			memberRoll: &maistrav1.ServiceMeshMemberRoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: controlPlaneNamespace,
				},
				Spec: maistrav1.ServiceMeshMemberRollSpec{
					Members: []string{
						"a",
					},
				},
				Status: maistrav1.ServiceMeshMemberRollStatus{
					ConfiguredMembers: []string{
						"a",
					},
				},
			},
			errorMessages: map[versions.Version]string{
				versions.V1_0: "namespace of manifest b/another-ingress not in mesh",
				versions.V1_1: "namespace of manifest b/another-ingress not in mesh",
				versions.V2_0: "Spec is invalid: error: [namespace \"b\" for ingress gateway \"another-ingress\" must be part of the mesh, namespace \"d\" for ingress gateway \"another-egress\" must be part of the mesh]",
			},
		},
		{
			name: "valid namespaces",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: newControlPlane().ObjectMeta,
				Spec: maistrav2.ControlPlaneSpec{
					Profiles: []string{"maistra"},
					Gateways: &maistrav2.GatewaysConfig{
						IngressGateways: map[string]*maistrav2.IngressGatewayConfig{
							"another-ingress": {
								GatewayConfig: maistrav2.GatewayConfig{
									Enablement: maistrav2.Enablement{
										Enabled: &enabled,
									},
									Namespace: "a",
									Service: maistrav2.GatewayServiceConfig{
										Metadata: &maistrav2.MetadataConfig{
											Labels: map[string]string{
												"app": "istio",
											},
										},
									},
								},
							},
						},
						EgressGateways: map[string]*maistrav2.EgressGatewayConfig{
							"another-egress": {
								GatewayConfig: maistrav2.GatewayConfig{
									Enablement: maistrav2.Enablement{
										Enabled: &enabled,
									},
									Namespace: "c",
									Service: maistrav2.GatewayServiceConfig{
										Metadata: &maistrav2.MetadataConfig{
											Labels: map[string]string{
												"app": "istio",
											},
										},
									},
								},
							},
						},
					},
				},
				Status: maistrav2.ControlPlaneStatus{},
			},
			memberRoll: &maistrav1.ServiceMeshMemberRoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: controlPlaneNamespace,
				},
				Spec: maistrav1.ServiceMeshMemberRollSpec{
					Members: []string{
						"a",
						"c",
					},
				},
				Status: maistrav1.ServiceMeshMemberRollStatus{
					ConfiguredMembers: []string{
						"a",
						"c",
					},
				},
			},
		},
	}

	operatorNamespace := "istio-operator"
	InitializeGlobals(operatorNamespace)()

	for _, tc := range testCases {
		name := tc.name
		for _, version := range versions.GetSupportedVersions() {
			tc.controlPlane.Spec.Version = version.String()
			tc.name = name + "." + tc.controlPlane.Spec.Version
			t.Run(tc.name, func(t *testing.T) {
				tc.controlPlane.Status.SetCondition(status.Condition{
					Type:               status.ConditionTypeReconciled,
					Status:             status.ConditionStatusFalse,
					Reason:             "",
					Message:            "",
					LastTransitionTime: oneMinuteAgo,
				})

				namespace := &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: tc.controlPlane.Namespace},
				}

				cl, tracker := test.CreateClient(tc.controlPlane, tc.memberRoll, namespace)
				fakeEventRecorder := &record.FakeRecorder{}

				r := NewControlPlaneInstanceReconciler(
					common.ControllerResources{
						Client:            cl,
						Scheme:            tracker.Scheme,
						EventRecorder:     fakeEventRecorder,
						OperatorNamespace: operatorNamespace,
					},
					tc.controlPlane,
					cni.Config{Enabled: true})

				if tc.setupFn != nil {
					tc.setupFn(cl, tracker)
				}
				// run initial reconcile to update the SMCP status
				_, err := r.Reconcile(ctx)

				expectedErrorMessage := tc.errorMessages[version]
				if expectedErrorMessage == "" {
					expectedErrorMessage = tc.errorMessage
				}
				if expectedErrorMessage != "" {
					if err == nil {
						t.Fatal(tc.name, "-", "Expected reconcile to fail, but it didn't")
					}

					updatedControlPlane := &maistrav2.ServiceMeshControlPlane{}
					test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(&tc.controlPlane.ObjectMeta), updatedControlPlane))
					newStatus := &updatedControlPlane.Status

					assert.True(strings.Contains(newStatus.GetCondition(status.ConditionTypeReconciled).Message, expectedErrorMessage), "Expected reconciliation error:\n    "+expectedErrorMessage+"\nbut got:\n    "+newStatus.GetCondition(status.ConditionTypeReconciled).Message, t)
				} else {
					if err != nil {
						t.Fatal(tc.name, "-", "Expected no errors, but got error: ", err)
					}
				}
			})
		}

	}
}

func assertInstanceReconcilerFails(r ControlPlaneInstanceReconciler, t *testing.T) {
	_, err := r.Reconcile(ctx)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}
}

func TestParallelInstallationOfCharts(t *testing.T) {
	var testCases = []struct {
		name                       string
		reactorsForFirstReconcile  []clienttesting.Reactor
		expectFirstReconcileToFail bool
		base                       map[string]interface{}
		input                      map[string]interface{}
		expectedResult             map[string]interface{}
	}{
		{
			name: "normal-case",
		},
		{
			name: "process-component-manifests-fails",
			reactorsForFirstReconcile: []clienttesting.Reactor{
				&clienttesting.SimpleReactor{Verb: "create", Resource: "deployments", Reaction: func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					create := action.(clienttesting.CreateAction)
					deploy := create.GetObject().(*unstructured.Unstructured)
					if deploy.GetName() == "istio-pilot" {
						return test.ClientFails()(action)
					}
					return false, nil, nil
				}},
			},
			expectFirstReconcileToFail: true,
		},
		{
			name: "calculate-readiness-fails",
			reactorsForFirstReconcile: []clienttesting.Reactor{
				&clienttesting.SimpleReactor{Verb: "list", Resource: "deployments", Reaction: test.ClientFails()},
			},
			expectFirstReconcileToFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			disabled := false
			enabled := true
			smcp := newControlPlane()
			smcp.Spec = maistrav2.ControlPlaneSpec{
				Profiles: []string{"maistra"},
				Version:  versions.V1_1.String(),
				Policy: &maistrav2.PolicyConfig{
					Type: maistrav2.PolicyTypeNone,
				},
				Telemetry: &maistrav2.TelemetryConfig{
					Type: maistrav2.TelemetryTypeNone,
				},
				Gateways: &maistrav2.GatewaysConfig{
					ClusterIngress: &maistrav2.ClusterIngressGatewayConfig{
						IngressGatewayConfig: maistrav2.IngressGatewayConfig{
							GatewayConfig: maistrav2.GatewayConfig{
								Enablement: maistrav2.Enablement{Enabled: &disabled},
							},
						},
					},
					ClusterEgress: &maistrav2.EgressGatewayConfig{
						GatewayConfig: maistrav2.GatewayConfig{
							Enablement: maistrav2.Enablement{Enabled: &disabled},
						},
					},
				},
				Tracing: &maistrav2.TracingConfig{Type: maistrav2.TracerTypeNone},
				Addons: &maistrav2.AddonsConfig{
					Prometheus: &maistrav2.PrometheusAddonConfig{
						Enablement: maistrav2.Enablement{Enabled: &disabled},
					},
					Grafana: &maistrav2.GrafanaAddonConfig{
						Enablement: maistrav2.Enablement{Enabled: &enabled},
						Install:    &maistrav2.GrafanaInstallConfig{},
					},
					Kiali: &maistrav2.KialiAddonConfig{
						Enablement: maistrav2.Enablement{Enabled: &disabled},
					},
				},
			}

			cl, tracker, r := newReconcilerTestFixture(smcp)

			// run initial reconcile to initialize reconcile status
			assertInstanceReconcilerSucceeds(r, t)
			assertInstanceReconcilerSucceeds(r, t)
			securityDeployment := assertDeploymentExists(cl, "istio-citadel", t)
			markDeploymentAvailable(cl, securityDeployment)
			assertInstanceReconcilerSucceeds(r, t)
			galleyDeployment := assertDeploymentExists(cl, "istio-galley", t)
			markDeploymentAvailable(cl, galleyDeployment)

			if tc.reactorsForFirstReconcile != nil {
				tracker.AddReaction(tc.reactorsForFirstReconcile...)

				if tc.expectFirstReconcileToFail {
					// first reconcile should fail
					_, err := r.Reconcile(ctx)
					assert.Failure(err, "Reconcile", t)
				} else {
					assertInstanceReconcilerSucceeds(r, t)
				}

				// we remove any reactors that cause failure
				tracker.RemoveReaction(tc.reactorsForFirstReconcile...)
			}

			// this reconcile must succeed
			assertInstanceReconcilerSucceeds(r, t)

			// check that both galley and citadel deployments have been created
			pilotDeployment := assertDeploymentExists(cl, "istio-pilot", t)
			sidecarInjectorWebhookDeployment := assertDeploymentExists(cl, "istio-sidecar-injector", t)

			// check if reconciledCondition indicates installation is paused and both galley and security are mentioned
			assertReconciledConditionMatches(cl, smcp, status.ConditionReasonPausingInstall, "[pilot sidecarInjectorWebhook]", t)

			markDeploymentAvailable(cl, pilotDeployment)

			// run reconcile again to see if the Reconciled condition is updated
			assertInstanceReconcilerSucceeds(r, t)
			assertReconciledConditionMatches(cl, smcp, status.ConditionReasonPausingInstall, "[sidecarInjectorWebhook]", t)

			markDeploymentAvailable(cl, sidecarInjectorWebhookDeployment)

			// run reconcile again to see if the Reconciled condition is updated
			assertInstanceReconcilerSucceeds(r, t)
			assertDeploymentExists(cl, "grafana", t)
			assertReconciledConditionMatches(cl, smcp, status.ConditionReasonPausingInstall, "[grafana]", t)
		})
	}
}

func TestValidation(t *testing.T) {
	var testCases = []struct {
		name        string
		spec        maistrav2.ControlPlaneSpec
		expectValid bool
	}{
		{
			name: "kiali-enabled-prometheus-disabled",
			spec: maistrav2.ControlPlaneSpec{
				Version:  versions.V2_0.String(),
				Profiles: []string{"maistra"},
				Addons: &maistrav2.AddonsConfig{
					Kiali: &maistrav2.KialiAddonConfig{
						Enablement: maistrav2.Enablement{Enabled: ptrTrue},
					},
					Prometheus: &maistrav2.PrometheusAddonConfig{
						Enablement: maistrav2.Enablement{Enabled: ptrFalse},
					},
				},
			},
			expectValid: false,
		},
		{
			name: "kiali-enabled-prometheus-enabled",
			spec: maistrav2.ControlPlaneSpec{
				Version:  versions.V2_0.String(),
				Profiles: []string{"maistra"},
				Addons: &maistrav2.AddonsConfig{
					Kiali: &maistrav2.KialiAddonConfig{
						Enablement: maistrav2.Enablement{Enabled: ptrTrue},
					},
					Prometheus: &maistrav2.PrometheusAddonConfig{
						Enablement: maistrav2.Enablement{Enabled: ptrTrue},
					},
				},
			},
			expectValid: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			smcp := newControlPlane()
			smcp.Spec = tc.spec

			cl, _, r := newReconcilerTestFixture(smcp)

			// run initial reconcile to initialize reconcile status
			assertInstanceReconcilerSucceeds(r, t)

			// run reconcile to apply profiles & validate SMCP
			if tc.expectValid {
				assertInstanceReconcilerSucceeds(r, t)
			} else {
				assertInstanceReconcilerFails(r, t)

				// check if Reconciled condition reason is set to ValidationError
				test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(smcp), smcp))
				condition := smcp.Status.GetCondition(status.ConditionTypeReconciled)
				assert.Equals(condition.Reason, status.ConditionReasonValidationError, "Unexpected Reason in Reconciled condition", t)
			}
		})
	}
}

func assertDeploymentExists(cl client.Client, name string, t *testing.T) *appsv1.Deployment {
	t.Helper()
	deploy := &appsv1.Deployment{}
	err := cl.Get(ctx, types.NamespacedName{Namespace: controlPlaneNamespace, Name: name}, deploy)
	if errors.IsNotFound(err) {
		t.Fatalf("Expected Deployment %q to exist, but it doesn't", name)
	} else if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	return deploy
}

func newReconcilerTestFixture(smcp *maistrav2.ServiceMeshControlPlane) (client.Client, *test.EnhancedTracker, ControlPlaneInstanceReconciler) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNamespace},
	}

	operatorNamespace := "istio-operator"
	InitializeGlobals(operatorNamespace)()

	cl, tracker := test.CreateClient(smcp, namespace)
	fakeEventRecorder := &record.FakeRecorder{}

	r := NewControlPlaneInstanceReconciler(
		common.ControllerResources{
			Client:            cl,
			Scheme:            tracker.Scheme,
			EventRecorder:     fakeEventRecorder,
			OperatorNamespace: operatorNamespace,
		},
		smcp,
		cni.Config{Enabled: true})

	return cl, tracker, r
}

func assertInstanceReconcilerSucceeds(r ControlPlaneInstanceReconciler, t *testing.T) {
	t.Helper()
	_, err := r.Reconcile(ctx)
	assert.Success(err, "Reconcile", t)
}

func assertReconciledConditionMatches(cl client.Client, smcp *maistrav2.ServiceMeshControlPlane, reason status.ConditionReason, messageSubstring string, t *testing.T) {
	t.Helper()
	test.PanicOnError(cl.Get(ctx, common.ToNamespacedName(smcp), smcp))
	reconciledCondition := smcp.Status.GetCondition(status.ConditionTypeReconciled)
	assert.Equals(reconciledCondition.Reason, reason, "Unexpected reconciledCondition.Reason", t)
	assert.True(
		strings.Contains(reconciledCondition.Message, messageSubstring),
		fmt.Sprintf("Expected to find %q in reconciledCondition.Message, but was: %s", messageSubstring, reconciledCondition.Message), t)
}

func markDeploymentAvailable(cl client.Client, deployment *appsv1.Deployment) {
	deployment.Status.Conditions = []appsv1.DeploymentCondition{
		{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionTrue,
		},
	}
	test.PanicOnError(cl.Update(ctx, deployment))
}

func newTestReconciler() *controlPlaneInstanceReconciler {
	reconciler := NewControlPlaneInstanceReconciler(
		common.ControllerResources{},
		&maistrav2.ServiceMeshControlPlane{},
		cni.Config{Enabled: true})
	return reconciler.(*controlPlaneInstanceReconciler)
}
