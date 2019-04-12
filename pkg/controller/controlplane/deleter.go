package controlplane

import (
	"context"

	istiov1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/v1alpha3"

	"k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *controlPlaneReconciler) Delete() (reconcile.Result, error) {
	allErrors := []error{}
	// prepare to write a new reconciliation status
	r.instance.Status.RemoveCondition(istiov1alpha3.ConditionTypeReconciled)
	// ensure ComponentStatus is ready
	if r.instance.Status.ComponentStatus == nil {
		r.instance.Status.ComponentStatus = []*istiov1alpha3.ComponentStatus{}
	}
	for index := len(r.instance.Status.ComponentStatus) - 1; index >= 0; index-- {
		status := r.instance.Status.ComponentStatus[index]
		err := r.processComponentManifests(status.Resource)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	r.status.ObservedGeneration = r.instance.GetGeneration()
	err := utilerrors.NewAggregate(allErrors)
	updateDeleteStatus(&r.status.StatusType, err)

	r.instance.Status = *r.status
	updateErr := r.client.Status().Update(context.TODO(), r.instance)
	if updateErr != nil && !errors.IsGone(updateErr) {
		r.log.Error(err, "error updating ControlPlane status for object", "object", r.instance.GetName())
		if err == nil {
			// XXX: is this the right thing to do?
			return reconcile.Result{}, updateErr
		}
	}
	return reconcile.Result{}, err
}
