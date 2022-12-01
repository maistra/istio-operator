package controlplane

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	maistrav1 "github.com/maistra/istio-operator/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/apis/maistra/v2"
	"github.com/maistra/istio-operator/controllers/common"
	"github.com/maistra/istio-operator/controllers/common/cni"
	"github.com/maistra/istio-operator/controllers/common/test"
	"github.com/maistra/istio-operator/controllers/common/test/assert"
)

func TestCalculateComponentReadinessMap(t *testing.T) {
	memberNamespace := "member-namespace"
	nonMemberNamespace := "non-member-namespace"
	memberNamespaces := []string{memberNamespace}

	testCases := []struct {
		name                  string
		gateways              *maistrav2.GatewaysConfig
		alwaysReadyComponents string
		objects               []runtime.Object
		expectedMap           map[string]bool
	}{
		{
			// deployment is ready and should appear in the readiness map with readiness set to true
			name: "deployment-in-cp-namespace-ready",
			objects: []runtime.Object{
				newDeployment("foo", controlPlaneNamespace, "component1", true),
			},
			expectedMap: map[string]bool{
				"component1": true,
			},
		},
		{
			// deployment is not ready and should appear in the readiness map with readiness set to false
			name: "deployment-in-cp-namespace-unready",
			objects: []runtime.Object{
				newDeployment("foo", controlPlaneNamespace, "component1", false),
			},
			expectedMap: map[string]bool{
				"component1": false,
			},
		},
		{
			// statefulset is ready and should appear in the readiness map with readiness set to true
			name: "statefulset-in-cp-namespace-ready",
			objects: []runtime.Object{
				newStatefulSet("foo", controlPlaneNamespace, controlPlaneNamespace, "component1", true),
			},
			expectedMap: map[string]bool{
				"component1": true,
			},
		},
		{
			// statefulset is not ready and should appear in the readiness map with readiness set to false
			name: "statefulset-in-cp-namespace-unready",
			objects: []runtime.Object{
				newStatefulSet("foo", controlPlaneNamespace, controlPlaneNamespace, "component1", false),
			},
			expectedMap: map[string]bool{
				"component1": false,
			},
		},
		{
			// daemonset is ready and should appear in the readiness map with readiness set to true
			name: "daemonset-in-cp-namespace-ready",
			objects: []runtime.Object{
				newDaemonSet("foo", controlPlaneNamespace, controlPlaneNamespace, "component1", true),
			},
			expectedMap: map[string]bool{
				"component1": true,
			},
		},
		{
			// daemonset is not ready and should appear in the readiness map with readiness set to false
			name: "daemonset-in-cp-namespace-unready",
			objects: []runtime.Object{
				newDaemonSet("foo", controlPlaneNamespace, controlPlaneNamespace, "component1", false),
			},
			expectedMap: map[string]bool{
				"component1": false,
			},
		},
		{
			// objects without the maistra.io/owner label should never appear in the map
			name: "objects-with-no-owner-label",
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: controlPlaneNamespace,
						Labels: map[string]string{
							// NOTE: no maistra.io/owner label
							common.KubernetesAppComponentKey: "component1",
						},
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: controlPlaneNamespace,
						Labels: map[string]string{
							common.KubernetesAppComponentKey: "component2",
						},
					},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: controlPlaneNamespace,
						Labels: map[string]string{
							common.KubernetesAppComponentKey: "component3",
						},
					},
				},
			},
			expectedMap: map[string]bool{},
		},
		{
			// when multiple objects belong to the same component, the component is ready only when all objects are ready
			name: "multiple-objects-same-component",
			objects: []runtime.Object{
				newDeployment("component1-foo-is-ready", controlPlaneNamespace, "component1", true),
				newDeployment("component1-bar-is-not-ready", controlPlaneNamespace, "component1", false),
				newDeployment("component2-foo-is-ready", controlPlaneNamespace, "component2", true),
				newDeployment("component2-bar-is-ready", controlPlaneNamespace, "component2", true),
				newDeployment("component3-foo-is-not-ready", controlPlaneNamespace, "component3", false),
				newDeployment("component3-bar-is-not-ready", controlPlaneNamespace, "component3", false),
			},
			expectedMap: map[string]bool{
				"component1": false,
				"component2": true,
				"component3": false,
			},
		},
		{
			// readiness map must contain entries for gateways deployed in member namespaces
			name: "gateways-in-member-namespaces",
			gateways: &maistrav2.GatewaysConfig{
				ClusterIngress: &maistrav2.ClusterIngressGatewayConfig{
					IngressGatewayConfig: maistrav2.IngressGatewayConfig{
						GatewayConfig: maistrav2.GatewayConfig{
							Namespace: memberNamespace,
						},
					},
				},
				ClusterEgress: &maistrav2.EgressGatewayConfig{
					GatewayConfig: maistrav2.GatewayConfig{
						Namespace: memberNamespace,
					},
				},
				IngressGateways: map[string]*maistrav2.IngressGatewayConfig{
					"additional-ingress": {
						GatewayConfig: maistrav2.GatewayConfig{
							Namespace: memberNamespace,
						},
					},
				},
				EgressGateways: map[string]*maistrav2.EgressGatewayConfig{
					"additional-egress": {
						GatewayConfig: maistrav2.GatewayConfig{
							Namespace: memberNamespace,
						},
					},
				},
			},
			objects: []runtime.Object{
				newDeployment("deploy1", memberNamespace, "component1", true),
				newDeployment("deploy2", memberNamespace, "component2", true),
				newDeployment("deploy3", memberNamespace, "component3", true),
				newDeployment("deploy4", memberNamespace, "component4", true),
			},
			expectedMap: map[string]bool{
				"component1": true,
				"component2": true,
				"component3": true,
				"component4": true,
			},
		},
		{
			// readiness map must not contain entries for gateways deployed in non-member namespaces (even if the gateway
			// namespaces in the SMCP point to actual deployments that are ready)
			name: "gateways-in-non-member-namespaces",
			gateways: &maistrav2.GatewaysConfig{
				ClusterIngress: &maistrav2.ClusterIngressGatewayConfig{
					IngressGatewayConfig: maistrav2.IngressGatewayConfig{
						GatewayConfig: maistrav2.GatewayConfig{
							Namespace: nonMemberNamespace,
						},
					},
				},
				ClusterEgress: &maistrav2.EgressGatewayConfig{
					GatewayConfig: maistrav2.GatewayConfig{
						Namespace: nonMemberNamespace,
					},
				},
				IngressGateways: map[string]*maistrav2.IngressGatewayConfig{
					"additional-ingress": {
						GatewayConfig: maistrav2.GatewayConfig{
							Namespace: nonMemberNamespace,
						},
					},
				},
				EgressGateways: map[string]*maistrav2.EgressGatewayConfig{
					"additional-egress": {
						GatewayConfig: maistrav2.GatewayConfig{
							Namespace: nonMemberNamespace,
						},
					},
				},
			},
			objects: []runtime.Object{
				newDeployment("deploy1", nonMemberNamespace, "component1", true),
				newDeployment("deploy2", nonMemberNamespace, "component2", true),
				newDeployment("deploy3", nonMemberNamespace, "component3", true),
				newDeployment("deploy4", nonMemberNamespace, "component4", true),
			},
			expectedMap: map[string]bool{},
		},
		{
			// components without objects marked as always ready should appear in the map as ready
			name:                  "always-ready-components",
			alwaysReadyComponents: "alwaysReady1,alwaysReady2",
			objects:               []runtime.Object{},
			expectedMap: map[string]bool{
				"alwaysReady1": true,
				"alwaysReady2": true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			smcp := newControlPlane()
			smcp.Spec.Gateways = tc.gateways
			if tc.alwaysReadyComponents != "" {
				smcp.Status.Annotations = map[string]string{
					statusAnnotationAlwaysReadyComponents: tc.alwaysReadyComponents,
				}
			}

			cl, tracker := test.CreateClient(tc.objects...)
			fakeEventRecorder := &record.FakeRecorder{}

			smmr := &maistrav1.ServiceMeshMemberRoll{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: controlPlaneNamespace,
				},
				Spec: maistrav1.ServiceMeshMemberRollSpec{
					Members: memberNamespaces,
				},
				Status: maistrav1.ServiceMeshMemberRollStatus{
					Members:           memberNamespaces,
					ConfiguredMembers: memberNamespaces,
				},
			}
			test.PanicOnError(tracker.Add(smmr))

			instanceReconciler := NewControlPlaneInstanceReconciler(
				common.ControllerResources{
					Client:            cl,
					Scheme:            scheme.Scheme,
					EventRecorder:     fakeEventRecorder,
					OperatorNamespace: "istio-operator",
				},
				smcp,
				cni.Config{Enabled: true}).(*controlPlaneInstanceReconciler)

			readinessMap, err := instanceReconciler.calculateComponentReadinessMap(ctx)
			if err != nil {
				t.Fatalf("Unexpected error in calculateComponentReadinessMap: %v", err)
			}

			assert.DeepEquals(readinessMap, tc.expectedMap, "Unexpected readiness map", t)
		})
	}
}

func newDeployment(name, namespace, component string, ready bool) *appsv1.Deployment {
	var readyReplicas int32
	if ready {
		readyReplicas = 1
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				common.OwnerKey:                  controlPlaneNamespace,
				common.KubernetesAppComponentKey: component,
			},
			Generation: 1,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:           1,
			ReadyReplicas:      readyReplicas,
			ObservedGeneration: 1,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func newStatefulSet(name, namespace, owner, component string, ready bool) *appsv1.StatefulSet {
	var readyReplicas int32
	if ready {
		readyReplicas = 1
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				common.OwnerKey:                  owner,
				common.KubernetesAppComponentKey: component,
			},
			Generation: 1,
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      1,
			ReadyReplicas: readyReplicas,
		},
	}
}

func newDaemonSet(name, namespace, owner, component string, ready bool) *appsv1.DaemonSet {
	var numberUnavailable int32
	if !ready {
		numberUnavailable = 1
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				common.OwnerKey:                  owner,
				common.KubernetesAppComponentKey: component,
			},
			Generation: 1,
		},
		Status: appsv1.DaemonSetStatus{
			NumberUnavailable: numberUnavailable,
		},
	}
}
