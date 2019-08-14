package controlplane

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func (r *ControlPlaneReconciler) Delete() error {
	r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeNormal, eventReasonDeleting, "Deleting service mesh")
	err := r.prune(-1)
	defer func() {
		if err == nil {
			r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeNormal, eventReasonDeleted, "Successfully deleted service mesh components")
		} else {
			r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeWarning, eventReasonFailedDeletingComponents, fmt.Sprintf("Error occurred during service mesh deletion: %s", err))
		}
	}()
	return err
}
