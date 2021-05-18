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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=maistra-io

// FederationStatus is the Schema for reporting the status of a MeshFederation.
// The name of the FederationStatus will match the name of the istiod pod to
// which it is associated.  There will be one FederationStatus resource for
// each istiod pod.
type FederationStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is unused
	Spec FederationStatusSpec `json:"spec,omitempty"`
	// Status of the mesh federations
	Status FederationStatusStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FederationStatusList contains a list of FederationStatus
type FederationStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FederationStatus `json:"items"`
}

// FederationStatusSpec is always empty
type FederationStatusSpec struct{}

// FederationStatusStatus represents the status of each federated mesh.
type FederationStatusStatus struct {
	// Meshes represents the status of each federated mesh.  The key
	// represents the name of the MeshFederation resource configuring
	// federation for with a remote mesh (as namespace/name).
	// +optional
	// +nullable
	// +patchMergeKey=mesh
	// +patchStrategy=merge,retainKeys
	Meshes []FederationStatusDetails `json:"meshes,omitempty"`
}

// FederationStatusDetails provides details about a particular federated mesh.
type FederationStatusDetails struct {
	// Mesh is the mesh to which this status applies.  This maps to a
	// MeshFederation resource.
	// +required
	Mesh string `json:"mesh"`
	// Exports provides details about the services exported by this mesh.
	// +required
	// +patchMergeKey=localService
	// +patchStrategy=merge,retainKeys
	Exports []MeshServiceMapping `json:"exports"`
	// Imports provides details about the services imported by this mesh.
	// +required
	// +patchMergeKey=localService
	// +patchStrategy=merge,retainKeys
	Imports []MeshServiceMapping `json:"imports"`
	// Discovery provides details about the connection to the remote mesh.
	// +required
	Discovery MeshDiscoveryStatus `json:"discovery"`
}

// ServiceKey provides all the details about a Service
type ServiceKey struct {
	// Name represents the simple name of the service, e.g. the metadata.name
	// field of a kubernetes Service.
	// +required
	Name string `json:"name"`
	// Namespace represents the namespace within which the service resides.
	// +required
	Namespace string `json:"namespace"`
	// Hostname represents fully qualified domain name (FQDN) used to access
	// the service.
	// +required
	Hostname string `json:"hostname"`
}

// MeshServiceMapping represents the name mapping between an exported service
// and its local counterpart.
type MeshServiceMapping struct {
	// LocalService represents the service in the local (i.e. this) mesh. For an
	// exporting mesh, this would be the service being exported. For an
	// importing mesh, this would be the imported service.
	// +required
	LocalService ServiceKey `json:"localService"`
	// ExportedName represents the fully qualified domain name (FQDN) of an
	// exported service.  For an exporting mesh, this is the name that is
	// exported to the remote mesh. For an importing mesh, this would be the
	// name of the service exported by the remote mesh.
	// +required
	ExportedName string `json:"exportedName"`
}

// MeshDiscoveryStatus represents the status of the discovery connection between
// meshes.
type MeshDiscoveryStatus struct {
	// Remotes represents details related to the inbound connections remote
	// meshes.
	// +optional
	// +patchMergeKey=source
	// +patchStrategy=merge,retainKeys
	Remotes []DiscoveryRemoteStatus `json:"remotes,omitempty"`
	// Watch represents details related to the outbound connection to the
	// remote mesh.
	// +required
	Watch DiscoveryWatchStatus `json:"watch,omitempty"`
}

// DiscoveryRemoteStatus represents details related to an inbound connection
// from a remote mesh.
type DiscoveryRemoteStatus struct {
	DiscoveryConnectionStatus `json:",inline"`
	// Source represents the source of the remote watch.
	// +required
	Source string `json:"source"`
}

// DiscoveryWatchStatus represents details related to the outbound connection
// to the remote mesh.
type DiscoveryWatchStatus struct {
	DiscoveryConnectionStatus `json:",inline"`
}

// DiscoveryConnectionStatus represents details related to connections with
// remote meshes.
type DiscoveryConnectionStatus struct {
	// Connected identfies an active connection with the remote mesh.
	// +required
	Connected bool `json:"connected"`
	// LastConnected represents the last time a connection with the remote mesh
	// was successful.
	// +optional
	LastConnected metav1.Time `json:"lastConnected,omitempty"`
	// LastEvent represents the last time an event was received from the remote
	// mesh.
	// +optional
	LastEvent metav1.Time `json:"lastEvent,omitempty"`
	// LastFullSync represents the last time a full sync was performed with the
	// remote mesh.
	// +optional
	LastFullSync metav1.Time `json:"lastFullSync,omitempty"`
	// LastDisconnect represents the last time the connection with the remote
	// mesh was disconnected.
	// +optional
	LastDisconnect metav1.Time `json:"lastDisconnect,omitempty"`
	// LastDisconnectStatus is the status returned the last time the connection
	// with the remote mesh was terminated.
	// +optional
	LastDisconnectStatus string `json:"lastDisconnectStatus,omitempty"`
}
