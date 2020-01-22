package controlplane

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

func (r *ControlPlaneReconciler) Delete() error {
	r.EventRecorder.Event(r.Instance, corev1.EventTypeNormal, eventReasonDeleting, "Deleting service mesh")
	err := r.prune("")
	if err == nil {
		r.EventRecorder.Event(r.Instance, corev1.EventTypeNormal, eventReasonDeleted, "Successfully deleted service mesh resources")
	} else {
		r.EventRecorder.Event(r.Instance, corev1.EventTypeWarning, eventReasonFailedDeletingResources, fmt.Sprintf("Error deleting service mesh resources: %s", err))
	}
	return err
}
