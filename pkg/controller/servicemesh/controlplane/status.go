package controlplane

import (
	"github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	"k8s.io/apimachinery/pkg/api/errors"
)

func updateReconcileStatus(status *v1.StatusType, err error) {
	installStatus := status.GetCondition(v1.ConditionTypeInstalled).Status
	if err == nil {
		if installStatus != v1.ConditionStatusTrue {
			status.SetCondition(v1.Condition{
				Type:   v1.ConditionTypeInstalled,
				Reason: v1.ConditionReasonInstallSuccessful,
				Status: v1.ConditionStatusTrue,
			})
			status.SetCondition(v1.Condition{
				Type:   v1.ConditionTypeReconciled,
				Reason: v1.ConditionReasonInstallSuccessful,
				Status: v1.ConditionStatusTrue,
			})
		} else {
			status.SetCondition(v1.Condition{
				Type:   v1.ConditionTypeReconciled,
				Reason: v1.ConditionReasonReconcileSuccessful,
				Status: v1.ConditionStatusTrue,
			})
		}
	} else if installStatus == v1.ConditionStatusUnknown {
		status.SetCondition(v1.Condition{
			Type:    v1.ConditionTypeInstalled,
			Reason:  v1.ConditionReasonInstallError,
			Status:  v1.ConditionStatusFalse,
			Message: err.Error(),
		})
		status.SetCondition(v1.Condition{
			Type:    v1.ConditionTypeReconciled,
			Reason:  v1.ConditionReasonInstallError,
			Status:  v1.ConditionStatusFalse,
			Message: err.Error(),
		})
	} else {
		status.SetCondition(v1.Condition{
			Type:    v1.ConditionTypeReconciled,
			Reason:  v1.ConditionReasonReconcileError,
			Status:  v1.ConditionStatusFalse,
			Message: err.Error(),
		})
	}
}

func updateDeleteStatus(status *v1.StatusType, err error) {
	if err == nil || errors.IsNotFound(err) || errors.IsGone(err) {
		status.SetCondition(v1.Condition{
			Type:   v1.ConditionTypeInstalled,
			Status: v1.ConditionStatusFalse,
			Reason: v1.ConditionReasonDeletionSuccessful,
		})
		status.SetCondition(v1.Condition{
			Type:   v1.ConditionTypeReconciled,
			Status: v1.ConditionStatusTrue,
			Reason: v1.ConditionReasonDeletionSuccessful,
		})
	} else {
		status.SetCondition(v1.Condition{
			Type:    v1.ConditionTypeReconciled,
			Status:  v1.ConditionStatusFalse,
			Reason:  v1.ConditionReasonDeletionError,
			Message: err.Error(),
		})
	}
}
