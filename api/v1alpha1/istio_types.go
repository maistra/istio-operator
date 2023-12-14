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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"maistra.io/istio-operator/pkg/helm"
)

const IstioKind = "Istio"

// IstioSpec defines the desired state of Istio
type IstioSpec struct {
	// +sail:version
	// Defines the version of Istio to install.
	// Must be one of: v1.20.0, v1.19.4, latest.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName="Istio Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:v1.20.0", "urn:alm:descriptor:com.tectonic.ui:select:v1.19.4", "urn:alm:descriptor:com.tectonic.ui:select:latest"}
	// +kubebuilder:validation:Enum=v1.20.0;v1.19.4;latest
	Version string `json:"version"`

	// +sail:profile
	// The built-in installation configuration profile to use.
	// The 'default' profile is always applied. On OpenShift, the 'openshift' profile is also applied on top of 'default'.
	// Must be one of: ambient, default, demo, empty, external, minimal, openshift, preview, remote.
	// +++PROFILES-DROPDOWN-HIDDEN-UNTIL-WE-FULLY-IMPLEMENT-THEM+++operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Profile",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:ambient", "urn:alm:descriptor:com.tectonic.ui:select:default", "urn:alm:descriptor:com.tectonic.ui:select:demo", "urn:alm:descriptor:com.tectonic.ui:select:empty", "urn:alm:descriptor:com.tectonic.ui:select:external", "urn:alm:descriptor:com.tectonic.ui:select:minimal", "urn:alm:descriptor:com.tectonic.ui:select:preview", "urn:alm:descriptor:com.tectonic.ui:select:remote"}
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	// +kubebuilder:validation:Enum=ambient;default;demo;empty;external;minimal;openshift;preview;remote
	Profile string `json:"profile,omitempty"`

	// Defines the values to be passed to the Helm charts when installing Istio.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Helm Values"
	Values json.RawMessage `json:"values,omitempty"`

	// Defines the non-validated values to be passed to the Helm charts when installing Istio.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Helm RawValues"
	RawValues json.RawMessage `json:"rawValues,omitempty"`
}

func (s *IstioSpec) GetValues() helm.HelmValues {
	return toHelmValues(s.Values)
}

func (s *IstioSpec) GetRawValues() helm.HelmValues {
	return toHelmValues(s.RawValues)
}

func (s *IstioSpec) SetValues(values helm.HelmValues) error {
	jsonVals, err := json.Marshal(values)
	if err != nil {
		return err
	}
	s.Values = jsonVals
	return nil
}

// IstioStatus defines the observed state of Istio
type IstioStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// Istio object. It corresponds to the object's generation, which is
	// updated on mutation by the API Server. The information in the status
	// pertains to this particular generation of the object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of the object's current state.
	Conditions []IstioCondition `json:"conditions,omitempty"`

	// Reports the current state of the object.
	State IstioConditionReason `json:"state,omitempty"`
}

// GetCondition returns the condition of the specified type
func (s *IstioStatus) GetCondition(conditionType IstioConditionType) IstioCondition {
	if s != nil {
		for i := range s.Conditions {
			if s.Conditions[i].Type == conditionType {
				return s.Conditions[i]
			}
		}
	}
	return IstioCondition{Type: conditionType, Status: metav1.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *IstioStatus) SetCondition(condition IstioCondition) {
	var now time.Time
	if testTime == nil {
		now = time.Now()
	} else {
		now = *testTime
	}

	// The lastTransitionTime only gets serialized out to the second.  This can
	// break update skipping, as the time in the resource returned from the client
	// may not match the time in our cached status during a reconcile.  We truncate
	// here to save any problems down the line.
	lastTransitionTime := metav1.NewTime(now.Truncate(time.Second))

	for i, prevCondition := range s.Conditions {
		if prevCondition.Type == condition.Type {
			if prevCondition.Status != condition.Status {
				condition.LastTransitionTime = lastTransitionTime
			} else {
				condition.LastTransitionTime = prevCondition.LastTransitionTime
			}
			s.Conditions[i] = condition
			return
		}
	}

	// If the condition does not exist, initialize the lastTransitionTime
	condition.LastTransitionTime = lastTransitionTime
	s.Conditions = append(s.Conditions, condition)
}

// A Condition represents a specific observation of the object's state.
type IstioCondition struct {
	// The type of this condition.
	Type IstioConditionType `json:"type,omitempty"`

	// The status of this condition. Can be True, False or Unknown.
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// Unique, single-word, CamelCase reason for the condition's last transition.
	Reason IstioConditionReason `json:"reason,omitempty"`

	// Human-readable message indicating details about the last transition.
	Message string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// IstioConditionType represents the type of the condition.  Condition stages are:
// Installed, Reconciled, Ready
type IstioConditionType string

// IstioConditionReason represents a short message indicating how the condition came
// to be in its present state.
type IstioConditionReason string

const (
	// IstioConditionTypeReconciled signifies whether the controller has
	// successfully reconciled the resources defined through the CR.
	IstioConditionTypeReconciled IstioConditionType = "Reconciled"

	// IstioConditionReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.
	IstioConditionReasonReconcileError IstioConditionReason = "ReconcileError"
)

const (
	// IstioConditionTypeReady signifies whether any Deployment, StatefulSet,
	// etc. resources are Ready.
	IstioConditionTypeReady IstioConditionType = "Ready"

	// IstioConditionReasonIstioRevisionNotFound indicates that the active IstioRevision is not found.
	IstioConditionReasonIstioRevisionNotFound IstioConditionReason = "ActiveRevisionNotFound"

	// IstioConditionReasonIstiodNotReady indicates that the control plane is fully reconciled, but istiod is not ready.
	IstioConditionReasonIstiodNotReady IstioConditionReason = "IstiodNotReady"

	// IstioConditionReasonCNINotReady indicates that the control plane is fully reconciled, but istio-cni-node is not ready.
	IstioConditionReasonCNINotReady IstioConditionReason = "CNINotReady"
)

const (
	// IstioConditionReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.
	IstioConditionReasonHealthy IstioConditionReason = "Healthy"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=istio-io
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description="Whether the control plane installation is ready to handle requests."
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="The current state of this object."
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="The version of the control plane installation."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"

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
