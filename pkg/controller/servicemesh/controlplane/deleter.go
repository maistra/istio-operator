package controlplane

import (
	"context"
	"fmt"

	errors2 "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/hacks"
)

func (r *ControlPlaneReconciler) Delete() error {
	reconciledCondition := r.Status.GetCondition(v1.ConditionTypeReconciled)
	if reconciledCondition.Status != v1.ConditionStatusFalse || reconciledCondition.Reason != v1.ConditionReasonDeleting {
		r.Status.SetCondition(v1.Condition{
			Type:    v1.ConditionTypeReconciled,
			Status:  v1.ConditionStatusFalse,
			Reason:  v1.ConditionReasonDeleting,
			Message: "Deleting service mesh",
		})
		r.Status.SetCondition(v1.Condition{
			Type:    v1.ConditionTypeReady,
			Status:  v1.ConditionStatusFalse,
			Reason:  v1.ConditionReasonDeleting,
			Message: "Deleting service mesh",
		})

		err := r.PostStatus()
		return err // return regardless of error; deletion will continue when update event comes back into the operator
	}

	r.Log.Info("Deleting ServiceMeshControlPlane")

	r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeNormal, eventReasonDeleting, "Deleting service mesh")
	err := r.prune(-1)
	if err != nil {
		r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeWarning, eventReasonFailedDeletingResources, fmt.Sprintf("Error deleting service mesh resources: %s", err))
		r.Status.SetCondition(v1.Condition{
			Type:    v1.ConditionTypeReconciled,
			Status:  v1.ConditionStatusFalse,
			Reason:  v1.ConditionReasonDeletionError,
			Message: fmt.Sprintf("Error deleting service mesh: %s", err),
		})
		statusErr := r.PostStatus()
		if statusErr != nil {
			// we must return the original error, thus we can only log the status update error
			r.Log.Error(statusErr, "Error updating status")
		}
		return err
	}

	r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeNormal, eventReasonDeleted, "Successfully deleted service mesh resources")

	// get fresh SMCP from cache to minimize the chance of a conflict during update (the SMCP might have been updated during the execution of r.prune())
	instance := &v1.ServiceMeshControlPlane{}
	if err := r.Client.Get(context.TODO(), client.ObjectKey{r.Instance.Namespace, r.Instance.Name}, instance); err == nil {
		finalizers := sets.NewString(instance.Finalizers...)
		finalizers.Delete(common.FinalizerName)
		instance.SetFinalizers(finalizers.List())
		if err := r.Client.Update(context.TODO(), instance); err == nil {
			r.Log.Info("Removed finalizer")
			hacks.ReduceLikelihoodOfRepeatedReconciliation()
		} else if !(errors.IsGone(err) || errors.IsNotFound(err)) {
			r.Manager.GetRecorder(controllerName).Event(instance, corev1.EventTypeWarning, eventReasonFailedRemovingFinalizer, fmt.Sprintf("Error occurred removing finalizer from service mesh: %s", err)) // TODO: this event probably isn't needed at all
			return errors2.Wrap(err, "Error removing ServiceMeshControlPlane finalizer")
		}
	} else if !(errors.IsGone(err) || errors.IsNotFound(err)) {
		r.Manager.GetRecorder(controllerName).Event(instance, corev1.EventTypeWarning, eventReasonFailedRemovingFinalizer, fmt.Sprintf("Error occurred removing finalizer from service mesh: %s", err))
		return errors2.Wrap(err, "Error getting ServiceMeshControlPlane prior to removing finalizer")
	}

	return nil
}
