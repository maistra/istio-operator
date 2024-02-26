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
	. "github.com/onsi/ginkgo/v2"
)

type Action int

const (
	Delete Action = iota
	Apply
	Deploy
	Undeploy
)

var istioYaml string

// deployOperator deploys the operator to either an OpenShift cluster or a Kubernetes cluster based on the value of the 'ocp' variable.
// The operator will be deployed in the namespace specified by the 'namespace' variable.
func deployOperator() error {
	var err error

	GinkgoWriter.Println("Deploying Operator using default helm charts located in /chart folder")

	if ocp == "true" {
		GinkgoWriter.Println("Deploying to OpenShift cluster")
		err = deploy("openshift", "")
	} else {
		GinkgoWriter.Println("Deploying to Kubernetes cluster")
		err = deploy("kind", "")
	}

	if err != nil {
		GinkgoWriter.Println("Error deploying operator:", err)
		return err
	}

	return nil
}

// undeployOperator is a function that undeploys the operator from either an OpenShift cluster or a Kubernetes cluster.
// If the 'ocp' variable is set to "true", the operator will be undeployed from the OpenShift cluster.
// Otherwise, it will be undeployed from the Kubernetes cluster.
func undeployOperator() error {
	var err error

	if ocp == "true" {
		GinkgoWriter.Println("Un-Deploying from OpenShift cluster")
		err = undeploy("openshift", "")
	} else {
		GinkgoWriter.Println("Un-Deploying from Kubernetes cluster")
		err = undeploy("kind", "")
	}

	if err != nil {
		GinkgoWriter.Println("Error undeploying operator:", err)
		return err
	}

	return nil
}

// deployIstioControlPlane deploys the Istio control plane with the specified version.
// The control panel will be installed in the namespace specified by the 'controlPlaneNamespace' variable.
func deployIstioControlPlane(version string) error {
	// Deploy Istio control plane
	err := deploy("istio", version)
	if err != nil {
		GinkgoWriter.Println("Error deploying Istio control plane:", err)
		Fail("Error deploying Istio control plane")
	}

	GinkgoWriter.Println("Istio control plane deployed successfully")
	return nil
}

// undeployIstioControlPlane undeploys the Istio Control Plane for a specific version.
func undeployIstioControlPlane(version string) error {
	GinkgoWriter.Println("Undeploying Istio Control Plane for version:", version)

	err := undeploy("istio", version)
	if err != nil {
		GinkgoWriter.Println("Error undeploying Istio control plane:", err)
		return err
	}

	GinkgoWriter.Println("Istio control plane undeployed successfully")
	return nil
}

// deploy deploys the specified platform.
// It calls processDeploy function with the given platform and Deploy constant.
// If an error occurs during the deployment process, it returns the error.
// Otherwise, it returns nil.
func deploy(platform, version string) error {
	if err := processDeploy(platform, Deploy, version); err != nil {
		return err
	}
	return nil
}

// undeploy undeploys the specified platform.
// It calls the processDeploy function with the Undeploy action for the given platform.
// If an error occurs during the undeploy process, it is returned.
func undeploy(platform, version string) error {
	if err := processDeploy(platform, Undeploy, version); err != nil {
		return err
	}
	return nil
}

// processDeploy processes the deployment of a platform based on the given action.
// It takes a platform string and an action Action as parameters.
func processDeploy(platform string, action Action, version string) error {
	var output []byte
	var err error

	var params []string
	if platform == "openshift" {
		params = append(params, "--set=platform=openshift")
	}

	if action == Deploy {
		var yamlString string
		if platform == "istio" {
			// Deploy Istio control plane
			yamlString, err = readAndReplaceVersionInManifest(version)
			istioYaml = yamlString
			if err != nil {
				GinkgoWriter.Println("Error updating Istio manifest:", err)
				return err
			}
		} else {
			// Get YAML from Helm template for the istio operator
			GinkgoWriter.Println("Deploying Operator using default helm charts located in /chart folder")
			output, err = getYamlFromHelmTemplate(namespace, image, params)
			if err != nil {
				return err
			}
			yamlString = string(output)
		}

		// Apply the YAML
		err = applyFromYamlString(yamlString)

	} else if action == Undeploy {
		if platform == "istio" {
			if err := deleteFromYamlString(istioYaml); err != nil {
				GinkgoWriter.Println("Error deleting Istio manifest:", err)
				return err
			}

			return nil
		}

		GinkgoWriter.Println("Un-Deploying Operator by using helm templates generated")
		// Get YAML from Helm template for the istio operator
		output, err = getYamlFromHelmTemplate(namespace, image, params)
		if err != nil {
			return err
		}

		yamlString := string(output)
		// Delete the YAML
		err = deleteFromYamlString(yamlString)
	}

	if err != nil {
		return err
	}

	return nil
}
