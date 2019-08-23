package controlplane

import (
	"github.com/maistra/istio-operator/pkg/apis/maistra/v1"
)

func updateReconcileStatus(status *v1.StatusType, err error) {
	installStatus := status.GetCondition(v1.ConditionTypeInstalled).Status
	if err == nil {
		if installStatus != v1.ConditionStatusTrue {
			status.SetCondition(v1.Condition{
				Type:    v1.ConditionTypeInstalled,
				Reason:  v1.ConditionReasonInstallSuccessful,
				Message: "Successfully installed all mesh components",
				Status:  v1.ConditionStatusTrue,
			})
			status.SetCondition(v1.Condition{
				Type:    v1.ConditionTypeReconciled,
				Reason:  v1.ConditionReasonInstallSuccessful,
				Message: "Successfully installed all mesh components",
				Status:  v1.ConditionStatusTrue,
			})
		} else {
			status.SetCondition(v1.Condition{
				Type:    v1.ConditionTypeReconciled,
				Reason:  v1.ConditionReasonReconcileSuccessful,
				Message: "Successfully reconciled",
				Status:  v1.ConditionStatusTrue,
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
