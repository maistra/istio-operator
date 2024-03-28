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

package kube

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/util/tests/kube"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var ctx = context.TODO()

func TestHasFinalizer(t *testing.T) {
	testCases := []struct {
		name           string
		finalizers     []string
		expectedResult bool
	}{
		{
			name:           "nil finalizers",
			finalizers:     nil,
			expectedResult: false,
		},
		{
			name:           "has other finalizer",
			finalizers:     []string{"example.com/some-finalizer"},
			expectedResult: false,
		},
		{
			name:           "has finalizer in question",
			finalizers:     []string{constants.FinalizerName},
			expectedResult: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			obj := &v1alpha1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: tc.finalizers,
				},
			}
			g.Expect(HasFinalizer(obj, constants.FinalizerName)).To(Equal(tc.expectedResult))
		})
	}
}

func TestRemoveFinalizer(t *testing.T) {
	tests := []struct {
		name               string
		initialFinalizers  []string
		interceptorFuncs   interceptor.Funcs
		expectResult       ctrl.Result
		expectError        bool
		checkFinalizers    bool
		expectedFinalizers []string
	}{
		{
			name: "object not found",
			interceptorFuncs: interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errors.NewNotFound(schema.GroupResource{}, "Istio")
				},
			},
			expectResult: ctrl.Result{},
			expectError:  false,
		},
		{
			name:              "update conflict",
			initialFinalizers: []string{constants.FinalizerName},
			interceptorFuncs: interceptor.Funcs{
				Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					return errors.NewConflict(schema.GroupResource{}, "dummy", fmt.Errorf("simulated conflict error"))
				},
			},
			expectResult: ctrl.Result{RequeueAfter: 2 * time.Second},
			expectError:  false,
		},
		{
			name:              "update error",
			initialFinalizers: []string{constants.FinalizerName},
			interceptorFuncs: interceptor.Funcs{
				Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					return fmt.Errorf("simulated update error")
				},
			},
			expectResult: ctrl.Result{},
			expectError:  true,
		},
		{
			name:               "success with single finalizer",
			initialFinalizers:  []string{constants.FinalizerName},
			expectResult:       ctrl.Result{},
			expectError:        false,
			checkFinalizers:    true,
			expectedFinalizers: nil,
		},
		{
			name:               "success with other finalizers",
			initialFinalizers:  []string{constants.FinalizerName, "example.com/some-finalizer"},
			expectResult:       ctrl.Result{},
			expectError:        false,
			checkFinalizers:    true,
			expectedFinalizers: []string{"example.com/some-finalizer"},
		},
		{
			name:               "success with no finalizer",
			initialFinalizers:  nil,
			expectResult:       ctrl.Result{},
			expectError:        false,
			checkFinalizers:    true,
			expectedFinalizers: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			obj := &v1alpha1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: tc.initialFinalizers,
				},
			}

			cl := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(obj).
				WithInterceptorFuncs(tc.interceptorFuncs).
				Build()

			result, err := RemoveFinalizer(ctx, cl, obj, constants.FinalizerName)

			g.Expect(result).To(Equal(tc.expectResult))

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tc.checkFinalizers {
				g.Expect(cl.Get(ctx, kube.GetObjectKey(obj), obj)).To(Succeed())
				g.Expect(obj.GetFinalizers()).To(ConsistOf(tc.expectedFinalizers))
			}
		})
	}
}

func TestAddFinalizer(t *testing.T) {
	tests := []struct {
		name               string
		initialFinalizers  []string
		interceptorFuncs   interceptor.Funcs
		expectResult       ctrl.Result
		expectError        bool
		checkFinalizers    bool
		expectedFinalizers []string
	}{
		{
			name: "object not found",
			interceptorFuncs: interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errors.NewNotFound(schema.GroupResource{}, "Istio")
				},
			},
			expectResult: ctrl.Result{},
			expectError:  false,
		},
		{
			name:              "update conflict",
			initialFinalizers: nil,
			interceptorFuncs: interceptor.Funcs{
				Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					return errors.NewConflict(schema.GroupResource{}, "dummy", fmt.Errorf("simulated conflict error"))
				},
			},
			expectResult: ctrl.Result{RequeueAfter: 2 * time.Second},
			expectError:  false,
		},
		{
			name:              "update error",
			initialFinalizers: nil,
			interceptorFuncs: interceptor.Funcs{
				Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					return fmt.Errorf("simulated update error")
				},
			},
			expectResult: ctrl.Result{},
			expectError:  true,
		},
		{
			name:               "success with no previous finalizer",
			initialFinalizers:  nil,
			expectResult:       ctrl.Result{},
			expectError:        false,
			checkFinalizers:    true,
			expectedFinalizers: []string{constants.FinalizerName},
		},
		{
			name:               "success with other finalizers",
			initialFinalizers:  []string{"example.com/some-finalizer"},
			expectResult:       ctrl.Result{},
			expectError:        false,
			checkFinalizers:    true,
			expectedFinalizers: []string{"example.com/some-finalizer", constants.FinalizerName},
		},
		{
			name:               "finalizer already present",
			initialFinalizers:  []string{constants.FinalizerName},
			expectResult:       ctrl.Result{},
			expectError:        false,
			checkFinalizers:    true,
			expectedFinalizers: []string{constants.FinalizerName},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			obj := &v1alpha1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Finalizers: tc.initialFinalizers,
				},
			}

			cl := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(obj).
				WithInterceptorFuncs(tc.interceptorFuncs).
				Build()

			result, err := AddFinalizer(ctx, cl, obj, constants.FinalizerName)

			g.Expect(result).To(Equal(tc.expectResult))

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tc.checkFinalizers {
				g.Expect(cl.Get(ctx, kube.GetObjectKey(obj), obj)).To(Succeed())
				g.Expect(obj.GetFinalizers()).To(ConsistOf(tc.expectedFinalizers))
			}
		})
	}
}
