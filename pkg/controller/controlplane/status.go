package controlplane

import (
	istiov1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/v1alpha3"

	"k8s.io/apimachinery/pkg/api/errors"
)

func updateReconcileStatus(status *istiov1alpha3.StatusType, err error) {
	installStatus := status.GetCondition(istiov1alpha3.ConditionTypeInstalled).Status
	if err == nil {
		if installStatus != istiov1alpha3.ConditionStatusTrue {
			status.SetCondition(istiov1alpha3.Condition{
				Type:   istiov1alpha3.ConditionTypeInstalled,
				Reason: istiov1alpha3.ConditionReasonInstallSuccessful,
				Status: istiov1alpha3.ConditionStatusTrue,
			})
			status.SetCondition(istiov1alpha3.Condition{
				Type:   istiov1alpha3.ConditionTypeReconciled,
				Reason: istiov1alpha3.ConditionReasonInstallSuccessful,
				Status: istiov1alpha3.ConditionStatusTrue,
			})
		} else {
			status.SetCondition(istiov1alpha3.Condition{
				Type:   istiov1alpha3.ConditionTypeReconciled,
				Reason: istiov1alpha3.ConditionReasonReconcileSuccessful,
				Status: istiov1alpha3.ConditionStatusTrue,
			})
		}
	} else if installStatus == istiov1alpha3.ConditionStatusUnknown {
		status.SetCondition(istiov1alpha3.Condition{
			Type:    istiov1alpha3.ConditionTypeInstalled,
			Reason:  istiov1alpha3.ConditionReasonInstallError,
			Status:  istiov1alpha3.ConditionStatusFalse,
			Message: err.Error(),
		})
		status.SetCondition(istiov1alpha3.Condition{
			Type:    istiov1alpha3.ConditionTypeReconciled,
			Reason:  istiov1alpha3.ConditionReasonInstallError,
			Status:  istiov1alpha3.ConditionStatusFalse,
			Message: err.Error(),
		})
	} else {
		status.SetCondition(istiov1alpha3.Condition{
			Type:    istiov1alpha3.ConditionTypeReconciled,
			Reason:  istiov1alpha3.ConditionReasonReconcileError,
			Status:  istiov1alpha3.ConditionStatusFalse,
			Message: err.Error(),
		})
	}
}

func updateDeleteStatus(status *istiov1alpha3.StatusType, err error) {
	if err == nil || errors.IsNotFound(err) || errors.IsGone(err) {
		status.SetCondition(istiov1alpha3.Condition{
			Type:   istiov1alpha3.ConditionTypeInstalled,
			Status: istiov1alpha3.ConditionStatusFalse,
			Reason: istiov1alpha3.ConditionReasonDeletionSuccessful,
		})
		status.SetCondition(istiov1alpha3.Condition{
			Type:   istiov1alpha3.ConditionTypeReconciled,
			Status: istiov1alpha3.ConditionStatusTrue,
			Reason: istiov1alpha3.ConditionReasonDeletionSuccessful,
		})
	} else {
		status.SetCondition(istiov1alpha3.Condition{
			Type:    istiov1alpha3.ConditionTypeReconciled,
			Status:  istiov1alpha3.ConditionStatusFalse,
			Reason:  istiov1alpha3.ConditionReasonDeletionError,
			Message: err.Error(),
		})
	}
}
