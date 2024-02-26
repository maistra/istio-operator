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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"maistra.io/istio-operator/api/v1alpha1"
	"sigs.k8s.io/yaml"
)

type Version struct {
	Name string `yaml:"name"`
}

type IstioVersion struct {
	Versions []Version `yaml:"versions"`
}

// getYamlFromHelmTemplate is a function that generates YAML from a Helm template.
// It takes optional arguments and returns the generated YAML as a byte slice.
// If an error occurs during the generation process, it returns the error.
func getYamlFromHelmTemplate(namespace string, image string, args []string) ([]byte, error) {
	baseArgs := []string{"template", "chart", "chart", "--include-crds", "--namespace", namespace, fmt.Sprintf("--set=image=%s", image)}

	argsList := append(baseArgs, args...)
	cmd := exec.Command("helm", argsList...)

	baseDir := filepath.Dir(filepath.Dir(filepath.Dir(wd)))
	GinkgoWriter.Println("The base working directory is: ", baseDir)
	cmd.Dir = baseDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		GinkgoWriter.Printf("Error running Helm template: %s\n", err)
		return nil, err
	}

	return output, nil
}

// Apply the resources from the YAML string
func applyFromYamlString(output string) error {
	return processYamlString(output, Apply)
}

// Delete the resources from the YAML string
func deleteFromYamlString(output string) error {
	return processYamlString(output, Delete)
}

// Process the YAML string and apply or delete the resources
// It takes a byte slice and an action Action as parameters.
// If an error occurs during the process, it returns the error.
func processYamlString(output string, action Action) error {
	yamlDocs := strings.Split(output, "---")
	for _, doc := range yamlDocs {
		// Skip non-valid YAML documents, because some of them are empty or contain comments that cannot be parsed
		// Workaround becase helm template output is not directly usable
		if !strings.Contains(doc, "apiVersion") {
			continue
		}

		var cmd *exec.Cmd
		switch action {
		case Delete:
			cmd = exec.Command(command, "delete", "-f", "-")
		case Apply:
			cmd = exec.Command(command, "apply", "-f", "-")
		default:
			return fmt.Errorf("unsupported action: %v", action)
		}

		cmd.Stdin = bytes.NewBufferString(doc)

		if output, err := cmd.CombinedOutput(); err != nil {
			// Workaround for when the Eventually fails and retry the deletion of the entire yaml string again
			if strings.Contains(string(output), "not found") {
				continue
			}

			GinkgoWriter.Printf("Error running kubectl command: %v\n", err)
			GinkgoWriter.Printf("Output: %s\n", output)
			return err
		}
	}
	return nil
}

// Check the current state of the operator.
// It checks if the istio-operator exist, the current statue and if the deployment is available.
func getOperatorState() (string, error) {
	GinkgoWriter.Printf("Check Operator state, POD: NAMESPACE: \"%s\"   POD NAME: \"%s\"\n", namespace, deploymentName)

	err := checkDeploymentAvailable(namespace, deploymentName)
	if err != nil {
		GinkgoWriter.Printf("Error checking deployment available: %v\n", err)
		return "", err
	}

	podName, err := getPodNameFromLabel(namespace, "control-plane=istio-operator")
	if err != nil {
		GinkgoWriter.Printf("Error getting pod name from deployment: %v\n", err)
		return "", err
	}
	GinkgoWriter.Printf("Pod name: %s\n", podName)
	Expect(podName).To(ContainSubstring(deploymentName))

	podPhase, err := getPodPhase(namespace, podName)
	if err != nil {
		GinkgoWriter.Printf("Error checking pod phase: %v\n", err)
		return podPhase, err
	}

	return podPhase, nil
}

// istioControlPlaneIsInstalledAndRunning checks if the Istio control plane is installed and running.
// It waits for Istio to be reconciled and ready, checks if the istiod pod is running,
// captures the last 30 seconds of the log, and verifies that the istio-operator has stopped reconciling the resource.
// Finally, it checks if the installed Istio version matches the expected version.
// Returns:
// - bool: true if the Istio control plane is installed and running, false otherwise.
// func istioControlPlaneIsRunning(version string) bool {
// 	checkIstioLogs()

// 	// Check that the istio version installed is the expected one

// 	return true
// }

// Get the installed Istio
// Returns:
// - string: The installed Istio version.
func getInstalledIstioVersion() string {
	cmd := exec.Command(command, "get", "istio", istioName, "-n", controlPlaneNamespace, "-o", "jsonpath={.spec.version}")
	istioVersion, err := cmd.Output()
	if err != nil {
		GinkgoWriter.Printf("Error getting istio version: %v, output: %s\n", err, istioVersion)
		return ""
	}
	istioVersionStr := strings.TrimSpace(string(istioVersion))
	GinkgoWriter.Printf("Istio version installed in the namespace %s: %s\n", controlPlaneNamespace, istioVersionStr)

	return istioVersionStr
}

// Check if the istio-operator has stopped reconciling the resource
func checkIstioStoppedReconciling() error {
	last30secondsOfLog, err := captureLast30SecondsOfLog()
	if err != nil {
		GinkgoWriter.Printf("Error capturing last 30 seconds of log: %v\n", err)
		return err
	}

	if strings.Contains(last30secondsOfLog, "Reconciliation done") {
		GinkgoWriter.Println("Expected istio-operator to stop reconciling the resource, but it didn't")
		GinkgoWriter.Printf("Log was captured at %s\n", time.Now().Format(time.RFC3339))
		return fmt.Errorf("expected istio-operator to stop reconciling the resource, but it didn't")
	}

	GinkgoWriter.Println("Reconciliation done successfully")
	return nil
}

// waitForIstioCondition waits for a specific condition to be true for an Istio resource.
// It executes the specified command with the given Istio name, condition, and timeout.
// If the command fails or the condition is not met within the timeout, an error is returned.
func waitForIstioCondition(command, istioName, condition string) error {
	cmd := exec.Command(command, "wait", fmt.Sprintf("istio/%s", istioName), "--for", fmt.Sprintf("condition=%s=True", condition), "--timeout", timeout)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error executing command: %v, output: %s", err, output)
	}

	GinkgoWriter.Println(fmt.Sprintf("Istio %s is %s", istioName, condition))
	return nil
}

// Capture the last 30 seconds of the logs
// captureLast30SecondsOfLog captures the logs of a specific deployment within the last 30 seconds.
// It executes the `logs` command with the specified deployment name, namespace, and time range.
// The captured logs are returned as a string.
// If there is an error capturing the logs, an error is returned along with the stderr output.
func captureLast30SecondsOfLog() (string, error) {
	cmd := exec.Command(command, "logs", fmt.Sprintf("deploy/%s", deploymentName), "-n", namespace, "--since", "30s")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error capturing logs: %v, stderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// getPodPhase
// It retrieves the current phase of a pod in the specified namespace.
// Returns:
// - status: The current phase of the pod.
// - error: An error if the pod is not running.
func getPodPhase(ns, podName string) (string, error) {
	cmd := exec.Command(command, "get", "pod", podName, "-n", ns, "-o", "jsonpath={.status.phase}")
	podPhase, err := cmd.CombinedOutput()
	if err != nil {
		return string(podPhase), fmt.Errorf("error getting pod status: %v, output: %s", err, string(podPhase))
	}

	GinkgoWriter.Printf("Current pod %s status: %s\n", podName, string(podPhase))

	return string(podPhase), nil
}

// Check deployment is available
// checkDeploymentAvailable checks if a deployment is available in the specified namespace.
// It waits for the deployment to be available by executing the `wait` command with the specified parameters.
// If the deployment is not available within the specified timeout, an error is returned.
// Returns:
// - An error if the deployment is not available within the timeout, otherwise nil.
func checkDeploymentAvailable(ns, deploymentName string) error {
	GinkgoWriter.Printf("Check Deployment Available: NAMESPACE: \"%s\"   DEPLOYMENT NAME: \"%s\"\n", ns, deploymentName)

	waitDeploymentCommand := exec.Command(command, "wait", "deployment", deploymentName, "-n", ns, "--for", "condition=Available=True", "--timeout", timeout)

	waitOutput, err := waitDeploymentCommand.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error waiting for deployment: %v, output: %s", err, waitOutput)
	}

	GinkgoWriter.Println(fmt.Sprintf("Deployment %s is available", deploymentName))
	return nil
}

// Check if only istio resources are deployed
// checkOnlyIstioIsDeployed checks if only the Istio deployment is present in the specified namespace.
// It returns true if only the Istio deployment is found, otherwise false.
func checkOnlyIstioIsDeployed(ns string) bool {
	GinkgoWriter.Printf("Check that only istio resources are deployed in the namespace %s\n", ns)

	deploymentsCmd := exec.Command(command, "get", "deployments", "-n", ns, "-o", "jsonpath={.items[*].metadata.name}")
	deploymentsOutput, err := deploymentsCmd.Output()
	if err != nil {
		Fail(fmt.Sprintf("error getting deployments: %v, output: %s", err, deploymentsOutput))
	}

	deploymentsList := strings.Fields(string(deploymentsOutput))

	if len(deploymentsList) != 1 || deploymentsList[0] != "istiod" {
		GinkgoWriter.Printf("Expected only 1 deployment with name 'istiod', but got %d deployments\n", len(deploymentsList))
		return false
	}

	return true
}

func createNamespaceIfNotExists(namespace string) error {
	cmd := exec.Command(command, "get", "namespace", namespace)
	if err := cmd.Run(); err != nil {
		cmd = exec.Command(command, "create", "namespace", namespace)
		return cmd.Run()
	}
	return nil
}

// namespaceEmpty checks if the specified namespace is empty by verifying if there are any resources present in the namespace.
// Returns:
// - true if the namespace is empty, false otherwise.
func namespaceEmpty(ns string) bool {
	GinkgoWriter.Printf("Check that the namespace %s is empty after the undeploy\n", ns)

	cmd := exec.Command(command, "get", "all", "-n", ns)
	output, err := cmd.CombinedOutput()
	if err != nil {
		Fail(fmt.Sprintf("error getting all resources in namespace: %v, output: %s", err, output))
	}

	if strings.Contains(string(output), "No resources found in") {
		return true
	}

	GinkgoWriter.Println("Resources in the namespace: ", ns)
	GinkgoWriter.Println(string(output))
	return false
}

func deleteNamespace(ns string) error {
	GinkgoWriter.Printf("Delete namespace: %s\n", controlPlaneNamespace)

	cmd := exec.Command(command, "delete", "namespace", controlPlaneNamespace)
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// namespaceIsDeleted checks if the specified namespace is deleted or exist
// If the namespace is deleted, it returns true. Otherwise, it returns false.
func namespaceIsDeleted(ns string) bool {
	cmd := exec.Command(command, "get", "namespace", ns)
	if err := cmd.Run(); err == nil {
		GinkgoWriter.Println("Namespace still exists...")

		// Show all resources in the namespace for debugging purposes
		cmd = exec.Command(command, "get", "all", "-n", ns)
		output, err := cmd.CombinedOutput()
		if err != nil {
			GinkgoWriter.Println("Error getting all resources in namespace:", err)
		}
		GinkgoWriter.Println("Resources in the namespace: ", ns)
		GinkgoWriter.Println(string(output))
		return false
	}

	GinkgoWriter.Println("Namespace deleted")
	return true
}

// Check if the CNI Daemon is deployed
// cniDaemonIsDeployed checks if the CNI Daemon is deployed.
// It returns true if the CNI Daemon is deployed, false otherwise.
func cniDaemonIsDeployed(ns string) bool {
	if ocp == "true" {
		cmd := exec.Command(command, "rollout", "status", "ds/istio-cni-node", "-n", ns)
		if err := cmd.Run(); err != nil {
			GinkgoWriter.Println("Error checking istio cni:", err)
			return false
		}
		GinkgoWriter.Println("CNI Daemon is deployed")
		return true
	}

	// On kind cluster the cni daemon is not deployed
	cmd := exec.Command(command, "get", "daemonset", "-n", ns)
	output, err := cmd.CombinedOutput()
	if err != nil {
		GinkgoWriter.Println("Error getting daemonset:", err)
		GinkgoWriter.Println(string(output))
		return false
	}

	if strings.Contains(string(output), "cni") {
		GinkgoWriter.Println("Daemonset istio-cni-node is deployed")
		GinkgoWriter.Println(string(output))
		return true
	}

	GinkgoWriter.Println("Daemonset istio-cni-node is not deployed")
	return false
}

// podExist checks that the pod exists in the namespace based in the given label
// It return nil error if the pod exists, otherwise it returns an error
func podExist(ns, label string) bool {
	podName, err := getPodNameFromLabel(ns, label)
	if err != nil {
		if strings.Contains(err.Error(), "array index out of bounds") {
			GinkgoWriter.Printf("Pod %s does not exist\n", podName)
			return false
		}
		GinkgoWriter.Printf("Error getting pod name: %v\n", err)
		return false
	}

	if strings.Contains(podName, "istio-operator") {
		GinkgoWriter.Printf("Pod %s exists\n", podName)
		return true
	}

	return false
}

// Get the podName from the deployment
// getPodNameFromLabel retrieves the name of a pod in a given namespace based on a label.
// It executes a command to get the pods in the specified namespace with the given label,
// and returns the name of the first pod found.
// If an error occurs during the command execution, it returns an empty string and the error.
func getPodNameFromLabel(ns, label string) (string, error) {
	cmd := exec.Command(command, "get", "pods", "-n", ns, "-l", label, "-o", "jsonpath={.items[0].metadata.name}")
	podName, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error getting pod name: %v, output: %s", err, podName)
	}
	return string(podName), nil
}

// Get the istio versions from the versions.yaml file
// It takes a filename string as a parameter and returns a slice of strings.
// Returns the list of istio versions
func getIstioVersions(filename string) ([]string, error) {
	GinkgoWriter.Println("Getting the istio versions")

	yamlFile, err := os.ReadFile(filename)
	if err != nil {
		GinkgoWriter.Println("Error reading the versions.yaml file")
		return nil, err
	}

	var config IstioVersion
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		GinkgoWriter.Println("Error unmarshalling the versions.yaml file")
		return nil, err
	}

	var versionList []string
	for _, v := range config.Versions {
		versionList = append(versionList, v.Name)
	}

	if len(versionList) == 0 {
		return nil, fmt.Errorf("no istio versions found to deploy the control plane in the versions.yaml file")
	}

	GinkgoWriter.Println("Istio versions in yaml file:")
	for _, name := range versionList {
		GinkgoWriter.Println(name)
	}

	return versionList, nil
}

func readAndReplaceVersionInManifest(version string) (string, error) {
	// Read Istio manifest
	baseDir := filepath.Dir(filepath.Dir(filepath.Dir(wd)))
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
