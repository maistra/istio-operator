package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

func init() {
	SchemeBuilder.Register(&ControlPlaneMember{}, &ControlPlaneMemberList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControlPlaneMember is the Schema for the controlplanemembers API
// +k8s:openapi-gen=true
type ControlPlaneMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status ControlPlaneMemberStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControlPlaneMemberList contains a list of ControlPlane
type ControlPlaneMemberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlaneMember `json:"items"`
}

type ControlPlaneMemberStatus struct {
	ObservedGeneration     int64    `json:"observedGeneration,omitempty"`
	MeshGeneration         int64    `json:"meshGeneration,omitempty"`
	ManagedServiceAccounts []string `json:"managedServiceAccounts,omitempty"`
	ManagedRoleBindings    []string `json:"managedRoleBindings,omitempty"`
}
