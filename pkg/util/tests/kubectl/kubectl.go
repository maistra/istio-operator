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
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubectl

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/util/tests/shell"
	r "github.com/istio-ecosystem/sail-operator/pkg/util/tests/types"
)

const DefaultBinary = "kubectl"

var (
	ErrNotFound       = errors.New("resource was not found")
	EmptyResourceList = r.ResourceList{
		APIVersion: "v1",
		Items:      []interface{}{},
		Kind:       "List",
		Metadata:   r.Metadata{ResourceVersion: ""},
	}
)

// kubectl return the kubectl command
// If the environment variable COMMAND is set, it will return the value of COMMAND
// Otherwise, it will return the default value "kubectl" as default
// Arguments:
// - format: format of the command without kubeclt or oc
// - args: arguments of the command
func kubectl(format string, args ...interface{}) string {
	binary := DefaultBinary
	if cmd := os.Getenv("COMMAND"); cmd != "" {
		binary = cmd
	}

	return binary + " " + fmt.Sprintf(format, args...)
}

// ApplyString applies the given yaml string to the cluster
func ApplyString(yamlString string) error {
	cmd := kubectl("apply --server-side -f -")
	_, err := shell.ExecuteCommandWithInput(cmd, yamlString)
	if err != nil {
		return fmt.Errorf("error applying yaml: %v", err)
	}

	return nil
}

// DeleteString delete the given yaml string to the cluster
func DeleteString(yamlString string) error {
	cmd := kubectl("delete -f -")
	output, err := shell.ExecuteCommandWithInput(cmd, yamlString)
	if err != nil {
		// Workaround because the resource can be already deleted
		if strings.Contains(output, "not found") {
			return nil
		}

		return fmt.Errorf("error deleting yaml: %v", err)

	}

	return nil
}

// GetJSONCondition returns the condition of a resource
func GetJSONCondition(ns, resourceType, resourceName string) ([]r.Condition, error) {
	var resource r.Resource

	output, err := GetJSON(ns, resourceType, resourceName)
	if err != nil {
		return []r.Condition{}, err
	}

	err = json.Unmarshal([]byte(output), &resource)
	if err != nil {
		return []r.Condition{}, err
	}

	return resource.Status.Conditions, nil
}

// GetPodPhase returns the phase of a pod
func GetPodPhase(ns, selector string) (string, error) {
	var resource r.Resource

	podName, err := GetPodName(ns, selector)
	if err != nil {
		return "", err
	}

	output, err := GetJSON(ns, "pod", podName)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(output), &resource)
	if err != nil {
		return "", err
	}

	return resource.Status.Phase, nil
}

// GetCRDs returns all the CRDs names in a list
func GetCRDs() ([]string, error) {
	cmd := kubectl("get crds -o jsonpath={.items[*].metadata.name}")
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return []string{}, fmt.Errorf("error getting crds: %v", err)
	}

	return strings.Split(output, " "), nil
}

// GetJSONList returns a json list of the resources of a namespace
func GetJSONList(ns string) (r.ResourceList, error) {
	// TODO: improve the function to get all the resources
	output, err := GetJSON(ns, "all", "")
	if err != nil {
		return EmptyResourceList, err
	}

	var resourceList r.ResourceList
	err = json.Unmarshal([]byte(output), &resourceList)
	if err != nil {
		return EmptyResourceList, err
	}

	// Return an empty list if there are no resources
	if len(resourceList.Items) == 0 {
		return EmptyResourceList, nil
	}

	return resourceList, nil
}

// GetJSON returns the json of a resource
// Arguments:
// - ns: namespace
// - resourceType: type of the resource
// - resourceName: name of the resource
func GetJSON(ns, resourceType, resourceName string) (string, error) {
	cmd := kubectl("get %s %s -n %s -o json", resourceType, resourceName, ns)
	json, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}

	return json, nil
}

// GetPodName returns the pod name from a label, if there is more than one pod, it will return an error
func GetPodName(ns, label string) (string, error) {
	var podList []string
	podList, err := GetPodsName(ns, label)
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

// GetPodsName returns the pod name from a label
func GetPodsName(ns, label string) ([]string, error) {
	var podList []string
	cmd := kubectl("get pods -n %s -l %s -o jsonpath={.items[*].metadata.name}", ns, label)
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
	cmd := kubectl("create namespace %s", ns)
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		if strings.Contains(output, "AlreadyExists") {
			return nil
		}

		return fmt.Errorf("error creating namespace: %v, output: %s", err, output)
	}

	return nil
}

// DeleteNamespace deletes a namespace
func DeleteNamespace(ns string) error {
	cmd := kubectl("delete namespace %s", ns)
	_, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("error deleting namespace: %v", err)
	}

	return nil
}

// CheckNamespaceExist checks if a namespace exists
// If the namespace exists, it will return nil
// If the namespace does not exist, it will return an error
func CheckNamespaceExist(ns string) error {
	cmd := kubectl("get namespace %s -o jsonpath={metadata.name}", ns)
	_, err := shell.ExecuteCommand(cmd)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ErrNotFound
		}

		return fmt.Errorf("error checking namespace: %v", err)
	}

	return nil
}

// GetDeploymentNames returns the deployments of a namespace
func GetDeploymentNames(ns string) ([]string, error) {
	var deployments []string
	cmd := kubectl("get deployments -n %s -o jsonpath={.items[*].metadata.name}", ns)
	output, err := shell.ExecuteCommand(cmd)
	deployments = strings.Split(output, " ")
	if err != nil {
		return deployments, fmt.Errorf("error getting deployments names: %v, output: %s", err, output)
	}
	return deployments, nil
}

// Logs returns the logs of a deployment
// Arguments:
// - ns: namespace
// - selector: selector of the pod
// - Since: time range
func Logs(ns, selector string, since time.Duration) (string, error) {
	podName, err := GetPodName(ns, selector)
	if err != nil {
		return "", err
	}
	cmd := kubectl("logs %s -n %s  --since=%s", podName, ns, since)
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}

	return output, nil
}

// GetDaemonSets returns the daemonsets of a namespace
// Return a list of daemonsets
func GetDaemonSetNames(ns string) ([]string, error) {
	var daemonsets []string
	cmd := kubectl("get daemonsets -n %s -o jsonpath={.items[*].metadata.name}", ns)
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

// DeleteCRDs deletes the CRDs by given list of crds names
func DeleteCRDs(crds []string) error {
	for _, crd := range crds {
		cmd := kubectl("delete crd %s", crd)
		_, err := shell.ExecuteCommand(cmd)
		if err != nil {
			return fmt.Errorf("error deleting crd %s: %v", crd, err)
		}
	}

	return nil
}
