package istio

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"runtime/debug"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	v1alpha1 "maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/helm"
	"maistra.io/istio-operator/pkg/test"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"istio.io/istio/pkg/ptr"
)

var (
	ctx            = context.Background()
	istioNamespace = "my-istio-namespace"
	istioKey       = types.NamespacedName{
		Name: "my-istio",
	}
	objectMeta = metav1.ObjectMeta{
		Name: istioKey.Name,
	}
)

func TestReconcile(t *testing.T) {
	test.SetupScheme()
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

		if istio.Status.State != v1alpha1.IstioConditionReasonReconcileError {
			t.Errorf("Expected status.state to be %q, but got %q", v1alpha1.IstioConditionReasonReconcileError, istio.Status.State)
		}

		reconciledCond := istio.Status.GetCondition(v1alpha1.IstioConditionTypeReconciled)
		if reconciledCond.Status != metav1.ConditionFalse {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionFalse, reconciledCond.Status)
		}

		readyCond := istio.Status.GetCondition(v1alpha1.IstioConditionTypeReady)
		if readyCond.Status != metav1.ConditionUnknown {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionUnknown, readyCond.Status)
		}
	})

	t.Run("returns error when getAggregatedValues fails", func(t *testing.T) {
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

		if istio.Status.State != v1alpha1.IstioConditionReasonReconcileError {
			t.Errorf("Expected status.state to be %q, but got %q", v1alpha1.IstioConditionReasonReconcileError, istio.Status.State)
		}

		reconciledCond := istio.Status.GetCondition(v1alpha1.IstioConditionTypeReconciled)
		if reconciledCond.Status != metav1.ConditionFalse {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionFalse, reconciledCond.Status)
		}

		readyCond := istio.Status.GetCondition(v1alpha1.IstioConditionTypeReady)
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

		if istio.Status.State != v1alpha1.IstioConditionReasonReconcileError {
			t.Errorf("Expected status.state to be %q, but got %q", v1alpha1.IstioConditionReasonReconcileError, istio.Status.State)
		}

		reconciledCond := istio.Status.GetCondition(v1alpha1.IstioConditionTypeReconciled)
		if reconciledCond.Status != metav1.ConditionFalse {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionFalse, reconciledCond.Status)
		}

		readyCond := istio.Status.GetCondition(v1alpha1.IstioConditionTypeReady)
		if readyCond.Status != metav1.ConditionUnknown {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionUnknown, readyCond.Status)
		}
	})
}

func TestUpdateStatus(t *testing.T) {
	test.SetupScheme()
	resourceDir := t.TempDir()

	generation := int64(100)
	oneMinuteAgo := oneMinuteAgo()

	testCases := []struct {
		name              string
		reconciliationErr error
		istio             *v1alpha1.Istio
		revision          *v1alpha1.IstioRevision
		interceptorFuncs  *interceptor.Funcs
		disallowWrites    bool
		wantErr           bool
		expectedStatus    v1alpha1.IstioStatus
	}{
		{
			name:              "reconciliation error",
			reconciliationErr: fmt.Errorf("reconciliation error"),
			wantErr:           true,
			expectedStatus: v1alpha1.IstioStatus{
				State:              v1alpha1.IstioConditionReasonReconcileError,
				ObservedGeneration: generation,
				Conditions: []v1alpha1.IstioCondition{
					{
						Type:    v1alpha1.IstioConditionTypeReconciled,
						Status:  metav1.ConditionFalse,
						Reason:  v1alpha1.IstioConditionReasonReconcileError,
						Message: "reconciliation error",
					},
					{
						Type:    v1alpha1.IstioConditionTypeReady,
						Status:  metav1.ConditionFalse,
						Reason:  v1alpha1.IstioConditionReasonReconcileError,
						Message: "reconciliation error",
					},
				},
			},
		},
		{
			name:    "mirrors status of active revision",
			wantErr: false,
			revision: &v1alpha1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioKey.Name,
				},
				Spec: v1alpha1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
				Status: v1alpha1.IstioRevisionStatus{
					State: v1alpha1.IstioRevisionConditionReasonHealthy,
					Conditions: []v1alpha1.IstioRevisionCondition{
						{
							Type:    v1alpha1.IstioRevisionConditionTypeReconciled,
							Status:  metav1.ConditionTrue,
							Reason:  v1alpha1.IstioRevisionConditionReasonHealthy,
							Message: "reconciled message",
						},
						{
							Type:    v1alpha1.IstioRevisionConditionTypeReady,
							Status:  metav1.ConditionTrue,
							Reason:  v1alpha1.IstioRevisionConditionReasonHealthy,
							Message: "ready message",
						},
					},
				},
			},
			expectedStatus: v1alpha1.IstioStatus{
				State:              v1alpha1.IstioConditionReasonHealthy,
				ObservedGeneration: generation,
				Conditions: []v1alpha1.IstioCondition{
					{
						Type:    v1alpha1.IstioConditionTypeReconciled,
						Status:  metav1.ConditionTrue,
						Reason:  v1alpha1.IstioConditionReasonHealthy,
						Message: "reconciled message",
					},
					{
						Type:    v1alpha1.IstioConditionTypeReady,
						Status:  metav1.ConditionTrue,
						Reason:  v1alpha1.IstioConditionReasonHealthy,
						Message: "ready message",
					},
				},
			},
		},
		{
			name:    "active revision not found",
			wantErr: false,
			expectedStatus: v1alpha1.IstioStatus{
				State:              v1alpha1.IstioConditionReasonHealthy,
				ObservedGeneration: generation,
				Conditions: []v1alpha1.IstioCondition{
					{
						Type:    v1alpha1.IstioConditionTypeReconciled,
						Status:  metav1.ConditionFalse,
						Reason:  v1alpha1.IstioConditionReasonIstioRevisionNotFound,
						Message: "active IstioRevision not found",
					},
					{
						Type:    v1alpha1.IstioConditionTypeReady,
						Status:  metav1.ConditionFalse,
						Reason:  v1alpha1.IstioConditionReasonIstioRevisionNotFound,
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
					Generation: 100,
				},
				Spec: v1alpha1.IstioSpec{
					Version:   "my-version",
					Namespace: istioNamespace,
				},
				Status: v1alpha1.IstioStatus{
					ObservedGeneration: 100,
					State:              v1alpha1.IstioConditionReasonHealthy,
					Conditions: []v1alpha1.IstioCondition{
						{
							Type:               v1alpha1.IstioConditionTypeReconciled,
							Status:             metav1.ConditionTrue,
							Reason:             v1alpha1.IstioConditionReasonHealthy,
							Message:            "reconciled message",
							LastTransitionTime: *oneMinuteAgo,
						},
						{
							Type:               v1alpha1.IstioConditionTypeReady,
							Status:             metav1.ConditionTrue,
							Reason:             v1alpha1.IstioConditionReasonHealthy,
							Message:            "ready message",
							LastTransitionTime: *oneMinuteAgo,
						},
					},
				},
			},
			revision: &v1alpha1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioKey.Name,
				},
				Spec: v1alpha1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
				Status: v1alpha1.IstioRevisionStatus{
					State: v1alpha1.IstioRevisionConditionReasonHealthy,
					Conditions: []v1alpha1.IstioRevisionCondition{
						{
							Type:               v1alpha1.IstioRevisionConditionTypeReconciled,
							Status:             metav1.ConditionTrue,
							Reason:             v1alpha1.IstioRevisionConditionReasonHealthy,
							Message:            "reconciled message",
							LastTransitionTime: *oneMinuteAgo,
						},
						{
							Type:               v1alpha1.IstioRevisionConditionTypeReady,
							Status:             metav1.ConditionTrue,
							Reason:             v1alpha1.IstioRevisionConditionReasonHealthy,
							Message:            "ready message",
							LastTransitionTime: *oneMinuteAgo,
						},
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
						Generation: 100,
					},
					Spec: v1alpha1.IstioSpec{
						Version:   "my-version",
						Namespace: istioNamespace,
					},
				}
			}

			initObjs := []client.Object{istio}
			if tc.revision != nil {
				initObjs = append(initObjs, tc.revision)
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
		})
	}
}

func TestReconcileActiveRevision(t *testing.T) {
	test.SetupScheme()
	resourceDir := t.TempDir()

	const version = "my-version"

	testCases := []struct {
		name                 string
		istioValues          helm.HelmValues
		revValues            *helm.HelmValues
		expectOwnerReference bool
	}{
		{
			name:                 "creates IstioRevision",
			istioValues:          helm.HelmValues{"key": "value"},
			expectOwnerReference: true,
		},
		{
			name:                 "updates IstioRevision",
			istioValues:          helm.HelmValues{"key": "new-value"},
			revValues:            &helm.HelmValues{"key": "old-value"},
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
							Values:  toJSON(tc.istioValues),
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
									Values:  toJSON(*tc.revValues),
								},
							},
						)
					}

					cl := newFakeClientBuilder().WithObjects(initObjs...).Build()
					reconciler := NewIstioReconciler(cl, scheme.Scheme, resourceDir, nil)

					err := reconciler.reconcileActiveRevision(ctx, istio, tc.istioValues)
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

					if diff := cmp.Diff(tc.istioValues, fromJSON(rev.Spec.Values)); diff != "" {
						t.Errorf("IstioRevision.spec.values don't match Istio.spec.values; diff (-expected, +actual):\n%v", diff)
					}
				})
			}
		})
	}
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
			r := &IstioReconciler{}
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
			actual := r.getActiveRevisionName(istio)
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

// TestGetAggregatedValues tests that the values are sourced from the following sources
// (with each source overriding the values from the previous sources):
//   - default profile(s)
//   - profile selected in IstioRevision.spec.profile
//   - IstioRevision.spec.values
//   - IstioRevision.spec.rawValues
//   - other (non-value) fields in the IstioRevision resource (e.g. the value global.istioNamespace is set from IstioRevision.spec.namespace)
func TestGetAggregatedValues(t *testing.T) {
	const version = "my-version"
	resourceDir := t.TempDir()
	profilesDir := path.Join(resourceDir, version, "profiles")
	Must(t, os.MkdirAll(profilesDir, 0o755))

	Must(t, os.WriteFile(path.Join(profilesDir, "default.yaml"), []byte((`
apiVersion: operator.istio.io/v1alpha1
kind: IstioRevision
spec:
  values:
    key1: from-default-profile
    key2: from-default-profile  # this gets overridden in my-profile
    key3: from-default-profile  # this gets overridden in my-profile and values
    key4: from-default-profile  # this gets overridden in my-profile, values, and rawValues`)), 0o644))

	Must(t, os.WriteFile(path.Join(profilesDir, "my-profile.yaml"), []byte((`
apiVersion: operator.istio.io/v1alpha1
kind: IstioRevision
spec:
  values:
    key2: overridden-in-my-profile
    key3: overridden-in-my-profile  # this gets overridden in values
    key4: overridden-in-my-profile  # this gets overridden in rawValues`)), 0o644))

	istio := v1alpha1.Istio{
		ObjectMeta: objectMeta,
		Spec: v1alpha1.IstioSpec{
			Version:   version,
			Profile:   "my-profile",
			Namespace: istioNamespace,
			Values: toJSON(helm.HelmValues{
				"key3": "overridden-in-values",
				"key4": "overridden-in-values", // this gets overridden in rawValues
			}),
			RawValues: toJSON(helm.HelmValues{
				"key4": "overridden-in-raw-values",
			}),
		},
	}

	result, err := getAggregatedValues(istio, []string{"default"}, resourceDir)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}

	expected := helm.HelmValues{
		"key1": "from-default-profile",
		"key2": "overridden-in-my-profile",
		"key3": "overridden-in-values",
		"key4": "overridden-in-raw-values",
		"global": map[string]any{
			"istioNamespace": istioNamespace, // this value is always added/overridden based on IstioRevision.spec.namespace
		},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Result does not match the expected HelmValues.\nExpected: %v\nActual: %v", expected, result)
	}
}

func toJSON(values helm.HelmValues) json.RawMessage {
	jsonVals, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return jsonVals
}

func fromJSON(values json.RawMessage) helm.HelmValues {
	helmValues := helm.HelmValues{}
	err := json.Unmarshal(values, &helmValues)
	if err != nil {
		panic(err)
	}
	return helmValues
}

func TestGetValuesFromProfiles(t *testing.T) {
	const version = "my-version"
	resourceDir := t.TempDir()
	profilesDir := path.Join(resourceDir, version, "profiles")
	Must(t, os.MkdirAll(profilesDir, 0o755))

	writeProfileFile := func(t *testing.T, path string, values ...string) {
		yaml := `
apiVersion: operator.istio.io/v1alpha1
kind: IstioRevision
spec:
  values:`
		for i, val := range values {
			if val != "" {
				yaml += fmt.Sprintf(`
    value%d: %s`, i+1, val)
			}
		}
		Must(t, os.WriteFile(path, []byte(yaml), 0o644))
	}

	writeProfileFile(t, path.Join(profilesDir, "default.yaml"), "1-from-default", "2-from-default")
	writeProfileFile(t, path.Join(profilesDir, "overlay.yaml"), "", "2-from-overlay")
	writeProfileFile(t, path.Join(profilesDir, "custom.yaml"), "1-from-custom")
	writeProfileFile(t, path.Join(resourceDir, version, "not-in-profiles-dir.yaml"), "should-not-be-accessible")

	tests := []struct {
		name         string
		profiles     []string
		expectValues helm.HelmValues
		expectErr    bool
	}{
		{
			name:         "nil default profiles",
			profiles:     nil,
			expectValues: helm.HelmValues{},
		},
		{
			name:     "default profile only",
			profiles: []string{"default"},
			expectValues: helm.HelmValues{
				"value1": "1-from-default",
				"value2": "2-from-default",
			},
		},
		{
			name:     "default and overlay",
			profiles: []string{"default", "overlay"},
			expectValues: helm.HelmValues{
				"value1": "1-from-default",
				"value2": "2-from-overlay",
			},
		},
		{
			name:     "default and overlay and custom",
			profiles: []string{"default", "overlay", "custom"},
			expectValues: helm.HelmValues{
				"value1": "1-from-custom",
				"value2": "2-from-overlay",
			},
		},
		{
			name:      "default profile empty",
			profiles:  []string{""},
			expectErr: true,
		},
		{
			name:      "profile not found",
			profiles:  []string{"invalid"},
			expectErr: true,
		},
		{
			name:      "path-traversal-attack",
			profiles:  []string{"../not-in-profiles-dir"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := getValuesFromProfiles(profilesDir, tt.profiles)
			if (err != nil) != tt.expectErr {
				t.Errorf("applyProfile() error = %v, expectErr %v", err, tt.expectErr)
			}

			if err == nil {
				if diff := cmp.Diff(tt.expectValues, actual); diff != "" {
					t.Errorf("profile wasn't applied properly; diff (-expected, +actual):\n%v", diff)
				}
			}
		})
	}
}

func TestMergeOverwrite(t *testing.T) {
	testCases := []struct {
		name                    string
		overrides, base, expect map[string]any
	}{
		{
			name:      "both empty",
			base:      make(map[string]any),
			overrides: make(map[string]any),
			expect:    make(map[string]any),
		},
		{
			name:      "nil overrides",
			base:      map[string]any{"key1": 42, "key2": "value"},
			overrides: nil,
			expect:    map[string]any{"key1": 42, "key2": "value"},
		},
		{
			name:      "nil base",
			base:      nil,
			overrides: map[string]any{"key1": 42, "key2": "value"},
			expect:    map[string]any{"key1": 42, "key2": "value"},
		},
		{
			name: "adds toplevel keys",
			base: map[string]any{
				"key2": "from base",
			},
			overrides: map[string]any{
				"key1": "from overrides",
			},
			expect: map[string]any{
				"key1": "from overrides",
				"key2": "from base",
			},
		},
		{
			name: "adds nested keys",
			base: map[string]any{
				"key1": map[string]any{
					"nested2": "from base",
				},
			},
			overrides: map[string]any{
				"key1": map[string]any{
					"nested1": "from overrides",
				},
			},
			expect: map[string]any{
				"key1": map[string]any{
					"nested1": "from overrides",
					"nested2": "from base",
				},
			},
		},
		{
			name: "overrides overrides base",
			base: map[string]any{
				"key1": "from base",
				"key2": map[string]any{
					"nested1": "from base",
				},
			},
			overrides: map[string]any{
				"key1": "from overrides",
				"key2": map[string]any{
					"nested1": "from overrides",
				},
			},
			expect: map[string]any{
				"key1": "from overrides",
				"key2": map[string]any{
					"nested1": "from overrides",
				},
			},
		},
		{
			name: "mismatched types",
			base: map[string]any{
				"key1": map[string]any{
					"desc": "key1 is a map in base",
				},
				"key2": "key2 is a string in base",
			},
			overrides: map[string]any{
				"key1": "key1 is a string in overrides",
				"key2": map[string]any{
					"desc": "key2 is a map in overrides",
				},
			},
			expect: map[string]any{
				"key1": "key1 is a string in overrides",
				"key2": map[string]any{
					"desc": "key2 is a map in overrides",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mergeOverwrite(tc.base, tc.overrides)
			if diff := cmp.Diff(tc.expect, result); diff != "" {
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
		SubResourcePatch: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
			t.Fatal("unexpected call to SubResourcePatch in", string(debug.Stack()))
			return nil
		},
	}
}

func oneMinuteAgo() *metav1.Time {
	t := metav1.NewTime(time.Now().Add(-1 * time.Minute))
	return &t
}
