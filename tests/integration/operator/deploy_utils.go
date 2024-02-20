package integration_operator

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

var (
	istioYaml string
)

// deployOperator deploys the operator to either an OpenShift cluster or a Kubernetes cluster based on the value of the 'ocp' variable.
// The operator will be deployed in the namespace specified by the 'namespace' variable.
func deployOperator() {
	var err error

	if ocp == "true" {
		GinkgoWriter.Println("Deploying to OpenShift cluster")
		err = deploy("openshift", "")
	} else {
		GinkgoWriter.Println("Deploying to Kubernetes cluster")
		err = deploy("kind", "")
	}

	if err != nil {
		GinkgoWriter.Println("Error deploying operator:", err)
		Fail("Error deploying operator")
	}
}

// undeployOperator is a function that undeploys the operator from either an OpenShift cluster or a Kubernetes cluster.
// If the 'ocp' variable is set to "true", the operator will be undeployed from the OpenShift cluster.
// Otherwise, it will be undeployed from the Kubernetes cluster.
func undeployOperator() {
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
		Fail("Error undeploying operator")
	}
}

// deployIstioControlPlane deploys the Istio control plane with the specified version.
// The control panel will be installed in the namespace specified by the 'control_plane_ns' variable.
func deployIstioControlPlane(version string) {
	GinkgoWriter.Println("Deploying Istio Control Plane for version:", version)

	if err := createNamespaceIfNotExists(control_plane_ns); err != nil {
		GinkgoWriter.Println("Error creating namespace:", err)
		Fail("Error creating namespace")
	}

	// Deploy Istio control plane
	err := deploy("istio", version)
	if err != nil {
		GinkgoWriter.Println("Error deploying Istio control plane:", err)
		Fail("Error deploying Istio control plane")
	}

	GinkgoWriter.Println("Istio control plane deployed successfully")
}

// undeployIstioControlPlane undeploys the Istio Control Plane for a specific version.
func undeployIstioControlPlane(version string) {
	GinkgoWriter.Println("Undeploying Istio Control Plane for version:", version)

	err := undeploy("istio", version)

	if err != nil {
		GinkgoWriter.Println("Error undeploying Istio control plane:", err)
		Fail("Error undeploying Istio control plane")
	}

	GinkgoWriter.Println("Istio control plane undeployed successfully")
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
			GinkgoWriter.Println("Deploying Istio Control Plane for version: ", version)
			yamlString, err = readAndReplaceVersionInManifest(version)
			istioYaml = yamlString
			GinkgoWriter.Println("Deploying Istio Control Plane using the following manifest: ", yamlString)
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
			GinkgoWriter.Println("Undeploying Istio Control Plane for version: ", version)
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
