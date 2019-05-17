package controlplane

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ControlPlaneReconciler) Delete() (reconcile.Result, error) {
	return reconcile.Result{}, r.prune(-1)
}
