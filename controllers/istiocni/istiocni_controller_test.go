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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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
			reconciledCondition: newCondition(v1alpha1.IstioCNIConditionTypeReconciled, true, ""),
			readyCondition:      newCondition(v1alpha1.IstioCNIConditionTypeReady, true, ""),
			expectedState:       v1alpha1.IstioCNIConditionReasonHealthy,
		},
		{
			name:                "not reconciled",
			reconciledCondition: newCondition(v1alpha1.IstioCNIConditionTypeReconciled, false, v1alpha1.IstioCNIConditionReasonReconcileError),
			readyCondition:      newCondition(v1alpha1.IstioCNIConditionTypeReady, true, ""),
			expectedState:       v1alpha1.IstioCNIConditionReasonReconcileError,
		},
		{
			name:                "not ready",
			reconciledCondition: newCondition(v1alpha1.IstioCNIConditionTypeReconciled, true, ""),
			readyCondition:      newCondition(v1alpha1.IstioCNIConditionTypeReady, false, v1alpha1.IstioCNIConditionReasonIstiodNotReady),
			expectedState:       v1alpha1.IstioCNIConditionReasonIstiodNotReady,
		},
		{
			name:                "not reconciled nor ready",
			reconciledCondition: newCondition(v1alpha1.IstioCNIConditionTypeReconciled, false, v1alpha1.IstioCNIConditionReasonReconcileError),
			readyCondition:      newCondition(v1alpha1.IstioCNIConditionTypeReady, false, v1alpha1.IstioCNIConditionReasonIstiodNotReady),
			expectedState:       v1alpha1.IstioCNIConditionReasonReconcileError, // reconcile reason takes precedence over ready reason
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

func newCondition(conditionType v1alpha1.IstioCNIConditionType, status bool, reason v1alpha1.IstioCNIConditionReason) v1alpha1.IstioCNICondition {
	st := metav1.ConditionFalse
	if status {
		st = metav1.ConditionTrue
	}
	return v1alpha1.IstioCNICondition{
		Type:   conditionType,
		Status: st,
		Reason: reason,
	}
}

func TestDetermineReadyCondition(t *testing.T) {
	testCases := []struct {
		name          string
		cniEnabled    bool
		clientObjects []client.Object
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
				Type:   v1alpha1.IstioCNIConditionTypeReady,
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
				Type:    v1alpha1.IstioCNIConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioCNIConditionReasonCNINotReady,
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
				Type:    v1alpha1.IstioCNIConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioCNIConditionReasonCNINotReady,
				Message: "no istio-cni-node pods are currently scheduled",
			},
		},
		{
			name:          "CNI not found",
			clientObjects: []client.Object{},
			expected: v1alpha1.IstioCNICondition{
				Type:    v1alpha1.IstioCNIConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.IstioCNIConditionReasonCNINotReady,
				Message: "istio-cni-node DaemonSet not found",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tt.clientObjects...).Build()

			r := NewIstioCNIReconciler(cl, scheme.Scheme, nil)

			cni := &v1alpha1.IstioCNI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-istio",
				},
				Spec: v1alpha1.IstioCNISpec{
					Namespace: "istio-cni",
				},
			}

			result := r.determineReadyCondition(context.TODO(), cni)
			if result.Type != tt.expected.Type || result.Status != tt.expected.Status ||
				result.Reason != tt.expected.Reason || result.Message != tt.expected.Message {
				t.Errorf("Unexpected result.\nGot:\n    %+v\nexpected:\n    %+v", result, tt.expected)
			}
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
			result := applyImageDigests(tc.input, tc.config)
			if diff := cmp.Diff(tc.expectValues, result); diff != "" {
				t.Errorf("unexpected merge result; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
