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

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

var istioVersions []string

func TestInstall(t *testing.T) {
	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Install Operator Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")

	fillIstioVersions()

	if ocp {
		GinkgoWriter.Println("Running on OCP cluster")
		GinkgoWriter.Printf("Absolute Path: %s\n", wd)
	} else {
		GinkgoWriter.Println("Running on Kubernetes")
		GinkgoWriter.Printf("Absolute Path: %s\n", wd)
	}
}

func fillIstioVersions() {
	type Version struct {
		Name string `yaml:"name"`
	}

	type IstioVersion struct {
		Versions []Version `yaml:"versions"`
	}

	yamlFile, err := os.ReadFile(filepath.Join(baseDir, "versions.yaml"))
	if err != nil {
		Fail("Error reading the versions.yaml file")
	}

	var config IstioVersion
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		Fail("Error unmarshalling the versions.yaml file")
	}

	for _, v := range config.Versions {
		istioVersions = append(istioVersions, v.Name)
	}

	if len(istioVersions) == 0 {
		Fail("No istio versions found in the versions.yaml file")
	}

	GinkgoWriter.Println("Istio versions in yaml file:")
	for _, name := range istioVersions {
		GinkgoWriter.Println(name)
	}
}
