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

func (s *IstioHelmInstallSpec) GetValues() map[string]interface{} {
	var vals map[string]interface{}
	err := json.Unmarshal(s.Values, &vals)
	if err != nil {
		return nil
	}
	return vals
}

// IstioHelmInstallSpec defines the desired state of IstioHelmInstall
type IstioHelmInstallSpec struct {
	Version string `json:"version,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Values json.RawMessage `json:"values,omitempty"`
}

// IstioHelmInstallStatus defines the observed state of IstioHelmInstall
type IstioHelmInstallStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// IstioHelmInstall is the Schema for the istiohelminstalls API
type IstioHelmInstall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IstioHelmInstallSpec   `json:"spec,omitempty"`
	Status IstioHelmInstallStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IstioHelmInstallList contains a list of IstioHelmInstall
type IstioHelmInstallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IstioHelmInstall `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IstioHelmInstall{}, &IstioHelmInstallList{})
}
