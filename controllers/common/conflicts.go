package common

import (
	"context"
	"errors"
	"time"

	errors2 "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	errors3 "k8s.io/apimachinery/pkg/util/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type conflictHandlingReconciler struct {
	reconciler reconcile.Reconciler
}

func NewConflictHandlingReconciler(reconciler reconcile.Reconciler) reconcile.Reconciler {
	return &conflictHandlingReconciler{
		reconciler: reconciler,
	}
}

func (r *conflictHandlingReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	result, err := r.reconciler.Reconcile(ctx, request)
	if IsConflict(err) {
		log := logf.Log.WithName("conflict-handler")
		log.Info("Update conflict detected. Retrying...", "error", err)

		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Second,
		}, nil
	}
	return result, err
}

func IsConflict(err error) bool {
	if err == nil {
		return false
	}

	if apierrors.IsConflict(err) {
		return true
	} else if wrappedErr, ok := err.(errors3.Aggregate); ok && len(wrappedErr.Errors()) > 0 {
		for _, e := range wrappedErr.Errors() {
			if !IsConflict(e) {
				return false
			}
		}
		return true
	} else if wrappedErr := errors.Unwrap(err); wrappedErr != nil {
		return IsConflict(wrappedErr)
	} else if wrappedErr := errors2.Cause(err); wrappedErr != err {
		return IsConflict(wrappedErr)
	}

	return false
}
