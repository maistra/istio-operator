// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integrationoperator

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	"maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/util/tests/helm"
	"maistra.io/istio-operator/pkg/util/tests/kubectl"
	"sigs.k8s.io/yaml"
)

// deployOperator deploys the operator to either an OpenShift cluster or a Kubernetes cluster based on the value of the 'ocp' variable.
// The operator will be deployed in the namespace specified by the 'namespace' variable.
func deployOperator() error {
	extraArg := ""
	if ocp == "true" {
		extraArg = "--set=platform=openshift"
	}

	output, err := helm.Template("chart", filepath.Join(baseDir, "chart"), namespace, "--include-crds", fmt.Sprintf("--set=image=%s", image), extraArg)
	if err != nil {
		return err
	}

	err = kubectl.ApplyString(output)
	if err != nil {
		return err
	}

	return nil
}

// undeployOperator is a function that undeploys the operator from either an OpenShift cluster or a Kubernetes cluster.
// If the 'ocp' variable is set to "true", the operator will be undeployed from the OpenShift cluster.
// Otherwise, it will be undeployed from the Kubernetes cluster.
func undeployOperator() error {
	extraArg := ""
	if ocp == "true" {
		extraArg = "--set=platform=openshift"
	}

	output, err := helm.Template("chart", fmt.Sprintf("%s/chart", baseDir), namespace, "--include-crds", fmt.Sprintf("--set=image=%s", image), extraArg)
	if err != nil {
		return err
	}

	err = kubectl.DeleteString(output)
	if err != nil {
		return err
	}

	return nil
}

// createIstioCR create the istio CR for the specified version.
// The control panel will be installed in the namespace specified by the 'controlPlaneNamespace' variable.
// TODO: the controlPlaneNamespace variable is not been replaced in the source file, by default is set to istio-system.
func createIstioCR(version string) error {
	yamlString, err := readAndReplaceVersionInManifest(version)
	if err != nil {
		GinkgoWriter.Println("Error updating Istio manifest:", err)
		return err
	}

	err = kubectl.ApplyString(yamlString)
	if err != nil {
		GinkgoWriter.Println("Error installing resources")
		return err
	}

	GinkgoWriter.Println("Istio resource installed successfully")
	return nil
}

// deleteIstioCR delete the istio CR for the specified version.
func deleteIstioCR(version string) error {
	yamlString, err := readAndReplaceVersionInManifest(version)
	if err != nil {
		GinkgoWriter.Println("Error updating Istio manifest:", err)
		return err
	}

	err = kubectl.DeleteString(yamlString)
	if err != nil {
		GinkgoWriter.Println("Error deleting Istio resources")
		return err
	}

	GinkgoWriter.Println("Istio resources deleted successfully")
	return nil
}

func readAndReplaceVersionInManifest(version string) (string, error) {
	// Read Istio manifest
	istioManifest, err := os.ReadFile(filepath.Join(baseDir, istioManifest))
	if err != nil {
		return "", err
	}

	// Unmarshal YAML into custom Istio struct
	var istio v1alpha1.Istio
	if err := yaml.Unmarshal(istioManifest, &istio); err != nil {
		return "", err
	}

	// Modify version
	istio.Spec.Version = version

	// Marshal custom Istio struct back to YAML
	yamlBytes, err := yaml.Marshal(&istio)
	if err != nil {
		return "", err
	}

	return string(yamlBytes), nil
}

// Get the istio versions from the versions.yaml file
// It takes a filename string as a parameter and returns a slice of strings.
// Returns the list of istio versions
func getIstioVersions(filename string) []string {
	type Version struct {
		Name string `yaml:"name"`
	}

	type IstioVersion struct {
		Versions []Version `yaml:"versions"`
	}

	GinkgoWriter.Println("Getting the istio versions")

	yamlFile, err := os.ReadFile(filename)
	if err != nil {
		GinkgoWriter.Println("Error reading the versions.yaml file")
		Fail("Error reading the versions.yaml file")
	}

	var config IstioVersion
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		GinkgoWriter.Println("Error unmarshalling the versions.yaml file")
		Fail("Error unmarshalling the versions.yaml file")
	}

	var versionList []string
	for _, v := range config.Versions {
		versionList = append(versionList, v.Name)
	}

	if len(versionList) == 0 {
		Fail("No istio versions found in the versions.yaml file")
	}

	GinkgoWriter.Println("Istio versions in yaml file:")
	for _, name := range versionList {
		GinkgoWriter.Println(name)
	}

	return versionList
}
