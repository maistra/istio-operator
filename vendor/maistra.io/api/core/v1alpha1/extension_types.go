// Copyright Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceMeshExtensionSpec defines the desired state of ServiceMeshExtension
type ServiceMeshExtensionSpec struct {
	Image            string                        `json:"image,omitempty"`
	ImagePullPolicy  corev1.PullPolicy             `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	WorkloadSelector WorkloadSelector              `json:"workloadSelector,omitempty"`
	Phase            *FilterPhase                  `json:"phase"`
	Priority         *int                          `json:"priority,omitempty"`
	Config           string                        `json:"config,omitempty"`
}

// ServiceMeshExtensionStatus defines the observed state of ServiceMeshExtension
type ServiceMeshExtensionStatus struct {
	Phase              FilterPhase      `json:"phase,omitempty"`
	Priority           int              `json:"priority,omitempty"`
	ObservedGeneration int64            `json:"observedGeneration,omitempty"`
	Deployment         DeploymentStatus `json:"deployment,omitempty"`
}

type DeploymentStatus struct {
	Ready           bool   `json:"ready,omitempty"`
	ContainerSHA256 string `json:"containerSha256,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	URL             string `json:"url,omitempty"`
	Message         string `json:"message,omitempty"`
}

// WorkloadSelector is used to match workloads based on pod labels
type WorkloadSelector struct {
	Labels map[string]string `json:"labels"`
}

// FilterPhase defines point of injection of Envoy filter
type FilterPhase string

const (
	FilterPhasePreAuthN  = "PreAuthN"
	FilterPhasePostAuthN = "PostAuthN"
	FilterPhasePreAuthZ  = "PreAuthZ"
	FilterPhasePostAuthZ = "PostAuthZ"
	FilterPhasePreStats  = "PreStats"
	FilterPhasePostStats = "PostStats"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=maistra-io,shortName=sme
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceMeshExtension is the Schema for the servicemeshextensions API
type ServiceMeshExtension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceMeshExtensionSpec   `json:"spec,omitempty"`
	Status ServiceMeshExtensionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceMeshExtensionList contains a list of ServiceMeshExtension
type ServiceMeshExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceMeshExtension `json:"items"`
}
