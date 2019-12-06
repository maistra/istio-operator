package v1

import (
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

func init() {
	SchemeBuilder.Register(&ServiceMeshMember{}, &ServiceMeshMemberList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceMeshMember is the Schema for the servicemeshmembers API
// +k8s:openapi-gen=true
type ServiceMeshMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceMeshMemberSpec   `json:"spec,omitempty"`
	Status ServiceMeshMemberStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceMeshMemberList contains a list of ServiceMeshMember objects
type ServiceMeshMemberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceMeshMember `json:"items"`
}

// ServiceMeshMemberSpec defines the members of the mesh
type ServiceMeshMemberSpec struct {
	ControlPlaneRef ServiceMeshControlPlaneRef `json:"controlPlaneRef"`
}

// ServiceMeshControlPlaneRef is a reference to a ServiceMeshControlPlane object
type ServiceMeshControlPlaneRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// ServiceMeshMemberStatus contains information on whether the Member has been reconciled or not
type ServiceMeshMemberStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration"`

	// Represents the latest available observations of a ServiceMeshMember's current state.
	Conditions []ServiceMeshMemberCondition `json:"conditions"`
}

// ServiceMeshMemberConditionType represents the type of the condition.  Condition types are:
// Reconciled, NamespaceConfigured
type ServiceMeshMemberConditionType ConditionType

const (
	// ConditionTypeReconciled signifies whether or not the controller has
	// updated the ServiceMeshMemberRoll object based on this ServiceMeshMember.
	ConditionTypeMemberReconciled ServiceMeshMemberConditionType = "Reconciled"
	// ConditionTypeReady signifies whether the namespace has been configured
	// to use the mesh
	ConditionTypeMemberReady ServiceMeshMemberConditionType = "Ready"
)

type ServiceMeshMemberConditionReason string

const (
	// ConditionReasonDeletionError ...
	ConditionReasonMemberCannotCreateMemberRoll ServiceMeshMemberConditionReason = "CreateMemberRollFailed"
	ConditionReasonMemberCannotUpdateMemberRoll ServiceMeshMemberConditionReason = "UpdateMemberRollFailed"
	ConditionReasonMemberCannotDeleteMemberRoll ServiceMeshMemberConditionReason = "DeleteMemberRollFailed"
)

// Condition represents a specific condition on a resource
type ServiceMeshMemberCondition struct {
	Type               ServiceMeshMemberConditionType   `json:"type,omitempty"`
	Status             core.ConditionStatus             `json:"status,omitempty"`
	LastTransitionTime metav1.Time                      `json:"lastTransitionTime,omitempty"`
	Reason             ServiceMeshMemberConditionReason `json:"reason,omitempty"`
	Message            string                           `json:"message,omitempty"`
}

// GetCondition removes a condition for the list of conditions
func (s *ServiceMeshMemberStatus) GetCondition(conditionType ServiceMeshMemberConditionType) ServiceMeshMemberCondition {
	if s == nil {
		return ServiceMeshMemberCondition{Type: conditionType, Status: core.ConditionUnknown}
	}
	for i := range s.Conditions {
		if s.Conditions[i].Type == conditionType {
			return s.Conditions[i]
		}
	}
	return ServiceMeshMemberCondition{Type: conditionType, Status: core.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *ServiceMeshMemberStatus) SetCondition(condition ServiceMeshMemberCondition) *ServiceMeshMemberStatus {
	if s == nil {
		return nil
	}
	now := metav1.Now()
	for i := range s.Conditions {
		if s.Conditions[i].Type == condition.Type {
			if s.Conditions[i].Status != condition.Status {
				condition.LastTransitionTime = now
			} else {
				condition.LastTransitionTime = s.Conditions[i].LastTransitionTime
			}
			s.Conditions[i] = condition
			return s
		}
	}

	// If the condition does not exist,
	// initialize the lastTransitionTime
	condition.LastTransitionTime = now
	s.Conditions = append(s.Conditions, condition)
	return s
}
