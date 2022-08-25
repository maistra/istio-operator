package common

import (
	"fmt"
	"testing"
	"time"

	errors2 "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

var (
	resource         = schema.GroupResource{Group: "v1", Resource: "Pod"}
	errorNonConflict = fmt.Errorf("not a conflict")
	errorConflict    = apierrors.NewConflict(resource, "foo", fmt.Errorf("conflict"))
)

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name           string
		resultToReturn reconcile.Result
		errorToReturn  error
		expectedResult reconcile.Result
		expectedError  error
	}{
		{
			name:           "no-error",
			resultToReturn: reconcile.Result{Requeue: false},
			errorToReturn:  nil,
			expectedResult: reconcile.Result{Requeue: false},
			expectedError:  nil,
		},
		{
			name:           "conflict",
			resultToReturn: reconcile.Result{},
			errorToReturn:  errorConflict,
			expectedResult: reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second},
			expectedError:  nil,
		},
		{
			name:           "other-error",
			resultToReturn: reconcile.Result{},
			errorToReturn:  errorNonConflict,
			expectedResult: reconcile.Result{},
			expectedError:  errorNonConflict,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reconciler := &fakeReconciler{
				resultToReturn: tc.resultToReturn,
				errorToReturn:  tc.errorToReturn,
			}
			r := NewConflictHandlingReconciler(reconciler)

			result, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}})

			assert.Equals(result, tc.expectedResult, "Unexpected result returned by Reconcile()", t)
			assert.Equals(err, tc.expectedError, "Unexpected error returned by Reconcile()", t)
		})
	}
}

func TestIsConflict(t *testing.T) {
	testCases := []struct {
		name       string
		err        error
		isConflict bool
	}{
		{
			name:       "not-conflict",
			err:        errorNonConflict,
			isConflict: false,
		},
		{
			name:       "simple-conflict",
			err:        errorConflict,
			isConflict: true,
		},
		{
			name:       "wrapped-conflict",
			err:        errors2.Wrapf(errorConflict, "error message"),
			isConflict: true,
		},
		{
			name:       "aggregate-single-conflict",
			err:        utilerrors.NewAggregate([]error{errorConflict}),
			isConflict: true,
		},
		{
			name:       "aggregate-multiple-conflicts",
			err:        utilerrors.NewAggregate([]error{errorConflict, errorConflict}),
			isConflict: true,
		},
		{
			name:       "aggregate-conflict-with-others",
			err:        utilerrors.NewAggregate([]error{errorConflict, errorNonConflict}),
			isConflict: false,
		},
		{
			name:       "aggregate-no-conflicts",
			err:        utilerrors.NewAggregate([]error{errorNonConflict, errorNonConflict}),
			isConflict: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isConflict := IsConflict(tc.err)
			assert.Equals(isConflict, tc.isConflict, "invalid value returned by IsConflict()", t)
		})
	}
}

type fakeReconciler struct {
	resultToReturn reconcile.Result
	errorToReturn  error
}

func (r *fakeReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	return r.resultToReturn, r.errorToReturn
}
