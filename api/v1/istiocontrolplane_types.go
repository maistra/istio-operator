/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const IstioControlPlaneKind = "IstioControlPlane"

// IstioControlPlaneSpec defines the desired state of IstioControlPlane
type IstioControlPlaneSpec struct {
	Version string `json:"version,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Values json.RawMessage `json:"values,omitempty"`
}

func (s *IstioControlPlaneSpec) GetValues() map[string]interface{} {
	var vals map[string]interface{}
	err := json.Unmarshal(s.Values, &vals)
	if err != nil {
		return nil
	}
	return vals
}

func (s *IstioControlPlaneSpec) SetValues(values map[string]interface{}) error {
	jsonVals, err := json.Marshal(values)
	if err != nil {
		return err
	}
	s.Values = jsonVals
	return nil
}

// IstioControlPlaneStatus defines the observed state of IstioControlPlane
type IstioControlPlaneStatus struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	AppliedValues json.RawMessage `json:"appliedValues,omitempty"`
}

func (s *IstioControlPlaneStatus) GetAppliedValues() map[string]interface{} {
	var vals map[string]interface{}
	err := json.Unmarshal(s.AppliedValues, &vals)
	if err != nil {
		return nil
	}
	return vals
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// IstioControlPlane is the Schema for the istiocontrolplanes API
type IstioControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IstioControlPlaneSpec   `json:"spec,omitempty"`
	Status IstioControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IstioControlPlaneList contains a list of IstioControlPlane
type IstioControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IstioControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IstioControlPlane{}, &IstioControlPlaneList{})
}
