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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"gopkg.in/yaml.v2"
)

const (
	maxRetries = 10
	retryDelay = 5 * time.Second
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
		GinkgoWriter.Printf("Error running Helm template: %s", err)
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

		for i := 0; i < maxRetries; i++ {
			// Create a new instance of exec.Cmd for each retry
			cmdRetry := exec.Command(cmd.Args[0], cmd.Args[1:]...)
			cmdRetry.Stdin = cmd.Stdin

			if err := cmdRetry.Run(); err != nil {
				GinkgoWriter.Printf("Error running kubectl apply/delete: %s", err)
				if i < maxRetries-1 {
					GinkgoWriter.Printf("Retrying after %v...", retryDelay)
					time.Sleep(retryDelay)
				} else {
					return err
				}
			} else {
				break
			}
		}

	}
	return nil
}

// Check if the operator is running.
// It checks if the istio-operator pod is running and the istio-operator deployment is available in 'namespace' defined from the env var.
func operatorIsRunning() bool {
	GinkgoWriter.Printf("Check Operator is Running, POD: NAMESPACE: \"%s\"   POD NAME: \"%s\"", namespace, deploymentName)

	podName, err := getPodNameFromLabel(namespace, "control-plane=istio-operator")
	if err != nil {
		GinkgoWriter.Printf("Error getting pod name from deployment: %v", err)
		return false
	}

	GinkgoWriter.Printf("Pod name: %s", podName)
	err = checkPodRunning(namespace, podName)
	if err != nil {
		GinkgoWriter.Printf("Error checking pod running: %v\n", err)
		return false
	}

	err = checkDeploymentAvailable(namespace, deploymentName)
	if err != nil {
		GinkgoWriter.Printf("Error checking deployment available: %v\n", err)
		return false
	}

	return true
}

// istioControlPlaneIsInstalledAndRunning checks if the Istio control plane is installed and running.
// It waits for Istio to be reconciled and ready, checks if the istiod pod is running,
// captures the last 30 seconds of the log, and verifies that the istio-operator has stopped reconciling the resource.
// Finally, it checks if the installed Istio version matches the expected version.
// Returns:
// - bool: true if the Istio control plane is installed and running, false otherwise.
func istioControlPlaneIsInstalledAndRunning(version string) bool {
	err := waitForIstioCondition(command, istioName, "Reconciled")
	if err != nil {
		GinkgoWriter.Printf("Error waiting for Istio to be reconciled: %v", err)
		return false
	}

	GinkgoWriter.Println("Istio control plane reconciled successfully")

	err = waitForIstioCondition(command, istioName, "Ready")
	if err != nil {
		GinkgoWriter.Printf("Error waiting for Istio to be ready: %v", err)
		return false
	}

	GinkgoWriter.Println("Istio ready successfully")

	podName, err := getPodNameFromLabel(controlPlaneNamespace, "app=istiod")
	if err != nil {
		GinkgoWriter.Printf("Error getting pod name from deployment: %v", err)
		Fail("Error getting pod name from deployment")
	}

	for i := 0; i < maxRetries; i++ {
		err := checkPodRunning(controlPlaneNamespace, podName)
		if err != nil {
			GinkgoWriter.Printf("Error checking pod running: %v\n", err)
			if i < maxRetries-1 {
				GinkgoWriter.Printf("Retrying after %v...", retryDelay)
				time.Sleep(retryDelay)
			} else {
				return false
			}
		} else {
			// If the pod is running, break the loop
			break
		}
	}
	// Sleep 60s to settle down the control plane
	time.Sleep(60 * time.Second)

	checkIstioLogs()

	// Check that the istio version installed is the expected one
	verifyInstalledIstioVersion(version)

	return true
}

// Verify the installed Istio version match the provided version
// If the installed version does not match the expected version, it fails the test
func verifyInstalledIstioVersion(version string) {
	cmd := exec.Command(command, "get", "istio", istioName, "-n", controlPlaneNamespace, "-o", "jsonpath={.spec.version}")
	istioVersion, err := cmd.CombinedOutput()
	if err != nil {
		GinkgoWriter.Printf("Error getting istio version: %v, output: %s", err, istioVersion)
		Fail("Error getting istio version")
	}
	GinkgoWriter.Printf("Istio version installed in the namespace %s: %s\n", controlPlaneNamespace, string(istioVersion))

	if strings.TrimSpace(string(istioVersion)) != version {
		GinkgoWriter.Printf("Expected istio version %s, but got %s\n", version, string(istioVersion))
		Fail("Expected istio version does not match the installed version")
	}
}

// Check if the istio-operator has stopped reconciling the resource
func checkIstioLogs() {
	GinkgoWriter.Println("Check that the operator has stopped reconciling the resource (waiting 30s)")
	last30secondsOfLog, err := captureLast30SecondsOfLog()
	if err != nil {
		GinkgoWriter.Printf("Error capturing last 30 seconds of log: %v", err)
		Fail("Error capturing last 30 seconds of log")
	}

	if strings.Contains(last30secondsOfLog, "Reconciliation done") {
		GinkgoWriter.Println("Expected istio-operator to stop reconciling the resource, but it didn't:")
		GinkgoWriter.Println(last30secondsOfLog)
		GinkgoWriter.Printf("Note: The above log was captured at %s", time.Now().Format(time.RFC3339))
		Fail("Expected istio-operator to stop reconciling the resource, but it didn't")
	}

	GinkgoWriter.Println("Reconciliation done successfully")
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

// Check if the pod is running
// checkPodRunning checks the status of a pod in a given namespace.
// It retrieves the current phase of the pod and waits for it to be in the "Running" state.
// If the pod is not running after 12 attempts, it returns an error.
// Returns:
// - error: An error if the pod is not running.
func checkPodRunning(ns, podName string) error {
	// Get and print the current phase of the pod
	status := exec.Command(command, "get", "pod", podName, "-n", ns, "-o", "jsonpath={.status.phase}")
	podStatus, err := status.CombinedOutput()
	if err != nil {
		Fail(fmt.Sprintf("error getting pod status: %v, output: %s", err, string(podStatus)))
		return fmt.Errorf("error getting pod status: %v, output: %s", err, string(podStatus))
	}
	GinkgoWriter.Printf("Istio pod status: %s", string(podStatus))

	for i := 0; i < maxRetries; i++ {
		if strings.Contains(string(podStatus), "Running") {
			GinkgoWriter.Println("Istio pod is running")
			break
		}
		GinkgoWriter.Println("Waiting for the pod to be running...")
		time.Sleep(retryDelay)

		// Run the command again to check pod status
		status := exec.Command(command, "get", "pod", podName, "-n", ns, "-o", "jsonpath={.status.phase}")
		podStatus, err = status.CombinedOutput()
		if err != nil {
			Fail(fmt.Sprintf("error getting pod status: %v, output: %s", err, string(podStatus)))
			return fmt.Errorf("error getting pod status: %v, output: %s", err, string(podStatus))
		}

		GinkgoWriter.Printf("Istio pod current status: %s\n", string(podStatus))
	}

	if !strings.Contains(string(podStatus), "Running") {
		return fmt.Errorf("istio pod is not running")
	}

	return nil
}

// Check deployment is available
// checkDeploymentAvailable checks if a deployment is available in the specified namespace.
// It waits for the deployment to be available by executing the `wait` command with the specified parameters.
// If the deployment is not available within the specified timeout, an error is returned.
// Returns:
// - An error if the deployment is not available within the timeout, otherwise nil.
func checkDeploymentAvailable(ns, deploymentName string) error {
	GinkgoWriter.Printf("Check Deployment Available: NAMESPACE: \"%s\"   DEPLOYMENT NAME: \"%s\"", ns, deploymentName)

	waitDeploymentCommand := exec.Command(command, "wait", "deployment", deploymentName, "-n", ns, "--for", "condition=Available=True", "--timeout", timeout)

	waitOutput, err := waitDeploymentCommand.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error waiting for deployment: %v, output: %s", err, waitOutput)
	}

	return nil
}

// Check if only istio resources are deployed
// checkOnlyIstioIsDeployed checks if only the Istio deployment is present in the specified namespace.
// It returns true if only the Istio deployment is found, otherwise false.
func checkOnlyIstioIsDeployed(ns string) bool {
	GinkgoWriter.Printf("Check that only istio resources are deployed in the namespace %s", ns)

	deployments := exec.Command(command, "get", "deployments", "-n", ns, "-o", "jsonpath={.items[*].metadata.name}")
	deploymentsOutput, err := deployments.CombinedOutput()
	if err != nil {
		Fail(fmt.Sprintf("error getting deployments: %v, output: %s", err, deploymentsOutput))
	}
	GinkgoWriter.Printf("Deployments in the namespace: %s", string(deploymentsOutput))

	deploymentsList := strings.Split(string(deploymentsOutput), " ")

	// Check if the last element is an empty string and remove it
	if deploymentsList[len(deploymentsList)-1] == "" {
		deploymentsList = deploymentsList[:len(deploymentsList)-1]
	}

	// Check that only the istio deployment is present
	if len(deploymentsList) != 1 {
		GinkgoWriter.Printf("Expected only 1 deployment, but got %d", len(deploymentsList))
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

// checkNamespaceEmpty checks if the specified namespace is empty by verifying if there are any resources present in the namespace.
// It prints the deployments in the namespace and returns true if the namespace is empty, otherwise returns false.
func checkNamespaceEmpty(ns string) bool {
	GinkgoWriter.Printf("Check that the namespace %s is empty after the undeploy", ns)

	// We need to wait for the namespace to be empty because the undeploy process is not immediate
	if err := waitForNamespaceEmpty(ns); err != nil {
		GinkgoWriter.Println("Error waiting for namespace to be empty:", err)
		return false
	}

	GinkgoWriter.Printf("Namespace %s is empty", controlPlaneNamespace)
	return true
}

// waitForNamespaceEmpty waits for the specified namespace to be empty by checking if there are any resources present in the namespace.
func waitForNamespaceEmpty(ns string) error {
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command(command, "get", "all", "-n", ns)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}

		if strings.Contains(string(output), "No resources found in") {
			return nil
		}

		GinkgoWriter.Println("Namespace is not empty, waiting 10 seconds to check again")
		time.Sleep(retryDelay)
	}
	return errors.New("namespace still not empty after waiting")
}

// namespaceIsDeleted delete the namespace and checks if the namespace is deleted.
// It attempts to delete the namespace and verifies if it still exists.
// If the namespace is deleted, it returns true. Otherwise, it returns false.
func deleteAndCheckNamespaceIsDeleted() bool {
	GinkgoWriter.Printf("Delete namespace: %s", controlPlaneNamespace)

	cmd := exec.Command(command, "delete", "namespace", controlPlaneNamespace)
	if err := cmd.Run(); err != nil {
		GinkgoWriter.Println("Error deleting namespace:", err)
	}

	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command(command, "get", "namespace", controlPlaneNamespace)
		if err := cmd.Run(); err == nil {
			GinkgoWriter.Println("Namespace still exists. Attempting to delete it again.... attempt number ", i)
		} else {
			GinkgoWriter.Println("Namespace deleted")
			return true
		}

		// Show all resources in the namespace
		cmd = exec.Command(command, "get", "all", "-n", controlPlaneNamespace)
		output, err := cmd.CombinedOutput()
		if err != nil {
			GinkgoWriter.Println("Error getting all resources in namespace:", err)
		}
		GinkgoWriter.Println("Resources in the namespace: ", controlPlaneNamespace)
		GinkgoWriter.Println(string(output))

		time.Sleep(retryDelay)
	}

	return false
}

// Check if the CNI Daemon is deployed
// cniDaemonIsDeployed checks if the CNI Daemon is deployed.
// It returns true if the CNI Daemon is deployed, false otherwise.
func cniDaemonIsDeployed(ns string) bool {
	if ocp == "true" {
		// Check that the CNI Daemon is deployed when ocp is true
		for i := 0; i < maxRetries; i++ {
			cmd := exec.Command(command, "rollout", "status", "ds/istio-cni-node", "-n", ns)
			if err := cmd.Run(); err != nil {
				GinkgoWriter.Println("Error checking istio cni:", err)
				GinkgoWriter.Println("Daemonset istio-cni-node is not deployed, retrying in 10 seconds")
				time.Sleep(retryDelay)
			} else {
				GinkgoWriter.Println("CNI Daemon is deployed")
				return true
			}
		}
	} else {
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

	// We reach this point means the CNI Daemon is not deployed after the retries when ocp is true
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

	// Replace version
	var istio Istio
	if err := yaml.Unmarshal(istioManifest, &istio); err != nil {
		return "", err
	}
	istio.Spec.Version = version
	yaml, err := yaml.Marshal(&istio)

	return string(yaml), err
}
