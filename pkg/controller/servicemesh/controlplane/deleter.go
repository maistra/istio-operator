package controlplane

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ControlPlaneReconciler) Delete() (reconcile.Result, error) {
	r.Manager.GetRecorder(controllerName).Event(r.Instance, "Normal", "ServiceMeshDeleting", "Deleting service mesh")
	err := r.prune(-1)
	defer func() {
		if err == nil {
			r.Manager.GetRecorder(controllerName).Event(r.Instance, "Normal", "ServiceMeshDeleted", "Successfully deleted service mesh components")
		} else {
			r.Manager.GetRecorder(controllerName).Event(r.Instance, "Warning", "ServiceMeshDeleted", fmt.Sprintf("Error occurred during service mesh deletion: %s", err))
		}
	}()
	return reconcile.Result{}, err
}
