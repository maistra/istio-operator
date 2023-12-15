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

const IstioRevisionKind = "IstioRevision"

// IstioRevisionSpec defines the desired state of IstioRevision
type IstioRevisionSpec struct {
	// +sail:version
	// Defines the version of Istio to install.
	// Must be one of: v1.20.1, v1.20.0, v1.19.5, latest, gwAPIControllerMode.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName="Istio Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:v1.20.1", "urn:alm:descriptor:com.tectonic.ui:select:v1.20.0", "urn:alm:descriptor:com.tectonic.ui:select:v1.19.5", "urn:alm:descriptor:com.tectonic.ui:select:latest", "urn:alm:descriptor:com.tectonic.ui:select:gwAPIControllerMode"}
	// +kubebuilder:validation:Enum=v1.20.1;v1.20.0;v1.19.5;latest;gwAPIControllerMode
	Version string `json:"version"`

	// Namespace to which the Istio components should be installed.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:io.kubernetes:Namespace"}
	Namespace string `json:"namespace"`

	// Defines the values to be passed to the Helm charts when installing Istio.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Helm Values"
	Values json.RawMessage `json:"values,omitempty"`
}

func (s *IstioRevisionSpec) GetValues() helm.HelmValues {
	return toHelmValues(s.Values)
}

func (s *IstioRevisionSpec) SetValues(values helm.HelmValues) error {
	jsonVals, err := json.Marshal(values)
	if err != nil {
		return err
	}
	s.Values = jsonVals
	return nil
}

// IstioRevisionStatus defines the observed state of IstioRevision
type IstioRevisionStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// IstioRevision object. It corresponds to the object's generation, which is
	// updated on mutation by the API Server. The information in the status
	// pertains to this particular generation of the object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of the object's current state.
	Conditions []IstioRevisionCondition `json:"conditions,omitempty"`

	// Reports the current state of the object.
	State IstioRevisionConditionReason `json:"state,omitempty"`
}

// GetCondition returns the condition of the specified type
func (s *IstioRevisionStatus) GetCondition(conditionType IstioRevisionConditionType) IstioRevisionCondition {
	if s != nil {
		for i := range s.Conditions {
			if s.Conditions[i].Type == conditionType {
				return s.Conditions[i]
			}
		}
	}
	return IstioRevisionCondition{Type: conditionType, Status: metav1.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *IstioRevisionStatus) SetCondition(condition IstioRevisionCondition) {
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
type IstioRevisionCondition struct {
	// The type of this condition.
	Type IstioRevisionConditionType `json:"type,omitempty"`

	// The status of this condition. Can be True, False or Unknown.
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// Unique, single-word, CamelCase reason for the condition's last transition.
	Reason IstioRevisionConditionReason `json:"reason,omitempty"`

	// Human-readable message indicating details about the last transition.
	Message string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// IstioRevisionConditionType represents the type of the condition.  Condition stages are:
// Installed, Reconciled, Ready
type IstioRevisionConditionType string

// IstioRevisionConditionReason represents a short message indicating how the condition came
// to be in its present state.
type IstioRevisionConditionReason string

const (
	// IstioRevisionConditionTypeReconciled signifies whether the controller has
	// successfully reconciled the resources defined through the CR.
	IstioRevisionConditionTypeReconciled IstioRevisionConditionType = "Reconciled"

	// IstioRevisionConditionReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.
	IstioRevisionConditionReasonReconcileError IstioRevisionConditionReason = "ReconcileError"
)

const (
	// IstioRevisionConditionTypeReady signifies whether any Deployment, StatefulSet,
	// etc. resources are Ready.
	IstioRevisionConditionTypeReady IstioRevisionConditionType = "Ready"

	// IstioRevisionConditionReasonIstiodNotReady indicates that the control plane is fully reconciled, but istiod is not ready.
	IstioRevisionConditionReasonIstiodNotReady IstioRevisionConditionReason = "IstiodNotReady"

	// IstioRevisionConditionReasonCNINotReady indicates that the control plane is fully reconciled, but istio-cni-node is not ready.
	IstioRevisionConditionReasonCNINotReady IstioRevisionConditionReason = "CNINotReady"
)

const (
	// IstioRevisionConditionReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.
	IstioRevisionConditionReasonHealthy IstioRevisionConditionReason = "Healthy"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=istiorev,categories=istio-io
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description="Whether the control plane installation is ready to handle requests."
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="The current state of this object."
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="The version of the control plane installation."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"

// IstioRevision represents a single revision of an Istio Service Mesh deployment.
// Users shouldn't create IstioRevision objects directly. Instead, they should
// create an Istio object and allow the Istio operator to create the underlying
// IstioRevision object(s).
type IstioRevision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IstioRevisionSpec   `json:"spec,omitempty"`
	Status IstioRevisionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IstioRevisionList contains a list of IstioRevision
type IstioRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IstioRevision `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IstioRevision{}, &IstioRevisionList{})
}
