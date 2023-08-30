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

package v1alpha1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const IstioKind = "Istio"

// IstioSpec defines the desired state of Istio
type IstioSpec struct {
	Version string `json:"version,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Values json.RawMessage `json:"values,omitempty"`
}

func (s *IstioSpec) GetValues() map[string]interface{} {
	var vals map[string]interface{}
	err := json.Unmarshal(s.Values, &vals)
	if err != nil {
		return nil
	}
	return vals
}

func (s *IstioSpec) SetValues(values map[string]interface{}) error {
	jsonVals, err := json.Marshal(values)
	if err != nil {
		return err
	}
	s.Values = jsonVals
	return nil
}

// IstioStatus defines the observed state of Istio
type IstioStatus struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	AppliedValues json.RawMessage `json:"appliedValues,omitempty"`
}

func (s *IstioStatus) GetAppliedValues() map[string]interface{} {
	var vals map[string]interface{}
	err := json.Unmarshal(s.AppliedValues, &vals)
	if err != nil {
		return nil
	}
	return vals
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Istio represents an Istio Service Mesh deployment
type Istio struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IstioSpec   `json:"spec,omitempty"`
	Status IstioStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IstioList contains a list of Istio
type IstioList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Istio `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Istio{}, &IstioList{})
}
