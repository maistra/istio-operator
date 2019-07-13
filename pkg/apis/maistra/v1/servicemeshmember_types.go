package v1

import (
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

// ServiceMeshMemberStatus contains the state last used to reconcile the list
type ServiceMeshMemberStatus struct {
}
