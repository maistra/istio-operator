package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/maistra/istio-operator/pkg/apis/external"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VirtualService struct {
	external.Base `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VirtualServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualService `json:"items"`
}
