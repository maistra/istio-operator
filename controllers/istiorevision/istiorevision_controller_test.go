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
	"fmt"
	"strings"
	"testing"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"istio.io/istio/pkg/ptr"
)

func TestDeriveState(t *testing.T) {
	testCases := []struct {
		name                string
		reconciledCondition v1alpha1.IstioRevisionCondition
		readyCondition      v1alpha1.IstioRevisionCondition
		expectedState       v1alpha1.IstioRevisionConditionReason
	}{
		{
			name:                "healthy",
			reconciledCondition: newCondition(v1alpha1.IstioRevisionConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1alpha1.IstioRevisionConditionReady, metav1.ConditionTrue, ""),
			expectedState:       v1alpha1.IstioRevisionReasonHealthy,
		},
		{
			name:                "not reconciled",
			reconciledCondition: newCondition(v1alpha1.IstioRevisionConditionReconciled, metav1.ConditionFalse, v1alpha1.IstioRevisionReasonReconcileError),
			readyCondition:      newCondition(v1alpha1.IstioRevisionConditionReady, metav1.ConditionTrue, ""),
			expectedState:       v1alpha1.IstioRevisionReasonReconcileError,
		},
		{
			name:                "not ready",
			reconciledCondition: newCondition(v1alpha1.IstioRevisionConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1alpha1.IstioRevisionConditionReady, metav1.ConditionFalse, v1alpha1.IstioRevisionReasonIstiodNotReady),
			expectedState:       v1alpha1.IstioRevisionReasonIstiodNotReady,
		},
		{
			name:                "readiness unknown",
			reconciledCondition: newCondition(v1alpha1.IstioRevisionConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1alpha1.IstioRevisionConditionReady, metav1.ConditionUnknown, v1alpha1.IstioRevisionReasonReadinessCheckFailed),
			expectedState:       v1alpha1.IstioRevisionReasonReadinessCheckFailed,
		},
		{
			name:                "not reconciled nor ready",
			reconciledCondition: newCondition(v1alpha1.IstioRevisionConditionReconciled, metav1.ConditionFalse, v1alpha1.IstioRevisionReasonReconcileError),
			readyCondition:      newCondition(v1alpha1.IstioRevisionConditionReady, metav1.ConditionFalse, v1alpha1.IstioRevisionReasonIstiodNotReady),
			expectedState:       v1alpha1.IstioRevisionReasonReconcileError, // reconcile reason takes precedence over ready reason
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result := deriveState(tc.reconciledCondition, tc.readyCondition)
			g.Expect(result).To(Equal(tc.expectedState))
		})
	}
}

func newCondition(
	conditionType v1alpha1.IstioRevisionConditionType, status metav1.ConditionStatus, reason v1alpha1.IstioRevisionConditionReason,
) v1alpha1.IstioRevisionCondition {
	return v1alpha1.IstioRevisionCondition{
		Type:   conditionType,
		Status: status,
		Reason: reason,
	}
}

func TestDetermineReadyCondition(t *testing.T) {
	testCases := []struct {
		name          string
		values        *v1alpha1.Values
		clientObjects []client.Object
		interceptors  interceptor.Funcs
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
				Type:   v1alpha1.IstioRevisionConditionReady,
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
				Type:    v1alpha1.IstioRevisionConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioRevisionReasonIstiodNotReady,
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
				Type:    v1alpha1.IstioRevisionConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioRevisionReasonIstiodNotReady,
				Message: "istiod Deployment is scaled to zero replicas",
			},
		},
		{
			name:          "Istiod not found",
			values:        nil,
			clientObjects: []client.Object{},
			expected: v1alpha1.IstioRevisionCondition{
				Type:    v1alpha1.IstioRevisionConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioRevisionReasonIstiodNotReady,
				Message: "istiod Deployment not found",
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
				Type:   v1alpha1.IstioRevisionConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name:          "client error on get",
			clientObjects: []client.Object{},
			interceptors: interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					return fmt.Errorf("simulated error")
				},
			},
			expected: v1alpha1.IstioRevisionCondition{
				Type:    v1alpha1.IstioRevisionConditionReady,
				Status:  metav1.ConditionUnknown,
				Reason:  v1alpha1.IstioRevisionReasonReadinessCheckFailed,
				Message: "failed to get readiness: simulated error",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tt.clientObjects...).WithInterceptorFuncs(tt.interceptors).Build()

			r := NewIstioRevisionReconciler(cl, scheme.Scheme, "no-resource-dir", nil)

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
			g.Expect(result.Type).To(Equal(tt.expected.Type))
			g.Expect(result.Status).To(Equal(tt.expected.Status))
			g.Expect(result.Reason).To(Equal(tt.expected.Reason))
			g.Expect(result.Message).To(Equal(tt.expected.Message))
		})
	}
}

func TestDetermineInUseCondition(t *testing.T) {
	testCases := []struct {
		podLabels           map[string]string
		podAnnotations      map[string]string
		nsLabels            map[string]string
		enableAllNamespaces bool
		interceptors        interceptor.Funcs
		matchesRevision     string
		expectUnknownState  bool
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
		{
			interceptors: interceptor.Funcs{
				List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					return fmt.Errorf("simulated error")
				},
			},
			expectUnknownState: true,
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
				g := NewWithT(t)
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
					WithInterceptorFuncs(tc.interceptors).
					Build()

				r := NewIstioRevisionReconciler(cl, scheme.Scheme, "no-resource-dir", nil)

				result := r.determineInUseCondition(context.TODO(), rev)
				g.Expect(result.Type).To(Equal(v1alpha1.IstioRevisionConditionInUse))

				if tc.expectUnknownState {
					g.Expect(result.Status).To(Equal(metav1.ConditionUnknown))
					g.Expect(result.Reason).To(Equal(v1alpha1.IstioRevisionReasonUsageCheckFailed))
				} else {
					if revName == tc.matchesRevision {
						g.Expect(result.Status).To(Equal(metav1.ConditionTrue),
							fmt.Sprintf("Revision %s should be in use, but isn't\n"+
								"revision: %s\nexpected revision: %s\nnamespace labels: %+v\npod labels: %+v",
								revName, revName, tc.matchesRevision, tc.nsLabels, tc.podLabels))
					} else {
						g.Expect(result.Status).To(Equal(metav1.ConditionFalse),
							fmt.Sprintf("Revision %s should not be in use\n"+
								"revision: %s\nexpected revision: %s\nnamespace labels: %+v\npod labels: %+v",
								revName, revName, tc.matchesRevision, tc.nsLabels, tc.podLabels))
					}
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
