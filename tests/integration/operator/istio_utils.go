// Copyright Istio Authors
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

package integrationoperator

// Istio is a representation of the istio YAML file

type UpdateStrategy struct {
	Type                                       string `yaml:"type"`
	InactiveRevisionDeletionGracePeriodSeconds int    `yaml:"inactiveRevisionDeletionGracePeriodSeconds"`
}

type Requests struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
	// Add other optional fields here as needed
}

type Metadata struct {
	Name string `yaml:"name"`
}
type Resources struct {
	Requests *Requests `yaml:"requests,omitempty"`
	// Add other optional fields here as needed
}
type Pilot struct {
	Resources *Resources `yaml:"resources,omitempty"`
	// Add other optional fields here as needed
}
type RawValues struct {
	Pilot *Pilot `yaml:"pilot,omitempty"`
	// Add other optional fields here as needed
}

type Spec struct {
	Version        string          `yaml:"version"`
	Namespace      string          `yaml:"namespace"`
	UpdateStrategy *UpdateStrategy `yaml:"updateStrategy,omitempty"`
	RawValues      *RawValues      `yaml:"rawValues,omitempty"`
}

type Istio struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}
