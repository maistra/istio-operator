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

// ServiceImports is the Schema for configuring imported services.  The name of
// the ServiceImports resource must match the name of a MeshFederation resource
// defining the remote mesh from which the services will be imported.
type ServiceImports struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines rules for matching services to be imported.
	Spec   ServiceImportsSpec   `json:"spec,omitempty"`
	Status ServiceImportsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceImportsList contains a list of ServiceImport
type ServiceImportsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceImports `json:"items"`
}

type ServiceImportsSpec struct {
	// DomainSuffix specifies the domain suffix to be applies to imported
	// services.  If no domain suffix is specified, imported services will be
	// named as follows:
	//    <imported-name>.<imported-namespace>.svc.<mesh-name>-imports.local
	// If a domain suffix is specified, imported services will be named as
	// follows:
	//    <imported-name>.<imported-namespace>.<domain-suffix>
	// +optional
	DomainSuffix string `json:"domainSuffix,omitempty"`
	// Imports are the rules that determine which services are imported to the
	// mesh.  The list is processed in order and the first spec in the list that
	// applies to a service is the one that will be applied.  This allows more
	// specific selectors to be placed before more general selectors.
	Imports []ServiceImportRule `json:"imports,omitempty"`
}

type ServiceImportRule struct {
	// DomainSuffix applies the specified suffix to services imported by this
	// rule.  The behavior is identical to that of ServiceImportsSpec.DomainSuffix.
	// +optional
	DomainSuffix string `json:"domainSuffix,omitempty"`
	// ImportAsLocal imports the service as a local service in the mesh.  For
	// example, if an exported service, foo/bar is imported as some-ns/service,
	// the service will be imported as service.some-ns.svc.cluster.local in the
	// some-ns namespace.  If a service of this name already exists in the mesh,
	// the imported service's endpoints will be aggregated with any other
	// workloads associated with the service.  This setting overrides DomainSuffix.
	// +optional
	ImportAsLocal bool `json:"importAsLocal,omitempty"`
	// Type of rule.  Only Name type is supported.
	// +required
	Type ServiceImportExportSelectorType `json:"type"`
	// NameSelector provides a simple name matcher for importing services in
	// the mesh.
	// +optional
	NameSelector *ServiceNameMapping `json:"nameSelector,omitempty"`
}

type ServiceImportsStatus struct {
}
