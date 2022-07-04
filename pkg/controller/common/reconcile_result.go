package common

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func Reconciled() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func RequeueWithError(err error) (reconcile.Result, error) {
	return reconcile.Result{}, err
}

func RequeueAfter(time time.Duration) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: time}, nil
}
