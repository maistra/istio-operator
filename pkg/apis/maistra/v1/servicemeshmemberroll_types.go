package v1

import (
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

func init() {
	SchemeBuilder.Register(&ServiceMeshMemberRoll{}, &ServiceMeshMemberRollList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceMeshMemberRoll is the Schema for the servicemeshmemberrolls API
// +k8s:openapi-gen=true
type ServiceMeshMemberRoll struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceMeshMemberRollSpec   `json:"spec,omitempty"`
	Status ServiceMeshMemberRollStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceMeshMemberRollList contains a list of ServiceMeshMemberRoll
type ServiceMeshMemberRollList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceMeshMemberRoll `json:"items"`
}

// ServiceMeshMemberRollSpec defines the members of the mesh
type ServiceMeshMemberRollSpec struct {
	Members []string `json:"members,omitempty"`
}

// ServiceMeshMemberRollStatus contains the state last used to reconcile the list
type ServiceMeshMemberRollStatus struct {
	StatusBase `json:",inline"`

	ObservedGeneration           int64    `json:"observedGeneration,omitempty"`
	ServiceMeshGeneration        int64    `json:"meshGeneration,omitempty"`
	ServiceMeshReconciledVersion string   `json:"meshReconciledVersion,omitempty"`
	ConfiguredMembers            []string `json:"configuredMembers,omitempty"`
	MeshVersion                  string   `json:"meshVersion,omitempty"`

	// Represents the latest available observations of a ServiceMeshMemberRoll's current state.
	Conditions []ServiceMeshMemberRollCondition `json:"conditions"`
}

// ServiceMeshMemberRollConditionType represents the type of the condition.  Condition types are:
// Reconciled, NamespaceConfigured
type ServiceMeshMemberRollConditionType ConditionType

const (
	// ConditionTypeMemberRollReady signifies whether the namespace has been configured
	// to use the mesh
	ConditionTypeMemberRollReady ServiceMeshMemberRollConditionType = "Ready"
)

type ServiceMeshMemberRollConditionReason string

const (
	// ConditionReasonConfigured indicates that all namespaces were configured
	ConditionReasonConfigured ServiceMeshMemberRollConditionReason = "Configured"
	// ConditionReasonNamespaceMissing indicates that one of the namespaces to configure does not exist
	ConditionReasonNamespaceMissing ServiceMeshMemberRollConditionReason = "ErrNamespaceMissing"
)

// Condition represents a specific condition on a resource
type ServiceMeshMemberRollCondition struct {
	Type               ServiceMeshMemberRollConditionType   `json:"type,omitempty"`
	Status             core.ConditionStatus                 `json:"status,omitempty"`
	LastTransitionTime metav1.Time                          `json:"lastTransitionTime,omitempty"`
	Reason             ServiceMeshMemberRollConditionReason `json:"reason,omitempty"`
	Message            string                               `json:"message,omitempty"`
}

// GetCondition removes a condition for the list of conditions
func (s *ServiceMeshMemberRollStatus) GetCondition(conditionType ServiceMeshMemberRollConditionType) ServiceMeshMemberRollCondition {
	if s == nil {
		return ServiceMeshMemberRollCondition{Type: conditionType, Status: core.ConditionUnknown}
	}
	for i := range s.Conditions {
		if s.Conditions[i].Type == conditionType {
			return s.Conditions[i]
		}
	}
	return ServiceMeshMemberRollCondition{Type: conditionType, Status: core.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *ServiceMeshMemberRollStatus) SetCondition(condition ServiceMeshMemberRollCondition) *ServiceMeshMemberRollStatus {
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
