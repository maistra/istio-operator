package controlplane

import (
	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
)

func updateControlPlaneConditions(s *v1.ControlPlaneStatus, err error) {
	updateConditions(&s.StatusType, err, "Successfully installed all mesh components")
}

func updateComponentConditions(s *status.ComponentStatus, err error) {
	updateConditions(&s.StatusType, err, "Component installed successfully")
}

func updateConditions(s *status.StatusType, err error, successfulInstallMessage string) {
	installStatus := s.GetCondition(status.ConditionTypeInstalled).Status // TODO: controller should never read the status to decide what to do
	if err == nil {
		if installStatus != status.ConditionStatusTrue {
			s.SetCondition(status.Condition{
				Type:    status.ConditionTypeInstalled,
				Reason:  status.ConditionReasonInstallSuccessful,
				Message: successfulInstallMessage,
				Status:  status.ConditionStatusTrue,
			})
			s.SetCondition(status.Condition{
				Type:    status.ConditionTypeReconciled,
				Reason:  status.ConditionReasonInstallSuccessful,
				Message: successfulInstallMessage,
				Status:  status.ConditionStatusTrue,
			})
		} else {
			s.SetCondition(status.Condition{
				Type:    status.ConditionTypeReconciled,
				Reason:  status.ConditionReasonReconcileSuccessful,
				Message: "Successfully reconciled",
				Status:  status.ConditionStatusTrue,
			})
		}
	} else if installStatus == status.ConditionStatusUnknown {
		s.SetCondition(status.Condition{
			Type:    status.ConditionTypeInstalled,
			Reason:  status.ConditionReasonInstallError,
			Status:  status.ConditionStatusFalse,
			Message: err.Error(),
		})
		s.SetCondition(status.Condition{
			Type:    status.ConditionTypeReconciled,
			Reason:  status.ConditionReasonInstallError,
			Status:  status.ConditionStatusFalse,
			Message: err.Error(),
		})
	} else {
		s.SetCondition(status.Condition{
			Type:    status.ConditionTypeReconciled,
			Reason:  status.ConditionReasonReconcileError,
			Status:  status.ConditionStatusFalse,
			Message: err.Error(),
		})
	}
}
