// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package istiorevision

import (
	"context"
	"strings"
	"testing"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/test"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"istio.io/istio/pkg/ptr"
)

const operatorNamespace = "sail-operator"

func TestDeriveState(t *testing.T) {
	testCases := []struct {
		name                string
		reconciledCondition v1alpha1.IstioRevisionCondition
		readyCondition      v1alpha1.IstioRevisionCondition
		expectedState       v1alpha1.IstioRevisionConditionReason
	}{
		{
			name:                "healthy",
			reconciledCondition: newCondition(v1alpha1.IstioRevisionConditionTypeReconciled, true, ""),
			readyCondition:      newCondition(v1alpha1.IstioRevisionConditionTypeReady, true, ""),
			expectedState:       v1alpha1.IstioRevisionConditionReasonHealthy,
		},
		{
			name:                "not reconciled",
			reconciledCondition: newCondition(v1alpha1.IstioRevisionConditionTypeReconciled, false, v1alpha1.IstioRevisionConditionReasonReconcileError),
			readyCondition:      newCondition(v1alpha1.IstioRevisionConditionTypeReady, true, ""),
			expectedState:       v1alpha1.IstioRevisionConditionReasonReconcileError,
		},
		{
			name:                "not ready",
			reconciledCondition: newCondition(v1alpha1.IstioRevisionConditionTypeReconciled, true, ""),
			readyCondition:      newCondition(v1alpha1.IstioRevisionConditionTypeReady, false, v1alpha1.IstioRevisionConditionReasonIstiodNotReady),
			expectedState:       v1alpha1.IstioRevisionConditionReasonIstiodNotReady,
		},
		{
			name:                "not reconciled nor ready",
			reconciledCondition: newCondition(v1alpha1.IstioRevisionConditionTypeReconciled, false, v1alpha1.IstioRevisionConditionReasonReconcileError),
			readyCondition:      newCondition(v1alpha1.IstioRevisionConditionTypeReady, false, v1alpha1.IstioRevisionConditionReasonIstiodNotReady),
			expectedState:       v1alpha1.IstioRevisionConditionReasonReconcileError, // reconcile reason takes precedence over ready reason
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := deriveState(tc.reconciledCondition, tc.readyCondition)
			if result != tc.expectedState {
				t.Errorf("Expected reason %s, but got %s", tc.expectedState, result)
			}
		})
	}
}

func newCondition(conditionType v1alpha1.IstioRevisionConditionType,
	status bool,
	reason v1alpha1.IstioRevisionConditionReason,
) v1alpha1.IstioRevisionCondition {
	st := metav1.ConditionFalse
	if status {
		st = metav1.ConditionTrue
	}
	return v1alpha1.IstioRevisionCondition{
		Type:   conditionType,
		Status: st,
		Reason: reason,
	}
}

func TestDetermineReadyCondition(t *testing.T) {
	testCases := []struct {
		name          string
		cniEnabled    bool
		values        *v1alpha1.Values
		clientObjects []client.Object
		expected      v1alpha1.IstioRevisionCondition
	}{
		{
			name:   "Istiod ready",
			values: nil,
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
			},
			expected: v1alpha1.IstioRevisionCondition{
				Type:   v1alpha1.IstioRevisionConditionTypeReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name:   "Istiod not ready",
			values: nil,
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     1,
						AvailableReplicas: 1,
					},
				},
			},
			expected: v1alpha1.IstioRevisionCondition{
				Type:    v1alpha1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioRevisionConditionReasonIstiodNotReady,
				Message: "not all istiod pods are ready",
			},
		},
		{
			name:   "Istiod scaled to zero",
			values: nil,
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          0,
						ReadyReplicas:     0,
						AvailableReplicas: 0,
					},
				},
			},
			expected: v1alpha1.IstioRevisionCondition{
				Type:    v1alpha1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioRevisionConditionReasonIstiodNotReady,
				Message: "istiod Deployment is scaled to zero replicas",
			},
		},
		{
			name:          "Istiod not found",
			values:        nil,
			clientObjects: []client.Object{},
			expected: v1alpha1.IstioRevisionCondition{
				Type:    v1alpha1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioRevisionConditionReasonIstiodNotReady,
				Message: "istiod Deployment not found",
			},
		},
		{
			name: "Istiod and CNI ready",
			values: &v1alpha1.Values{
				IstioCni: &v1alpha1.CNIUsageConfig{
					Enabled: ptr.Of(true),
				},
			},
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: operatorNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 3,
						NumberReady:            3,
					},
				},
			},
			expected: v1alpha1.IstioRevisionCondition{
				Type:   v1alpha1.IstioRevisionConditionTypeReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "CNI not ready",
			values: &v1alpha1.Values{
				IstioCni: &v1alpha1.CNIUsageConfig{
					Enabled: ptr.Of(true),
				},
			},
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: operatorNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 1,
						NumberReady:            0,
					},
				},
			},
			expected: v1alpha1.IstioRevisionCondition{
				Type:    v1alpha1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioRevisionConditionReasonCNINotReady,
				Message: "not all istio-cni-node pods are ready",
			},
		},
		{
			name: "CNI pods not scheduled",
			values: &v1alpha1.Values{
				IstioCni: &v1alpha1.CNIUsageConfig{
					Enabled: ptr.Of(true),
				},
			},
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: operatorNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 0,
						NumberReady:            0,
					},
				},
			},
			expected: v1alpha1.IstioRevisionCondition{
				Type:    v1alpha1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioRevisionConditionReasonCNINotReady,
				Message: "no istio-cni-node pods are currently scheduled",
			},
		},
		{
			name: "CNI not found",
			values: &v1alpha1.Values{
				IstioCni: &v1alpha1.CNIUsageConfig{
					Enabled: ptr.Of(true),
				},
			},
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
			},
			expected: v1alpha1.IstioRevisionCondition{
				Type:    v1alpha1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioRevisionConditionReasonCNINotReady,
				Message: "istio-cni-node DaemonSet not found",
			},
		},
		{
			name: "Non-default revision",
			values: &v1alpha1.Values{
				Revision: "my-revision",
			},
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod-my-revision",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
			},
			expected: v1alpha1.IstioRevisionCondition{
				Type:   v1alpha1.IstioRevisionConditionTypeReady,
				Status: metav1.ConditionTrue,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tt.clientObjects...).Build()

			r := NewIstioRevisionReconciler(cl, scheme.Scheme, nil, operatorNamespace)

			rev := &v1alpha1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-istio",
				},
				Spec: v1alpha1.IstioRevisionSpec{
					Namespace: "istio-system",
					Values:    tt.values,
				},
			}

			result := r.determineReadyCondition(context.TODO(), rev)
			if result.Type != tt.expected.Type || result.Status != tt.expected.Status ||
				result.Reason != tt.expected.Reason || result.Message != tt.expected.Message {
				t.Errorf("Unexpected result.\nGot:\n    %+v\nexpected:\n    %+v", result, tt.expected)
			}
		})
	}
}

func TestDetermineInUseCondition(t *testing.T) {
	test.SetupScheme()

	testCases := []struct {
		podLabels           map[string]string
		podAnnotations      map[string]string
		nsLabels            map[string]string
		enableAllNamespaces bool
		matchesRevision     string
	}{
		// no labels on namespace or pod
		{
			nsLabels:        map[string]string{},
			podLabels:       map[string]string{},
			matchesRevision: "",
		},

		// namespace labels only
		{
			nsLabels:        map[string]string{"istio-injection": "enabled"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			matchesRevision: "default",
		},

		// pod labels only
		{
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},
		{
			podLabels:       map[string]string{"sidecar.istio.io/inject": "true"},
			matchesRevision: "default",
		},
		{
			podLabels:       map[string]string{"sidecar.istio.io/inject": "true", "istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},

		// ns and pod labels
		{
			nsLabels:        map[string]string{"istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "my-rev",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},

		// special case: when Values.sidecarInjectorWebhook.enableNamespacesByDefault is true, all pods should match the default revision
		// unless they are in one of the system namespaces ("kube-system","kube-public","kube-node-lease","local-path-storage")
		{
			enableAllNamespaces: true,
			matchesRevision:     "default",
		},
	}

	for _, revName := range []string{"default", "my-rev"} {
		for _, tc := range testCases {
			nameBuilder := strings.Builder{}
			nameBuilder.WriteString(revName + ":")
			if len(tc.nsLabels) == 0 && len(tc.podLabels) == 0 {
				nameBuilder.WriteString("no labels")
			}
			if len(tc.nsLabels) > 0 {
				nameBuilder.WriteString("NS:")
				for k, v := range tc.nsLabels {
					nameBuilder.WriteString(k + ":" + v + ",")
				}
			}
			if len(tc.podLabels) > 0 {
				nameBuilder.WriteString("POD:")
				for k, v := range tc.podLabels {
					nameBuilder.WriteString(k + ":" + v + ",")
				}
			}
			name := strings.TrimSuffix(nameBuilder.String(), ",")

			t.Run(name, func(t *testing.T) {
				rev := &v1alpha1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: revName,
					},
					Spec: v1alpha1.IstioRevisionSpec{
						Namespace: "istio-system",
						Version:   "my-version",
					},
				}
				if tc.enableAllNamespaces {
					rev.Spec.Values = &v1alpha1.Values{
						SidecarInjectorWebhook: &v1alpha1.SidecarInjectorConfig{
							EnableNamespacesByDefault: ptr.Of(true),
						},
					}
				}

				namespace := "bookinfo"
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   namespace,
						Labels: tc.nsLabels,
					},
				}

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "some-pod",
						Namespace:   namespace,
						Labels:      tc.podLabels,
						Annotations: tc.podAnnotations,
					},
				}

				cl := fake.NewClientBuilder().
					WithScheme(scheme.Scheme).
					WithObjects(rev, ns, pod).
					Build()

				r := NewIstioRevisionReconciler(cl, scheme.Scheme, nil, operatorNamespace)

				result, err := r.determineInUseCondition(context.TODO(), rev)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if result.Type != v1alpha1.IstioRevisionConditionTypeInUse {
					t.Errorf("unexpected condition type: %v", result.Type)
				}

				expectedStatus := metav1.ConditionFalse
				if revName == tc.matchesRevision {
					expectedStatus = metav1.ConditionTrue
				}

				if result.Status != expectedStatus {
					t.Errorf("Unexpected status. Revision %s reports being in use, but shouldn't be\n"+
						"revision: %s\nexpected revision: %s\nnamespace labels: %+v\npod labels: %+v",
						revName, revName, tc.matchesRevision, tc.nsLabels, tc.podLabels)
				}
			})
		}
	}
}

func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
