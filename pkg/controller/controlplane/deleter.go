package controlplane

import (
	istiov1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/v1alpha3"

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

	return reconcile.Result{}, utilerrors.NewAggregate(allErrors)
}
