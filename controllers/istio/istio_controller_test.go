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

package istio

import (
	"context"
	"fmt"
	"os"
	"path"
	"reflect"
	"runtime/debug"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/common"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"istio.io/istio/pkg/ptr"
)

var (
	ctx            = context.Background()
	istioNamespace = "my-istio-namespace"
	istioName      = "my-istio"
	istioKey       = types.NamespacedName{
		Name: istioName,
	}
	istioUID   = types.UID("my-istio-uid")
	objectMeta = metav1.ObjectMeta{
		Name: istioKey.Name,
	}
)

func TestReconcile(t *testing.T) {
	resourceDir := t.TempDir()

	req := ctrl.Request{NamespacedName: istioKey}

	t.Run("skips reconciliation when Istio not found", func(t *testing.T) {
		cl := newFakeClientBuilder().
			WithInterceptorFuncs(noWrites(t)).
			Build()
		reconciler := NewIstioReconciler(cl, scheme.Scheme, resourceDir, nil)

		_, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			t.Errorf("Expected no error, but got: %v", err)
		}
	})

	t.Run("skips reconciliation when Istio deleted", func(t *testing.T) {
		istio := &v1alpha1.Istio{
			ObjectMeta: metav1.ObjectMeta{
				Name:              istioKey.Name,
				DeletionTimestamp: oneMinuteAgo(),
				Finalizers:        []string{"dummy"}, // the fake client doesn't allow you to add a deleted object unless it has a finalizer
			},
		}

		cl := newFakeClientBuilder().
			WithObjects(istio).
			WithInterceptorFuncs(noWrites(t)).
			Build()
		reconciler := NewIstioReconciler(cl, scheme.Scheme, resourceDir, nil)

		_, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			t.Errorf("Expected no error, but got: %v", err)
		}
	})

	t.Run("returns error when it fails to get Istio", func(t *testing.T) {
		cl := newFakeClientBuilder().
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return fmt.Errorf("internal error")
				},
			}).
			Build()
		reconciler := NewIstioReconciler(cl, scheme.Scheme, resourceDir, nil)

		_, err := reconciler.Reconcile(ctx, req)
		if err == nil {
			t.Errorf("Expected an error, but got nil")
		}
	})

	t.Run("returns error when Istio version not set", func(t *testing.T) {
		istio := &v1alpha1.Istio{
			ObjectMeta: objectMeta,
		}

		cl := newFakeClientBuilder().
			WithObjects(istio).
			Build()
		reconciler := NewIstioReconciler(cl, scheme.Scheme, resourceDir, nil)

		_, err := reconciler.Reconcile(ctx, req)
		if err == nil {
			t.Errorf("Expected an error, but got nil")
		}

		Must(t, cl.Get(ctx, istioKey, istio))

		if istio.Status.State != v1alpha1.IstioReasonReconcileError {
			t.Errorf("Expected status.state to be %q, but got %q", v1alpha1.IstioReasonReconcileError, istio.Status.State)
		}

		reconciledCond := istio.Status.GetCondition(v1alpha1.IstioConditionReconciled)
		if reconciledCond.Status != metav1.ConditionFalse {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionFalse, reconciledCond.Status)
		}

		readyCond := istio.Status.GetCondition(v1alpha1.IstioConditionReady)
		if readyCond.Status != metav1.ConditionUnknown {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionUnknown, readyCond.Status)
		}
	})

	t.Run("returns error when computeIstioRevisionValues fails", func(t *testing.T) {
		istio := &v1alpha1.Istio{
			ObjectMeta: objectMeta,
			Spec: v1alpha1.IstioSpec{
				Version: "my-version",
			},
		}

		cl := newFakeClientBuilder().
			WithStatusSubresource(&v1alpha1.Istio{}).
			WithObjects(istio).
			Build()
		reconciler := NewIstioReconciler(cl, scheme.Scheme, resourceDir, []string{"invalid-profile"})

		_, err := reconciler.Reconcile(ctx, req)
		if err == nil {
			t.Errorf("Expected an error, but got nil")
		}

		Must(t, cl.Get(ctx, istioKey, istio))

		if istio.Status.State != v1alpha1.IstioReasonReconcileError {
			t.Errorf("Expected status.state to be %q, but got %q", v1alpha1.IstioReasonReconcileError, istio.Status.State)
		}

		reconciledCond := istio.Status.GetCondition(v1alpha1.IstioConditionReconciled)
		if reconciledCond.Status != metav1.ConditionFalse {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionFalse, reconciledCond.Status)
		}

		readyCond := istio.Status.GetCondition(v1alpha1.IstioConditionReady)
		if readyCond.Status != metav1.ConditionUnknown {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionUnknown, readyCond.Status)
		}
	})

	t.Run("returns error when reconcileActiveRevision fails", func(t *testing.T) {
		istio := &v1alpha1.Istio{
			ObjectMeta: objectMeta,
			Spec: v1alpha1.IstioSpec{
				Version: "my-version",
			},
		}

		cl := newFakeClientBuilder().
			WithObjects(istio).
			WithInterceptorFuncs(interceptor.Funcs{
				Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
					return fmt.Errorf("internal error")
				},
			}).
			Build()
		reconciler := NewIstioReconciler(cl, scheme.Scheme, resourceDir, nil)

		_, err := reconciler.Reconcile(ctx, req)
		if err == nil {
			t.Errorf("Expected an error, but got nil")
		}

		Must(t, cl.Get(ctx, istioKey, istio))

		if istio.Status.State != v1alpha1.IstioReasonReconcileError {
			t.Errorf("Expected status.state to be %q, but got %q", v1alpha1.IstioReasonReconcileError, istio.Status.State)
		}

		reconciledCond := istio.Status.GetCondition(v1alpha1.IstioConditionReconciled)
		if reconciledCond.Status != metav1.ConditionFalse {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionFalse, reconciledCond.Status)
		}

		readyCond := istio.Status.GetCondition(v1alpha1.IstioConditionReady)
		if readyCond.Status != metav1.ConditionUnknown {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionUnknown, readyCond.Status)
		}
	})
}

func TestUpdateStatus(t *testing.T) {
	resourceDir := t.TempDir()

	generation := int64(100)
	oneMinuteAgo := oneMinuteAgo()

	ownedByIstio := metav1.OwnerReference{
		APIVersion:         v1alpha1.GroupVersion.String(),
		Kind:               v1alpha1.IstioKind,
		Name:               istioName,
		UID:                istioUID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	ownedByAnotherIstio := metav1.OwnerReference{
		APIVersion:         v1alpha1.GroupVersion.String(),
		Kind:               v1alpha1.IstioKind,
		Name:               "some-other-Istio",
		UID:                "some-other-uid",
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	revision := func(name string, ownerRef metav1.OwnerReference, reconciled, ready, inUse bool) v1alpha1.IstioRevision {
		return v1alpha1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			Spec: v1alpha1.IstioRevisionSpec{Namespace: istioNamespace},
			Status: v1alpha1.IstioRevisionStatus{
				State: v1alpha1.IstioRevisionReasonHealthy,
				Conditions: []v1alpha1.IstioRevisionCondition{
					{Type: v1alpha1.IstioRevisionConditionReconciled, Status: toConditionStatus(reconciled)},
					{Type: v1alpha1.IstioRevisionConditionReady, Status: toConditionStatus(ready)},
					{Type: v1alpha1.IstioRevisionConditionInUse, Status: toConditionStatus(inUse)},
				},
			},
		}
	}

	testCases := []struct {
		name              string
		reconciliationErr error
		istio             *v1alpha1.Istio
		revisions         []v1alpha1.IstioRevision
		interceptorFuncs  *interceptor.Funcs
		disallowWrites    bool
		wantErr           bool
		expectedStatus    v1alpha1.IstioStatus
	}{
		{
			name:              "reconciliation error",
			reconciliationErr: fmt.Errorf("reconciliation error"),
			wantErr:           false,
			expectedStatus: v1alpha1.IstioStatus{
				State:              v1alpha1.IstioReasonReconcileError,
				ObservedGeneration: generation,
				Conditions: []v1alpha1.IstioCondition{
					{
						Type:    v1alpha1.IstioConditionReconciled,
						Status:  metav1.ConditionFalse,
						Reason:  v1alpha1.IstioReasonReconcileError,
						Message: "reconciliation error",
					},
					{
						Type:    v1alpha1.IstioConditionReady,
						Status:  metav1.ConditionUnknown,
						Reason:  v1alpha1.IstioReasonReconcileError,
						Message: "cannot determine readiness due to reconciliation error",
					},
				},
			},
		},
		{
			name:    "mirrors status of active revision",
			wantErr: false,
			revisions: []v1alpha1.IstioRevision{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            istioKey.Name,
						OwnerReferences: []metav1.OwnerReference{ownedByIstio},
					},
					Spec: v1alpha1.IstioRevisionSpec{
						Namespace: istioNamespace,
					},
					Status: v1alpha1.IstioRevisionStatus{
						State: v1alpha1.IstioRevisionReasonHealthy,
						Conditions: []v1alpha1.IstioRevisionCondition{
							{
								Type:    v1alpha1.IstioRevisionConditionReconciled,
								Status:  metav1.ConditionTrue,
								Reason:  v1alpha1.IstioRevisionReasonHealthy,
								Message: "reconciled message",
							},
							{
								Type:    v1alpha1.IstioRevisionConditionReady,
								Status:  metav1.ConditionTrue,
								Reason:  v1alpha1.IstioRevisionReasonHealthy,
								Message: "ready message",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            istioKey.Name + "-not-active",
						OwnerReferences: []metav1.OwnerReference{ownedByIstio},
					},
					Spec: v1alpha1.IstioRevisionSpec{
						Namespace: istioNamespace,
					},
					Status: v1alpha1.IstioRevisionStatus{
						State: v1alpha1.IstioRevisionReasonHealthy,
						Conditions: []v1alpha1.IstioRevisionCondition{
							{
								Type:    v1alpha1.IstioRevisionConditionReconciled,
								Status:  metav1.ConditionFalse,
								Reason:  v1alpha1.IstioRevisionReasonHealthy,
								Message: "shouldn't mirror this revision",
							},
							{
								Type:    v1alpha1.IstioRevisionConditionReady,
								Status:  metav1.ConditionFalse,
								Reason:  v1alpha1.IstioRevisionReasonHealthy,
								Message: "shouldn't mirror this revision",
							},
						},
					},
				},
			},
			expectedStatus: v1alpha1.IstioStatus{
				State:              v1alpha1.IstioReasonHealthy,
				ObservedGeneration: generation,
				Conditions: []v1alpha1.IstioCondition{
					{
						Type:    v1alpha1.IstioConditionReconciled,
						Status:  metav1.ConditionTrue,
						Reason:  v1alpha1.IstioReasonHealthy,
						Message: "reconciled message",
					},
					{
						Type:    v1alpha1.IstioConditionReady,
						Status:  metav1.ConditionTrue,
						Reason:  v1alpha1.IstioReasonHealthy,
						Message: "ready message",
					},
				},
				Revisions: v1alpha1.RevisionSummary{
					Total: 2,
					Ready: 1,
					InUse: 0,
				},
			},
		},
		{
			name:    "shows correct revision counts",
			wantErr: false,
			revisions: []v1alpha1.IstioRevision{
				// owned by the Istio under test; 3 todal, 2 ready, 1 in use
				revision(istioKey.Name, ownedByIstio, true, true, true),
				revision(istioKey.Name+"-old1", ownedByIstio, true, true, false),
				revision(istioKey.Name+"-old2", ownedByIstio, true, false, false),
				// not owned by the Istio being tested; shouldn't affect counts
				revision("some-other-istio", ownedByAnotherIstio, true, true, true),
			},
			expectedStatus: v1alpha1.IstioStatus{
				State:              v1alpha1.IstioReasonHealthy,
				ObservedGeneration: generation,
				Conditions: []v1alpha1.IstioCondition{
					{
						Type:   v1alpha1.IstioConditionReconciled,
						Status: metav1.ConditionTrue,
					},
					{
						Type:   v1alpha1.IstioConditionReady,
						Status: metav1.ConditionTrue,
					},
				},
				Revisions: v1alpha1.RevisionSummary{
					Total: 3,
					Ready: 2,
					InUse: 1,
				},
			},
		},
		{
			name:    "active revision not found",
			wantErr: false,
			expectedStatus: v1alpha1.IstioStatus{
				State:              v1alpha1.IstioReasonRevisionNotFound,
				ObservedGeneration: generation,
				Conditions: []v1alpha1.IstioCondition{
					{
						Type:    v1alpha1.IstioConditionReconciled,
						Status:  metav1.ConditionFalse,
						Reason:  v1alpha1.IstioReasonRevisionNotFound,
						Message: "active IstioRevision not found",
					},
					{
						Type:    v1alpha1.IstioConditionReady,
						Status:  metav1.ConditionFalse,
						Reason:  v1alpha1.IstioReasonRevisionNotFound,
						Message: "active IstioRevision not found",
					},
				},
			},
		},
		{
			name: "get active revision error",
			interceptorFuncs: &interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					if _, ok := obj.(*v1alpha1.IstioRevision); ok {
						return fmt.Errorf("get error")
					}
					return nil
				},
			},
			wantErr: true,
		},
		{
			name: "skips update when status unchanged",
			istio: &v1alpha1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name:       istioKey.Name,
					UID:        istioUID,
					Generation: 100,
				},
				Spec: v1alpha1.IstioSpec{
					Version:   "my-version",
					Namespace: istioNamespace,
				},
				Status: v1alpha1.IstioStatus{
					ObservedGeneration: 100,
					State:              v1alpha1.IstioReasonHealthy,
					Conditions: []v1alpha1.IstioCondition{
						{
							Type:               v1alpha1.IstioConditionReconciled,
							Status:             metav1.ConditionTrue,
							Reason:             v1alpha1.IstioReasonHealthy,
							Message:            "reconciled message",
							LastTransitionTime: *oneMinuteAgo,
						},
						{
							Type:               v1alpha1.IstioConditionReady,
							Status:             metav1.ConditionTrue,
							Reason:             v1alpha1.IstioReasonHealthy,
							Message:            "ready message",
							LastTransitionTime: *oneMinuteAgo,
						},
					},
				},
			},
			revisions: []v1alpha1.IstioRevision{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: istioKey.Name,
					},
					Spec: v1alpha1.IstioRevisionSpec{
						Namespace: istioNamespace,
					},
					Status: v1alpha1.IstioRevisionStatus{
						State: v1alpha1.IstioRevisionReasonHealthy,
						Conditions: []v1alpha1.IstioRevisionCondition{
							{
								Type:               v1alpha1.IstioRevisionConditionReconciled,
								Status:             metav1.ConditionTrue,
								Reason:             v1alpha1.IstioRevisionReasonHealthy,
								Message:            "reconciled message",
								LastTransitionTime: *oneMinuteAgo,
							},
							{
								Type:               v1alpha1.IstioRevisionConditionReady,
								Status:             metav1.ConditionTrue,
								Reason:             v1alpha1.IstioRevisionReasonHealthy,
								Message:            "ready message",
								LastTransitionTime: *oneMinuteAgo,
							},
						},
					},
				},
			},
			expectedStatus: v1alpha1.IstioStatus{
				State:              v1alpha1.IstioReasonHealthy,
				ObservedGeneration: generation,
				Conditions: []v1alpha1.IstioCondition{
					{
						Type:    v1alpha1.IstioConditionReconciled,
						Status:  metav1.ConditionTrue,
						Reason:  v1alpha1.IstioReasonHealthy,
						Message: "reconciled message",
					},
					{
						Type:    v1alpha1.IstioConditionReady,
						Status:  metav1.ConditionTrue,
						Reason:  v1alpha1.IstioReasonHealthy,
						Message: "ready message",
					},
				},
			},
			disallowWrites: true,
			wantErr:        false,
		},
		{
			name: "returns status update error",
			interceptorFuncs: &interceptor.Funcs{
				SubResourcePatch: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
					return fmt.Errorf("patch status error")
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var interceptorFuncs interceptor.Funcs
			if tc.disallowWrites {
				if tc.interceptorFuncs != nil {
					panic("can't use disallowWrites and interceptorFuncs at the same time")
				}
				interceptorFuncs = noWrites(t)
			} else if tc.interceptorFuncs != nil {
				interceptorFuncs = *tc.interceptorFuncs
			}

			istio := tc.istio
			if istio == nil {
				istio = &v1alpha1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:       istioKey.Name,
						UID:        istioUID,
						Generation: 100,
					},
					Spec: v1alpha1.IstioSpec{
						Version:   "my-version",
						Namespace: istioNamespace,
					},
				}
			}

			initObjs := []client.Object{istio}
			for _, rev := range tc.revisions {
				rev := rev
				initObjs = append(initObjs, &rev)
			}

			cl := newFakeClientBuilder().
				WithObjects(initObjs...).
				WithInterceptorFuncs(interceptorFuncs).
				Build()
			reconciler := NewIstioReconciler(cl, scheme.Scheme, resourceDir, nil)

			err := reconciler.updateStatus(ctx, istio, tc.reconciliationErr)
			if (err != nil) != tc.wantErr {
				t.Errorf("updateStatus() error = %v, wantErr %v", err, tc.wantErr)
			}

			Must(t, cl.Get(ctx, istioKey, istio))
			// clear timestamps for comparison
			for i := range istio.Status.Conditions {
				istio.Status.Conditions[i].LastTransitionTime = metav1.Time{}
			}
			if diff := cmp.Diff(tc.expectedStatus, istio.Status); diff != "" {
				t.Errorf("status wasn't updated as expected; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func toConditionStatus(b bool) metav1.ConditionStatus {
	if b {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func TestReconcileActiveRevision(t *testing.T) {
	resourceDir := t.TempDir()

	const version = "my-version"

	testCases := []struct {
		name                 string
		istioValues          v1alpha1.Values
		revValues            *v1alpha1.Values
		expectOwnerReference bool
	}{
		{
			name: "creates IstioRevision",
			istioValues: v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: "quay.io/hub",
				},
				MeshConfig: &v1alpha1.MeshConfig{
					AccessLogFile: "/dev/stdout",
				},
			},
			expectOwnerReference: true,
		},
		{
			name: "updates IstioRevision",
			istioValues: v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: "quay.io/new-hub",
				},
				MeshConfig: &v1alpha1.MeshConfig{
					AccessLogFile: "/dev/stdout",
				},
			},
			revValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "old-image",
				},
			},
			expectOwnerReference: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			subCases := []struct {
				name               string
				updateStrategyType *v1alpha1.UpdateStrategyType
				revName            string
			}{
				{
					name:               "default update strategy",
					updateStrategyType: nil,
					revName:            "my-istio",
				},
				{
					name:               "InPlace",
					updateStrategyType: ptr.Of(v1alpha1.UpdateStrategyTypeInPlace),
					revName:            "my-istio",
				},
				{
					name:               "RevisionBased",
					updateStrategyType: ptr.Of(v1alpha1.UpdateStrategyTypeRevisionBased),
					revName:            "my-istio-" + version,
				},
			}

			for _, sc := range subCases {
				t.Run(sc.name, func(t *testing.T) {
					istio := &v1alpha1.Istio{
						ObjectMeta: objectMeta,
						Spec: v1alpha1.IstioSpec{
							Version: version,
							Values:  &tc.istioValues,
						},
					}
					if sc.updateStrategyType != nil {
						istio.Spec.UpdateStrategy = &v1alpha1.IstioUpdateStrategy{
							Type: *sc.updateStrategyType,
						}
					}

					initObjs := []client.Object{istio}

					if tc.revValues != nil {
						initObjs = append(initObjs,
							&v1alpha1.IstioRevision{
								ObjectMeta: metav1.ObjectMeta{
									Name: sc.revName,
								},
								Spec: v1alpha1.IstioRevisionSpec{
									Version: version,
									Values:  tc.revValues,
								},
							},
						)
					}

					cl := newFakeClientBuilder().WithObjects(initObjs...).Build()
					reconciler := NewIstioReconciler(cl, scheme.Scheme, resourceDir, nil)

					_, err := reconciler.reconcileActiveRevision(ctx, istio, &tc.istioValues)
					if err != nil {
						t.Errorf("Expected no error, but got: %v", err)
					}

					revKey := types.NamespacedName{Name: sc.revName}
					rev := &v1alpha1.IstioRevision{}
					Must(t, cl.Get(ctx, revKey, rev))

					var expectedOwnerRefs []metav1.OwnerReference
					if tc.expectOwnerReference {
						expectedOwnerRefs = []metav1.OwnerReference{
							{
								APIVersion:         v1alpha1.GroupVersion.String(),
								Kind:               v1alpha1.IstioKind,
								Name:               istio.Name,
								UID:                istio.UID,
								Controller:         ptr.Of(true),
								BlockOwnerDeletion: ptr.Of(true),
							},
						}
					}
					if diff := cmp.Diff(rev.OwnerReferences, expectedOwnerRefs); diff != "" {
						t.Errorf("invalid ownerReference; diff (-expected, +actual):\n%v", diff)
					}

					if istio.Spec.Version != rev.Spec.Version {
						t.Errorf("IstioRevision.spec.version doesn't match Istio.spec.version; expected %s, got %s", istio.Spec.Version, rev.Spec.Version)
					}

					if diff := cmp.Diff(tc.istioValues.ToHelmValues(), rev.Spec.Values.ToHelmValues()); diff != "" {
						t.Errorf("IstioRevision.spec.values don't match Istio.spec.values; diff (-expected, +actual):\n%v", diff)
					}
				})
			}
		})
	}
}

func TestPruneInactiveRevisions(t *testing.T) {
	resourceDir := t.TempDir()

	const istioName = "my-istio"
	const istioUID = "my-uid"
	const version = "my-version"

	ownedByIstio := metav1.OwnerReference{
		APIVersion:         v1alpha1.GroupVersion.String(),
		Kind:               v1alpha1.IstioKind,
		Name:               istioName,
		UID:                istioUID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	ownedByAnotherIstio := metav1.OwnerReference{
		APIVersion:         v1alpha1.GroupVersion.String(),
		Kind:               v1alpha1.IstioKind,
		Name:               "some-other-Istio",
		UID:                "some-other-uid",
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	tenSecondsAgo := metav1.Time{Time: time.Now().Add(-10 * time.Second)}
	oneMinuteAgo := metav1.Time{Time: time.Now().Add(-1 * time.Minute)}
	testCases := []struct {
		name                string
		revName             string
		ownerReference      metav1.OwnerReference
		inUseCondition      *v1alpha1.IstioRevisionCondition
		rev                 *v1alpha1.IstioRevision
		expectDeletion      bool
		expectRequeueAfter  *time.Duration
		additionalRevisions []*v1alpha1.IstioRevision
	}{
		{
			name:           "preserves active IstioRevision even if not in use",
			revName:        istioName,
			ownerReference: ownedByIstio,
			inUseCondition: &v1alpha1.IstioRevisionCondition{
				Type:               v1alpha1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: oneMinuteAgo,
			},
			expectDeletion:     false,
			expectRequeueAfter: nil,
		},
		{
			name:           "preserves non-active IstioRevision that's in use",
			revName:        istioName + "-non-active",
			ownerReference: ownedByIstio,
			inUseCondition: &v1alpha1.IstioRevisionCondition{
				Type:               v1alpha1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: tenSecondsAgo,
			},
			expectDeletion:     false,
			expectRequeueAfter: nil,
		},
		{
			name:           "preserves unused non-active IstioRevision during grace period",
			revName:        istioName + "-non-active",
			ownerReference: ownedByIstio,
			inUseCondition: &v1alpha1.IstioRevisionCondition{
				Type:               v1alpha1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: tenSecondsAgo,
			},
			expectDeletion:     false,
			expectRequeueAfter: ptr.Of((v1alpha1.DefaultRevisionDeletionGracePeriodSeconds - 10) * time.Second),
		},
		{
			name:           "preserves IstioRevision owned by a different Istio",
			revName:        "other-istio-non-active",
			ownerReference: ownedByAnotherIstio,
			inUseCondition: &v1alpha1.IstioRevisionCondition{
				Type:               v1alpha1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: oneMinuteAgo,
			},
			expectDeletion:     false,
			expectRequeueAfter: nil,
		},
		{
			name:           "deletes non-active IstioRevision that's not in use",
			revName:        istioName + "-non-active",
			ownerReference: ownedByIstio,
			inUseCondition: &v1alpha1.IstioRevisionCondition{
				Type:               v1alpha1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: oneMinuteAgo,
			},
			expectDeletion:     true,
			expectRequeueAfter: nil,
		},
		{
			name:           "returns requeueAfter of earliest IstioRevision requiring pruning",
			revName:        istioName + "-non-active",
			ownerReference: ownedByIstio,
			inUseCondition: &v1alpha1.IstioRevisionCondition{
				Type:               v1alpha1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: oneMinuteAgo,
			},
			additionalRevisions: []*v1alpha1.IstioRevision{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            istioName + "-non-active2",
						OwnerReferences: []metav1.OwnerReference{ownedByIstio},
					},
					Status: v1alpha1.IstioRevisionStatus{
						Conditions: []v1alpha1.IstioRevisionCondition{
							{
								Type:               v1alpha1.IstioRevisionConditionInUse,
								Status:             metav1.ConditionFalse,
								LastTransitionTime: metav1.Time{Time: time.Now().Add(-25 * time.Second)},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            istioName + "-non-active3",
						OwnerReferences: []metav1.OwnerReference{ownedByIstio},
					},
					Status: v1alpha1.IstioRevisionStatus{
						Conditions: []v1alpha1.IstioRevisionCondition{
							{
								Type:               v1alpha1.IstioRevisionConditionInUse,
								Status:             metav1.ConditionFalse,
								LastTransitionTime: metav1.Time{Time: time.Now().Add(-20 * time.Second)},
							},
						},
					},
				},
			},
			expectDeletion:     true,
			expectRequeueAfter: ptr.Of((v1alpha1.DefaultRevisionDeletionGracePeriodSeconds - 25) * time.Second),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			istio := &v1alpha1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioName,
					UID:  istioUID,
				},
				Spec: v1alpha1.IstioSpec{
					Version: version,
				},
			}

			rev := &v1alpha1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name:            tc.revName,
					OwnerReferences: []metav1.OwnerReference{tc.ownerReference},
				},
				Status: v1alpha1.IstioRevisionStatus{
					Conditions: []v1alpha1.IstioRevisionCondition{*tc.inUseCondition},
				},
			}

			initObjs := []client.Object{istio, rev}
			for _, additionalRev := range tc.additionalRevisions {
				initObjs = append(initObjs, additionalRev)
			}

			cl := newFakeClientBuilder().WithObjects(initObjs...).Build()
			reconciler := NewIstioReconciler(cl, scheme.Scheme, resourceDir, nil)

			result, err := reconciler.pruneInactiveRevisions(ctx, istio)
			if err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			revisionWasDeleted := errors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(rev), rev))
			if tc.expectDeletion && !revisionWasDeleted {
				t.Error("Expected IstioRevision to be deleted, but it wasn't")
			} else if revisionWasDeleted && !tc.expectDeletion {
				t.Error("Expected IstioRevision to be preserved, but it was deleted")
			}

			if tc.expectRequeueAfter == nil {
				if result.RequeueAfter != 0 {
					t.Errorf("Didn't expect Istio to be requeued, but it was; requeueAfter: %v", result.RequeueAfter)
				}
			} else {
				if result.RequeueAfter == 0 {
					t.Error("Expected Istio to be requeued, but it wasn't")
				} else {
					diff := abs(result.RequeueAfter - *tc.expectRequeueAfter)
					if diff > time.Second {
						t.Errorf("Expected result.RequeueAfter to be around %v, but got %v", *tc.expectRequeueAfter, result.RequeueAfter)
					}
				}
			}
		})
	}
}

func abs(duration time.Duration) time.Duration {
	if duration < 0 {
		return -duration
	}
	return duration
}

func TestGetActiveRevisionName(t *testing.T) {
	tests := []struct {
		name                 string
		version              string
		updateStrategyType   *v1alpha1.UpdateStrategyType
		expectedRevisionName string
	}{
		{
			name:                 "No update strategy specified",
			version:              "1.0.0",
			updateStrategyType:   nil,
			expectedRevisionName: "test-istio",
		},
		{
			name:                 "InPlace",
			version:              "1.0.0",
			updateStrategyType:   ptr.Of(v1alpha1.UpdateStrategyTypeInPlace),
			expectedRevisionName: "test-istio",
		},
		{
			name:                 "RevisionBased v1.0.0",
			version:              "1.0.0",
			updateStrategyType:   ptr.Of(v1alpha1.UpdateStrategyTypeRevisionBased),
			expectedRevisionName: "test-istio-1-0-0",
		},
		{
			name:                 "RevisionBased v2.0.0",
			version:              "2.0.0",
			updateStrategyType:   ptr.Of(v1alpha1.UpdateStrategyTypeRevisionBased),
			expectedRevisionName: "test-istio-2-0-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			istio := &v1alpha1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-istio",
				},
				Spec: v1alpha1.IstioSpec{
					Version: tt.version,
				},
			}
			if tt.updateStrategyType != nil {
				istio.Spec.UpdateStrategy = &v1alpha1.IstioUpdateStrategy{
					Type: *tt.updateStrategyType,
				}
			}
			actual := getActiveRevisionName(istio)
			if actual != tt.expectedRevisionName {
				t.Errorf("getActiveRevisionName() = %v, want %v", actual, tt.expectedRevisionName)
			}
		})
	}
}

func newFakeClientBuilder() *fake.ClientBuilder {
	return fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithStatusSubresource(&v1alpha1.Istio{})
}

func TestGetPruningGracePeriod(t *testing.T) {
	tests := []struct {
		name           string
		updateStrategy *v1alpha1.IstioUpdateStrategy
		expected       time.Duration
	}{
		{
			name:           "Nil update strategy",
			updateStrategy: nil,
			expected:       v1alpha1.DefaultRevisionDeletionGracePeriodSeconds * time.Second,
		},
		{
			name:           "Nil grace period",
			updateStrategy: &v1alpha1.IstioUpdateStrategy{},
			expected:       v1alpha1.DefaultRevisionDeletionGracePeriodSeconds * time.Second,
		},
		{
			name: "Grace period less than minimum",
			updateStrategy: &v1alpha1.IstioUpdateStrategy{
				InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(v1alpha1.MinRevisionDeletionGracePeriodSeconds - 10)),
			},
			expected: v1alpha1.MinRevisionDeletionGracePeriodSeconds * time.Second,
		},
		{
			name: "Grace period more than minimum",
			updateStrategy: &v1alpha1.IstioUpdateStrategy{
				InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(v1alpha1.MinRevisionDeletionGracePeriodSeconds + 10)),
			},
			expected: (v1alpha1.MinRevisionDeletionGracePeriodSeconds + 10) * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			istio := &v1alpha1.Istio{
				Spec: v1alpha1.IstioSpec{
					UpdateStrategy: tt.updateStrategy,
				},
			}
			got := getPruningGracePeriod(istio)
			if got != tt.expected {
				t.Errorf("getPruningGracePeriod() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestGetAggregatedValues tests that the values are sourced from the following sources
// (with each source overriding the values from the previous sources):
//   - default profile(s)
//   - profile selected in IstioRevision.spec.profile
//   - IstioRevision.spec.values
//   - other (non-value) fields in the IstioRevision resource (e.g. the value global.istioNamespace is set from IstioRevision.spec.namespace)
func TestComputeIstioRevisionValues(t *testing.T) {
	const version = "my-version"
	resourceDir := t.TempDir()
	profilesDir := path.Join(resourceDir, version, "profiles")
	Must(t, os.MkdirAll(profilesDir, 0o755))

	Must(t, os.WriteFile(path.Join(profilesDir, "default.yaml"), []byte((`
apiVersion: operator.istio.io/v1alpha1
kind: IstioRevision
spec:
  values:
    pilot: 
      hub: from-default-profile
      tag: from-default-profile      # this gets overridden in my-profile
      image: from-default-profile    # this gets overridden in my-profile and values`)), 0o644))

	Must(t, os.WriteFile(path.Join(profilesDir, "my-profile.yaml"), []byte((`
apiVersion: operator.istio.io/v1alpha1
kind: IstioRevision
spec:
  values:
    pilot:
      tag: from-my-profile
      image: from-my-profile  # this gets overridden in values`)), 0o644))

	istio := v1alpha1.Istio{
		ObjectMeta: objectMeta,
		Spec: v1alpha1.IstioSpec{
			Version:   version,
			Profile:   "my-profile",
			Namespace: istioNamespace,
			Values: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "from-istio-spec-values",
				},
			},
		},
	}

	result, err := computeIstioRevisionValues(istio, []string{"default"}, resourceDir)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}

	expected := &v1alpha1.Values{
		Pilot: &v1alpha1.PilotConfig{
			Hub:   "from-default-profile",
			Tag:   ptr.Of(intstr.FromString("from-my-profile")),
			Image: "from-istio-spec-values",
		},
		Global: &v1alpha1.GlobalConfig{
			IstioNamespace: istioNamespace, // this value is always added/overridden based on IstioRevision.spec.namespace
		},
		Revision: objectMeta.Name,
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Result does not match the expected Values.\nExpected: %v\nActual: %v", expected, result)
	}
}

func TestApplyImageDigests(t *testing.T) {
	testCases := []struct {
		name         string
		config       common.OperatorConfig
		inputIstio   *v1alpha1.Istio
		inputValues  *v1alpha1.Values
		expectValues *v1alpha1.Values
	}{
		{
			name: "no-config",
			config: common.OperatorConfig{
				ImageDigests: map[string]common.IstioImageConfig{},
			},
			inputIstio: &v1alpha1.Istio{
				Spec: v1alpha1.IstioSpec{
					Version: "v1.20.0",
				},
			},
			inputValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "istiod-test",
				},
			},
			expectValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "istiod-test",
				},
			},
		},
		{
			name: "no-user-values",
			config: common.OperatorConfig{
				ImageDigests: map[string]common.IstioImageConfig{
					"v1.20.0": {
						IstiodImage:  "istiod-test",
						ProxyImage:   "proxy-test",
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			inputIstio: &v1alpha1.Istio{
				Spec: v1alpha1.IstioSpec{
					Version: "v1.20.0",
				},
			},
			inputValues: &v1alpha1.Values{},
			expectValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "istiod-test",
				},
				Global: &v1alpha1.GlobalConfig{
					Proxy: &v1alpha1.ProxyConfig{
						Image: "proxy-test",
					},
					ProxyInit: &v1alpha1.ProxyInitConfig{
						Image: "proxy-test",
					},
				},
				// ZTunnel: &v1alpha1.ZTunnelConfig{
				// 	Image: "ztunnel-test",
				// },
			},
		},
		{
			name: "user-supplied-image",
			config: common.OperatorConfig{
				ImageDigests: map[string]common.IstioImageConfig{
					"v1.20.0": {
						IstiodImage:  "istiod-test",
						ProxyImage:   "proxy-test",
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			inputIstio: &v1alpha1.Istio{
				Spec: v1alpha1.IstioSpec{
					Version: "v1.20.0",
				},
			},
			inputValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "istiod-custom",
				},
			},
			expectValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "istiod-custom",
				},
				Global: &v1alpha1.GlobalConfig{
					Proxy: &v1alpha1.ProxyConfig{
						Image: "proxy-test",
					},
					ProxyInit: &v1alpha1.ProxyInitConfig{
						Image: "proxy-test",
					},
				},
				// ZTunnel: &v1alpha1.ZTunnelConfig{
				// 	Image: "ztunnel-test",
				// },
			},
		},
		{
			name: "user-supplied-hub-tag",
			config: common.OperatorConfig{
				ImageDigests: map[string]common.IstioImageConfig{
					"v1.20.0": {
						IstiodImage:  "istiod-test",
						ProxyImage:   "proxy-test",
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			inputIstio: &v1alpha1.Istio{
				Spec: v1alpha1.IstioSpec{
					Version: "v1.20.0",
				},
			},
			inputValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: "docker.io/istio",
					Tag: ptr.Of(intstr.FromString("1.20.1")),
				},
			},
			expectValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: "docker.io/istio",
					Tag: ptr.Of(intstr.FromString("1.20.1")),
				},
				Global: &v1alpha1.GlobalConfig{
					Proxy: &v1alpha1.ProxyConfig{
						Image: "proxy-test",
					},
					ProxyInit: &v1alpha1.ProxyInitConfig{
						Image: "proxy-test",
					},
				},
				// ZTunnel: &v1alpha1.ZTunnelConfig{
				// 	Image: "ztunnel-test",
				// },
			},
		},
		{
			name: "version-without-defaults",
			config: common.OperatorConfig{
				ImageDigests: map[string]common.IstioImageConfig{
					"v1.20.0": {
						IstiodImage:  "istiod-test",
						ProxyImage:   "proxy-test",
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			inputIstio: &v1alpha1.Istio{
				Spec: v1alpha1.IstioSpec{
					Version: "v1.20.1",
				},
			},
			inputValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: "docker.io/istio",
					Tag: ptr.Of(intstr.FromString("1.20.2")),
				},
			},
			expectValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: "docker.io/istio",
					Tag: ptr.Of(intstr.FromString("1.20.2")),
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := applyImageDigests(tc.inputIstio, tc.inputValues, tc.config)
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

func noWrites(t *testing.T) interceptor.Funcs {
	return interceptor.Funcs{
		Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
			t.Fatal("unexpected call to Create in", string(debug.Stack()))
			return nil
		},
		Update: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.UpdateOption) error {
			t.Fatal("unexpected call to Update in", string(debug.Stack()))
			return nil
		},
		Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
			t.Fatal("unexpected call to Delete in", string(debug.Stack()))
			return nil
		},
		Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
			t.Fatal("unexpected call to Patch in", string(debug.Stack()))
			return nil
		},
		DeleteAllOf: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteAllOfOption) error {
			t.Fatal("unexpected call to DeleteAllOf in", string(debug.Stack()))
			return nil
		},
		SubResourceCreate: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
			t.Fatal("unexpected call to SubResourceCreate in", string(debug.Stack()))
			return nil
		},
		SubResourceUpdate: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ ...client.SubResourceUpdateOption) error {
			t.Fatal("unexpected call to SubResourceUpdate in", string(debug.Stack()))
			return nil
		},
		SubResourcePatch: func(_ context.Context, _ client.Client, _ string, obj client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
			t.Fatalf("unexpected call to SubResourcePatch with the object %+v: %v", obj, string(debug.Stack()))
			return nil
		},
	}
}

func oneMinuteAgo() *metav1.Time {
	t := metav1.NewTime(time.Now().Add(-1 * time.Minute))
	return &t
}
