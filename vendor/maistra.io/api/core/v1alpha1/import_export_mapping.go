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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServiceImportExportSelectorType string

const (
	LabelSelectorType ServiceImportExportSelectorType = "Label"
	NameSelectorType  ServiceImportExportSelectorType = "Name"
)

type ServiceName struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

func (s ServiceName) String() string {
	return fmt.Sprintf("%s/%s", s.Namespace, s.Name)
}

// XXX: this messes up crd generation
// func (s ServiceName) NamespacedName() types.NamespacedName {
// 	return types.NamespacedName{Namespace: s.Namespace, Name: s.Name}
// }

const MatchAny = "*"

type ServiceNameMapping struct {
	Name  ServiceName  `json:"name,omitempty"`
	Alias *ServiceName `json:"alias,omitempty"`
}

type ServiceImportExportLabelelector struct {
	// Namespace specifies to which namespace the selector applies.  An empty
	// value applies to all namespaces in the mesh.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Selector used to select Service resources in the namespace/mesh.  An
	// empty selector selects all services.
	// +required
	Selector metav1.LabelSelector `json:"selector,omitempty"`

	// Aliases is a map of aliases to apply to exported services.  If a name is
	// not found in the map, the original service name is exported.  A '*' will
	// match any name. The Aliases list will be processed in order, with the
	// first match found being applied to the exported service.
	// Examples:
	// */foo->*/bar will match foo service in any namesapce, exporting it as bar from its original namespace.
	// */foo->bar/bar will match foo service in any namespace, exporting it as bar/bar.
	// foo/*->bar/* will match any service in foo namespace, exporting it from the bar namespace with its original name
	// */*->bar/* will match any service and export it from the bar namespace with its original name.
	// */*->*/* is the same as not specifying anything
	// +optional
	Aliases []ServiceNameMapping `json:"aliases,omitempty"`
}


