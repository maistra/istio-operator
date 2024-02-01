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

package integration

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	versions       Versions
	defaultVersion string
	oldVersion     string
	newVersion     string
)

func init() {
	versionsFile := filepath.Join("..", "..", "..", "versions.yaml")
	versionsBytes, err := os.ReadFile(versionsFile)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(versionsBytes, &versions)
	if err != nil {
		panic(err)
	}

	defaultVersion = "latest"
	oldVersion = versions.Versions[1].Name
	newVersion = versions.Versions[0].Name
}

type Versions struct {
	CRDSourceVersion string        `json:"crdSourceVersion"`
	Versions         []VersionInfo `json:"versions"`
}

type VersionInfo struct {
	Name   string   `json:"name"`
	Repo   string   `json:"repo"`
	Branch string   `json:"branch,omitempty"`
	Commit string   `json:"commit"`
	Charts []string `json:"charts,omitempty"`
}
