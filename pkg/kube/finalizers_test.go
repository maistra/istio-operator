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
	"github.com/istio-ecosystem/sail-operator/pkg/common"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestHasFinalizer(t *testing.T) {
	testCases := []struct {
		finalizers     []string
		expectedResult bool
	}{
		{
			finalizers:     nil,
			expectedResult: false,
		},
		{
			finalizers:     []string{"example.com/some-finalizer"},
			expectedResult: false,
		},
		{
			finalizers:     []string{common.FinalizerName},
			expectedResult: true,
		},
	}
	for _, tc := range testCases {
		obj := &v1alpha1.Istio{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: tc.finalizers,
			},
		}
		assert.Equal(t, HasFinalizer(obj, common.FinalizerName), tc.expectedResult)
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
			initialFinalizers: []string{common.FinalizerName},
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
			initialFinalizers: []string{common.FinalizerName},
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
			initialFinalizers:  []string{common.FinalizerName},
			expectResult:       ctrl.Result{},
			expectError:        false,
			checkFinalizers:    true,
			expectedFinalizers: nil,
		},
		{
			name:               "success with other finalizers",
			initialFinalizers:  []string{common.FinalizerName, "example.com/some-finalizer"},
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

			result, err := RemoveFinalizer(context.TODO(), cl, obj, common.FinalizerName)

			assert.Equal(t, result, tc.expectResult)

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if tc.checkFinalizers {
				assert.NilError(t, cl.Get(context.TODO(), types.NamespacedName{Name: obj.GetName()}, obj))
				assert.DeepEqual(t, sets.NewString(obj.GetFinalizers()...), sets.NewString(tc.expectedFinalizers...))
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
			expectedFinalizers: []string{common.FinalizerName},
		},
		{
			name:               "success with other finalizers",
			initialFinalizers:  []string{"example.com/some-finalizer"},
			expectResult:       ctrl.Result{},
			expectError:        false,
			checkFinalizers:    true,
			expectedFinalizers: []string{"example.com/some-finalizer", common.FinalizerName},
		},
		{
			name:               "finalizer already present",
			initialFinalizers:  []string{common.FinalizerName},
			expectResult:       ctrl.Result{},
			expectError:        false,
			checkFinalizers:    true,
			expectedFinalizers: []string{common.FinalizerName},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

			result, err := AddFinalizer(context.TODO(), cl, obj, common.FinalizerName)

			assert.Equal(t, result, tc.expectResult)

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if tc.checkFinalizers {
				assert.NilError(t, cl.Get(context.TODO(), types.NamespacedName{Name: obj.GetName()}, obj))
				assert.DeepEqual(t, sets.NewString(obj.GetFinalizers()...), sets.NewString(tc.expectedFinalizers...))
			}
		})
	}
}
