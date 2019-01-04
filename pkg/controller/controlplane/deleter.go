package controlplane

import (
	"context"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *controlPlaneReconciler) Delete() (reconcile.Result, error) {
	allErrors := []error{}
	for key := range r.instance.Status.ComponentStatus {
		err := r.processComponentManifests(key)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	err := utilerrors.NewAggregate(allErrors)
	updateDeleteStatus(&r.instance.Status.StatusType, err)

	updateErr := r.client.Status().Update(context.TODO(), r.instance)
	if updateErr != nil {
		r.log.Error(err, "error updating ControlPlane status for object", "object", r.instance.GetName())
		if err == nil {
			// XXX: is this the right thing to do?
			return reconcile.Result{}, updateErr
		}
	}
	return reconcile.Result{}, err
}
