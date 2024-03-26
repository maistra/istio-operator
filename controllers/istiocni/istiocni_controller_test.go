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

package istiocni

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/common"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"istio.io/istio/pkg/ptr"
)

func TestDeriveState(t *testing.T) {
	testCases := []struct {
		name                string
		reconciledCondition v1alpha1.IstioCNICondition
		readyCondition      v1alpha1.IstioCNICondition
		expectedState       v1alpha1.IstioCNIConditionReason
	}{
		{
			name:                "healthy",
			reconciledCondition: newCondition(v1alpha1.IstioCNIConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1alpha1.IstioCNIConditionReady, metav1.ConditionTrue, ""),
			expectedState:       v1alpha1.IstioCNIReasonHealthy,
		},
		{
			name:                "not reconciled",
			reconciledCondition: newCondition(v1alpha1.IstioCNIConditionReconciled, metav1.ConditionFalse, v1alpha1.IstioCNIReasonReconcileError),
			readyCondition:      newCondition(v1alpha1.IstioCNIConditionReady, metav1.ConditionTrue, ""),
			expectedState:       v1alpha1.IstioCNIReasonReconcileError,
		},
		{
			name:                "not ready",
			reconciledCondition: newCondition(v1alpha1.IstioCNIConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1alpha1.IstioCNIConditionReady, metav1.ConditionFalse, v1alpha1.IstioCNIDaemonSetNotReady),
			expectedState:       v1alpha1.IstioCNIDaemonSetNotReady,
		},
		{
			name:                "readiness unknown",
			reconciledCondition: newCondition(v1alpha1.IstioCNIConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1alpha1.IstioCNIConditionReady, metav1.ConditionUnknown, v1alpha1.IstioCNIReasonReadinessCheckFailed),
			expectedState:       v1alpha1.IstioCNIReasonReadinessCheckFailed,
		},
		{
			name:                "not reconciled nor ready",
			reconciledCondition: newCondition(v1alpha1.IstioCNIConditionReconciled, metav1.ConditionFalse, v1alpha1.IstioCNIReasonReconcileError),
			readyCondition:      newCondition(v1alpha1.IstioCNIConditionReady, metav1.ConditionFalse, v1alpha1.IstioCNIDaemonSetNotReady),
			expectedState:       v1alpha1.IstioCNIReasonReconcileError, // reconcile reason takes precedence over ready reason
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

func newCondition(condType v1alpha1.IstioCNIConditionType, status metav1.ConditionStatus, reason v1alpha1.IstioCNIConditionReason) v1alpha1.IstioCNICondition {
	return v1alpha1.IstioCNICondition{
		Type:   condType,
		Status: status,
		Reason: reason,
	}
}

func TestDetermineReadyCondition(t *testing.T) {
	resourceDir := t.TempDir()

	testCases := []struct {
		name          string
		cniEnabled    bool
		clientObjects []client.Object
		interceptors  interceptor.Funcs
		expected      v1alpha1.IstioCNICondition
	}{
		{
			name: "CNI ready",
			clientObjects: []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: "istio-cni",
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 1,
						NumberReady:            1,
					},
				},
			},
			expected: v1alpha1.IstioCNICondition{
				Type:   v1alpha1.IstioCNIConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "CNI not ready",
			clientObjects: []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: "istio-cni",
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 1,
						NumberReady:            0,
					},
				},
			},
			expected: v1alpha1.IstioCNICondition{
				Type:    v1alpha1.IstioCNIConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioCNIDaemonSetNotReady,
				Message: "not all istio-cni-node pods are ready",
			},
		},
		{
			name: "CNI pods not scheduled",
			clientObjects: []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: "istio-cni",
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 0,
						NumberReady:            0,
					},
				},
			},
			expected: v1alpha1.IstioCNICondition{
				Type:    v1alpha1.IstioCNIConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioCNIDaemonSetNotReady,
				Message: "no istio-cni-node pods are currently scheduled",
			},
		},
		{
			name:          "CNI not found",
			clientObjects: []client.Object{},
			expected: v1alpha1.IstioCNICondition{
				Type:    v1alpha1.IstioCNIConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioCNIDaemonSetNotReady,
				Message: "istio-cni-node DaemonSet not found",
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
			expected: v1alpha1.IstioCNICondition{
				Type:    v1alpha1.IstioCNIConditionReady,
				Status:  metav1.ConditionUnknown,
				Reason:  v1alpha1.IstioCNIReasonReadinessCheckFailed,
				Message: "failed to get readiness: simulated error",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tt.clientObjects...).WithInterceptorFuncs(tt.interceptors).Build()

			r := NewIstioCNIReconciler(cl, scheme.Scheme, nil, resourceDir, nil, nil)

			cni := &v1alpha1.IstioCNI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-istio",
				},
				Spec: v1alpha1.IstioCNISpec{
					Namespace: "istio-cni",
				},
			}

			result := r.determineReadyCondition(context.TODO(), cni)
			g.Expect(result.Type).To(Equal(tt.expected.Type))
			g.Expect(result.Status).To(Equal(tt.expected.Status))
			g.Expect(result.Reason).To(Equal(tt.expected.Reason))
			g.Expect(result.Message).To(Equal(tt.expected.Message))
		})
	}
}

func TestApplyImageDigests(t *testing.T) {
	testCases := []struct {
		name         string
		config       common.OperatorConfig
		input        *v1alpha1.IstioCNI
		expectValues *v1alpha1.CNIValues
	}{
		{
			name: "no-config",
			config: common.OperatorConfig{
				ImageDigests: map[string]common.IstioImageConfig{},
			},
			input: &v1alpha1.IstioCNI{
				Spec: v1alpha1.IstioCNISpec{
					Version: "v1.20.0",
					Values: &v1alpha1.CNIValues{
						Cni: &v1alpha1.CNIConfig{
							Image: "istiocni-test",
						},
					},
				},
			},
			expectValues: &v1alpha1.CNIValues{
				Cni: &v1alpha1.CNIConfig{
					Image: "istiocni-test",
				},
			},
		},
		{
			name: "no-user-values",
			config: common.OperatorConfig{
				ImageDigests: map[string]common.IstioImageConfig{
					"v1.20.0": {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1alpha1.IstioCNI{
				Spec: v1alpha1.IstioCNISpec{
					Version: "v1.20.0",
					Values:  &v1alpha1.CNIValues{},
				},
			},
			expectValues: &v1alpha1.CNIValues{
				Cni: &v1alpha1.CNIConfig{
					Image: "cni-test",
				},
			},
		},
		{
			name: "user-supplied-image",
			config: common.OperatorConfig{
				ImageDigests: map[string]common.IstioImageConfig{
					"v1.20.0": {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1alpha1.IstioCNI{
				Spec: v1alpha1.IstioCNISpec{
					Version: "v1.20.0",
					Values: &v1alpha1.CNIValues{
						Cni: &v1alpha1.CNIConfig{
							Image: "cni-custom",
						},
					},
				},
			},
			expectValues: &v1alpha1.CNIValues{
				Cni: &v1alpha1.CNIConfig{
					Image: "cni-custom",
				},
			},
		},
		{
			name: "user-supplied-hub-tag",
			config: common.OperatorConfig{
				ImageDigests: map[string]common.IstioImageConfig{
					"v1.20.0": {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1alpha1.IstioCNI{
				Spec: v1alpha1.IstioCNISpec{
					Version: "v1.20.0",
					Values: &v1alpha1.CNIValues{
						Cni: &v1alpha1.CNIConfig{
							Hub: "docker.io/istio",
							Tag: ptr.Of(intstr.FromString("1.20.1")),
						},
					},
				},
			},
			expectValues: &v1alpha1.CNIValues{
				Cni: &v1alpha1.CNIConfig{
					Hub: "docker.io/istio",
					Tag: ptr.Of(intstr.FromString("1.20.1")),
				},
			},
		},
		{
			name: "version-without-defaults",
			config: common.OperatorConfig{
				ImageDigests: map[string]common.IstioImageConfig{
					"v1.20.0": {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1alpha1.IstioCNI{
				Spec: v1alpha1.IstioCNISpec{
					Version: "v1.20.1",
					Values: &v1alpha1.CNIValues{
						Cni: &v1alpha1.CNIConfig{
							Hub: "docker.io/istio",
							Tag: ptr.Of(intstr.FromString("1.20.2")),
						},
					},
				},
			},
			expectValues: &v1alpha1.CNIValues{
				Cni: &v1alpha1.CNIConfig{
					Hub: "docker.io/istio",
					Tag: ptr.Of(intstr.FromString("1.20.2")),
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := applyImageDigests(tc.input, tc.input.Spec.Values, tc.config)
			if diff := cmp.Diff(tc.expectValues, result); diff != "" {
				t.Errorf("unexpected merge result; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func TestDetermineStatus(t *testing.T) {
	tests := []struct {
		name         string
		reconcileErr error
	}{
		{
			name:         "no error",
			reconcileErr: nil,
		},
		{
			name:         "reconcile error",
			reconcileErr: fmt.Errorf("some reconcile error"),
		},
	}

	ctx := context.TODO()
	resourceDir := t.TempDir()
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	r := NewIstioCNIReconciler(cl, scheme.Scheme, nil, resourceDir, nil, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cni := &v1alpha1.IstioCNI{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cni",
					Generation: 123,
				},
			}

			reconciledCondition := r.determineReconciledCondition(tt.reconcileErr)
			readyCondition := r.determineReadyCondition(ctx, cni)

			status := r.determineStatus(ctx, cni, tt.reconcileErr)

			g.Expect(status.ObservedGeneration).To(Equal(cni.Generation))
			g.Expect(status.State).To(Equal(deriveState(reconciledCondition, readyCondition)))
			g.Expect(normalize(status.GetCondition(v1alpha1.IstioCNIConditionReconciled))).To(Equal(normalize(reconciledCondition)))
			g.Expect(normalize(status.GetCondition(v1alpha1.IstioCNIConditionReady))).To(Equal(normalize(readyCondition)))
		})
	}
}

func normalize(condition v1alpha1.IstioCNICondition) v1alpha1.IstioCNICondition {
	condition.LastTransitionTime = metav1.Time{}
	return condition
}
