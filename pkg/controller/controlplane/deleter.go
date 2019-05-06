package controlplane

import (
	"context"

	istiov1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/v1alpha3"

	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *controlPlaneReconciler) Delete() (reconcile.Result, error) {
	// prepare to write a new reconciliation status
	r.instance.Status.RemoveCondition(istiov1alpha3.ConditionTypeReconciled)
	err := r.prune(-1)
	updateDeleteStatus(&r.status.StatusType, err)

	r.instance.Status = *r.status
	updateErr := r.Client.Status().Update(context.TODO(), r.instance)
	if updateErr != nil && !errors.IsGone(updateErr) {
		r.Log.Error(updateErr, "error updating ControlPlane status for object", "object", r.instance.GetName())
		if err == nil {
			// XXX: is this the right thing to do?
			return reconcile.Result{}, updateErr
		}
	}
	return reconcile.Result{}, err
}
