package controlplane

import (
	"context"
	"fmt"

	errors2 "github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	maistra "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/hacks"
)

func (r *ControlPlaneReconciler) Delete() error {
	reconciledCondition := r.Status.GetCondition(maistra.ConditionTypeReconciled)
	if reconciledCondition.Status != maistra.ConditionStatusFalse || reconciledCondition.Reason != maistra.ConditionReasonDeleting {
		r.Status.SetCondition(maistra.Condition{
			Type:    maistra.ConditionTypeReconciled,
			Status:  maistra.ConditionStatusFalse,
			Reason:  maistra.ConditionReasonDeleting,
			Message: "Deleting service mesh",
		})
		r.Status.SetCondition(maistra.Condition{
			Type:    maistra.ConditionTypeReady,
			Status:  maistra.ConditionStatusFalse,
			Reason:  maistra.ConditionReasonDeleting,
			Message: "Deleting service mesh",
		})

		err := r.PostStatus()
		return err // return regardless of error; deletion will continue when update event comes back into the operator
	}

	r.Log.Info("Deleting ServiceMeshControlPlane")

	r.EventRecorder.Event(r.Instance, core.EventTypeNormal, eventReasonDeleting, "Deleting service mesh")
	err := r.prune("")
	if err == nil {
		r.EventRecorder.Event(r.Instance, core.EventTypeNormal, eventReasonDeleted, "Successfully deleted service mesh resources")
	} else {
		r.EventRecorder.Event(r.Instance, core.EventTypeWarning, eventReasonFailedDeletingResources, fmt.Sprintf("Error deleting service mesh resources: %s", err))
	}

	if err == nil {
		// set reconcile status to true to ensure reconciler is deleted from the cache
		r.Status.SetCondition(maistra.Condition{
			Type:    maistra.ConditionTypeReconciled,
			Status:  maistra.ConditionStatusTrue,
			Reason:  maistra.ConditionReasonDeleted,
			Message: "Service mesh deleted",
		})
	} else {
		r.Status.SetCondition(maistra.Condition{
			Type:    maistra.ConditionTypeReconciled,
			Status:  maistra.ConditionStatusFalse,
			Reason:  maistra.ConditionReasonDeletionError,
			Message: fmt.Sprintf("Error deleting service mesh: %s", err),
		})
		statusErr := r.PostStatus()
		if statusErr != nil {
			// we must return the original error, thus we can only log the status update error
			r.Log.Error(statusErr, "Error updating status")
		}
		return err
	}

	// get fresh SMCP from cache to minimize the chance of a conflict during update (the SMCP might have been updated during the execution of reconciler.Delete())
	instance := &maistra.ServiceMeshControlPlane{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{r.Instance.Namespace, r.Instance.Name}, instance); err == nil {
		finalizers := sets.NewString(instance.Finalizers...)
		finalizers.Delete(common.FinalizerName)
		instance.SetFinalizers(finalizers.List())
		if err := r.Client.Update(context.TODO(), instance); err == nil {
			r.Log.Info("Removed finalizer")
			hacks.ReduceLikelihoodOfRepeatedReconciliation()
		} else if !(errors.IsGone(err) || errors.IsNotFound(err)) {
			r.EventRecorder.Event(instance, core.EventTypeWarning, eventReasonFailedRemovingFinalizer, fmt.Sprintf("Error occurred removing finalizer from service mesh: %s", err)) // TODO: this event probably isn't needed at all
			return errors2.Wrap(err, "Error removing ServiceMeshControlPlane finalizer")
		}
	} else if !(errors.IsGone(err) || errors.IsNotFound(err)) {
		r.EventRecorder.Event(instance, core.EventTypeWarning, eventReasonFailedRemovingFinalizer, fmt.Sprintf("Error occurred removing finalizer from service mesh: %s", err))
		return errors2.Wrap(err, "Error getting ServiceMeshControlPlane prior to removing finalizer")
	}

	return nil
}
