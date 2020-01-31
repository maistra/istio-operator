package controlplane

import (
	"context"
	"fmt"

	errors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/hacks"
)

func (r *controlPlaneInstanceReconciler) Delete(ctx context.Context) error {
	reconciledCondition := r.Status.GetCondition(maistrav1.ConditionTypeReconciled)
	if reconciledCondition.Status != maistrav1.ConditionStatusFalse || reconciledCondition.Reason != maistrav1.ConditionReasonDeleting {
		r.Status.SetCondition(maistrav1.Condition{
			Type:    maistrav1.ConditionTypeReconciled,
			Status:  maistrav1.ConditionStatusFalse,
			Reason:  maistrav1.ConditionReasonDeleting,
			Message: "Deleting service mesh",
		})
		r.Status.SetCondition(maistrav1.Condition{
			Type:    maistrav1.ConditionTypeReady,
			Status:  maistrav1.ConditionStatusFalse,
			Reason:  maistrav1.ConditionReasonDeleting,
			Message: "Deleting service mesh",
		})

		err := r.PostStatus(ctx)
		return err // return regardless of error; deletion will continue when update event comes back into the operator
	}

	r.Log.Info("Deleting ServiceMeshControlPlane")

	r.EventRecorder.Event(r.Instance, corev1.EventTypeNormal, eventReasonDeleting, "Deleting service mesh")
	err := r.prune(ctx, "")
	if err == nil {
		r.EventRecorder.Event(r.Instance, corev1.EventTypeNormal, eventReasonDeleted, "Successfully deleted service mesh resources")
	} else {
		r.EventRecorder.Event(r.Instance, corev1.EventTypeWarning, eventReasonFailedDeletingResources, fmt.Sprintf("Error deleting service mesh resources: %s", err))
	}

	if err == nil {
		// set reconcile status to true to ensure reconciler is deleted from the cache
		r.Status.SetCondition(maistrav1.Condition{
			Type:    maistrav1.ConditionTypeReconciled,
			Status:  maistrav1.ConditionStatusTrue,
			Reason:  maistrav1.ConditionReasonDeleted,
			Message: "Service mesh deleted",
		})
	} else {
		r.Status.SetCondition(maistrav1.Condition{
			Type:    maistrav1.ConditionTypeReconciled,
			Status:  maistrav1.ConditionStatusFalse,
			Reason:  maistrav1.ConditionReasonDeletionError,
			Message: fmt.Sprintf("Error deleting service mesh: %s", err),
		})
		statusErr := r.PostStatus(ctx)
		if statusErr != nil {
			// we must return the original error, thus we can only log the status update error
			r.Log.Error(statusErr, "Error updating status")
		}
		return err
	}

	// get fresh SMCP from cache to minimize the chance of a conflict during update (the SMCP might have been updated during the execution of reconciler.Delete())
	instance := &maistrav1.ServiceMeshControlPlane{}
	if err := r.Client.Get(ctx, common.ToNamespacedName(r.Instance.ObjectMeta), instance); err == nil {
		finalizers := sets.NewString(instance.Finalizers...)
		finalizers.Delete(common.FinalizerName)
		instance.SetFinalizers(finalizers.List())
		if err := r.Client.Update(ctx, instance); err == nil {
			r.Log.Info("Removed finalizer")
			hacks.ReduceLikelihoodOfRepeatedReconciliation()
		} else if !(apierrors.IsGone(err) || apierrors.IsNotFound(err)) {
			r.EventRecorder.Event(instance, corev1.EventTypeWarning, eventReasonFailedRemovingFinalizer, fmt.Sprintf("Error occurred removing finalizer from service mesh: %s", err)) // TODO: this event probably isn't needed at all
			return errors.Wrap(err, "Error removing ServiceMeshControlPlane finalizer")
		}
	} else if !(apierrors.IsGone(err) || apierrors.IsNotFound(err)) {
		r.EventRecorder.Event(instance, corev1.EventTypeWarning, eventReasonFailedRemovingFinalizer, fmt.Sprintf("Error occurred removing finalizer from service mesh: %s", err))
		return errors.Wrap(err, "Error getting ServiceMeshControlPlane prior to removing finalizer")
	}

	return nil
}
