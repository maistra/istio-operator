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
package kubectl

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"maistra.io/istio-operator/pkg/util/tests/shell"
	resourcecondition "maistra.io/istio-operator/pkg/util/tests/types"
)

const DefaultCommandTool = "kubectl"

func commandTool() string {
	// Get the command tool from the environment
	// If not set, use the default command tool
	if cmd := os.Getenv("COMMAND"); cmd != "" {
		return cmd
	}

	return DefaultCommandTool
}

// ApplyFromYamlString applies the given yaml string to the cluster
func ApplyString(yamlString string) error {
	cmd := fmt.Sprintf("%s apply -f - ", commandTool())
	_, err := shell.ExecuteCommandWithInput(cmd, yamlString)
	if err != nil {
		return err
	}

	return nil
}

// DeleteFromYamlString delete the given yaml string to the cluster
func DeleteFromYamlString(yamlString string) error {
	cmd := fmt.Sprintf("%s delete -f - ", commandTool())
	output, err := shell.ExecuteCommandWithInput(cmd, yamlString)
	if err != nil {
		// Workaround because the resource can be already deleted
		if strings.Contains(output, "not found") {
			return nil
		}

		return err

	}

	return nil
}

// GetResourceCondition returns the condition of a resource
func GetResourceCondition(ns, resourceType, resourceName string) ([]resourcecondition.Conditions, error) {
	var resource resourcecondition.Resource

	output, err := GetResource(ns, resourceType, resourceName)
	if err != nil {
		return []resourcecondition.Conditions{}, err
	}

	err = json.Unmarshal([]byte(output), &resource)
	if err != nil {
		return []resourcecondition.Conditions{}, err
	}

	return resource.Status.Conditions, nil
}

// GetPodPhase returns the phase of a pod
func GetPodPhase(ns, podName string) (string, error) {
	var resource resourcecondition.Resource

	output, err := GetResource(ns, "pod", podName)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(output), &resource)
	if err != nil {
		return "", err
	}

	return resource.Status.Phase, nil
}

// GetResource returns the json of a resource
func GetResource(ns, resourceType, resourceName string) (string, error) {
	// `2>&1 | sed -n '/^{/,$p'` is a workaround for oc commands that can show Warning at the beggining of the output.
	cmd := fmt.Sprintf("%s get %s %s -n %s -o json 2>&1 | sed -n '/^{/,$p'", commandTool(), resourceType, resourceName, ns)
	json, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}

	return json, nil
}

// GetPodFromLabel returns the pod name from a label, if there is more than one pod, it will return an error
func GetPodFromLabel(ns, label string) (string, error) {
	var podList []string
	podList, err := GetPodsFromLabel(ns, label)
	if err != nil {
		return "", err
	}
	if len(podList) > 1 {
		return "", fmt.Errorf("more than one pod found with label %s", label)
	}
	if len(podList) == 0 {
		return "", fmt.Errorf("no pod found with label %s", label)
	}

	return podList[0], nil
}

// GetPodsFromLabel returns the pod name from a label
func GetPodsFromLabel(ns, label string) ([]string, error) {
	var podList []string
	cmd := fmt.Sprintf("%s get pods -n %s -l %s -o jsonpath={.items[*].metadata.name}", commandTool(), ns, label)
	output, err := shell.ExecuteCommand(cmd)
	podList = strings.Split(output, " ")
	if err != nil {
		return podList, fmt.Errorf("error getting pods names: %v, output: %s", err, output)
	}
	return podList, nil
}

// CreateNamespace creates a namespace
// If the namespace already exists, it will return nil
func CreateNamespace(ns string) error {
	cmd := fmt.Sprintf("%s create namespace %s", commandTool(), ns)
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		if strings.Contains(output, "AlreadyExists") {
			return nil
		}

		return err
	}

	return nil
}

// DeleteNamespace deletes a namespace
func DeleteNamespace(ns string) error {
	cmd := fmt.Sprintf("%s delete namespace %s", commandTool(), ns)
	_, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return err
	}

	return nil
}

// GetNamespace returns the namespace
// If the namespace does not exist, it will return an error
func GetNamespace(ns string) (string, error) {
	cmd := fmt.Sprintf("%s get namespace %s -o jsonpath={metadata.name}", commandTool(), ns)
	json, err := shell.ExecuteCommand(cmd)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// Workaround because Eventually seems to not be handling the error properly
			return fmt.Sprintf("namespace %s not found", ns), nil
		}

		return "", err
	}

	return json, nil
}

// GetDeployments returns the deployments of a namespace
func GetDeployments(ns string) ([]string, error) {
	var deployments []string
	cmd := fmt.Sprintf("%s get deployments -n %s -o jsonpath={.items[*].metadata.name}", commandTool(), ns)
	output, err := shell.ExecuteCommand(cmd)
	deployments = strings.Split(output, " ")
	if err != nil {
		return deployments, fmt.Errorf("error getting deployments names: %v, output: %s", err, output)
	}
	return deployments, nil
}

// GetDeploymentLogs returns the logs of a deployment
// Arguments:
// - ns: namespace
// - deploymentName: deployment name
// - Since: time range
func GetPodLogs(ns, podName, since string) (string, error) {
	cmd := fmt.Sprintf("%s logs %s -n %s  --since=%s", commandTool(), podName, ns, since)
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}

	return output, nil
}

// GetDaemonSets returns the daemonsets of a namespace
// Return a list of daemonsets
func GetDaemonSets(ns string) ([]string, error) {
	var daemonsets []string
	cmd := fmt.Sprintf("%s get daemonsets -n %s -o jsonpath={.items[*].metadata.name}", commandTool(), ns)
	output, err := shell.ExecuteCommand(cmd)
	// If output is empty, return an empty list
	if output == "" {
		return daemonsets, nil
	}

	daemonsets = strings.Split(output, " ")
	if err != nil {
		return daemonsets, fmt.Errorf("error getting daemonsets names: %v, output: %s", err, output)
	}
	return daemonsets, nil
}
