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

	"github.com/istio-ecosystem/sail-operator/tests/e2e/operator/util/shell"
)

const DefaultBinary = "kubectl"

var (
	ErrNotFound       = errors.New("resource was not found")
	EmptyResourceList = ResourceList{
		APIVersion: "v1",
		Items:      []interface{}{},
		Kind:       "List",
		Metadata:   Metadata{ResourceVersion: ""},
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

// GetConditions returns the condition of a resource
func GetConditions(ns, kind, name string) ([]Condition, error) {
	output, err := GetJSON(ns, kind, name)
	if err != nil {
		return []Condition{}, err
	}

	var resource Resource
	err = json.Unmarshal([]byte(output), &resource)
	if err != nil {
		return []Condition{}, err
	}

	return resource.Status.Conditions, nil
}

// GetPodPhase returns the phase of a pod
func GetPodPhase(ns, selector string) (string, error) {
	podName, err := GetPodName(ns, selector)
	if err != nil {
		return "", err
	}

	output, err := GetJSON(ns, "pod", podName)
	if err != nil {
		return "", err
	}

	var resource Resource
	err = json.Unmarshal([]byte(output), &resource)
	if err != nil {
		return "", err
	}

	return resource.Status.Phase, nil
}

// GetCRDs returns all the CRDs names in a list
func GetCRDs() ([]string, error) {
	cmd := kubectl("get crds -o name")
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return []string{}, fmt.Errorf("error getting crds: %v", err)
	}
	return extractNames(output), nil
}

// GetResourceList returns a json list of the resources of a namespace by resource name
func GetResourceList(ns, kind string) (ResourceList, error) {
	// TODO: improve the function to get all the resources
	output, err := GetJSON(ns, kind, "")
	if err != nil {
		return EmptyResourceList, err
	}

	var resourceList ResourceList
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
// - kind: type of the resource
// - name: name of the resource
func GetJSON(ns, kind, name string) (string, error) {
	cmd := kubectl("get %s %s %s -o json", kind, name, nsflag(ns))
	return shell.ExecuteCommand(cmd)
}

// GetYAML returns the yaml of a resource
// Arguments:
// - ns: namespace
// - kind: type of the resource
// - name: name of the resource
func GetYAML(ns, kind, name string) (string, error) {
	cmd := kubectl("get %s %s %s -o yaml", kind, name, nsflag(ns))
	return shell.ExecuteCommand(cmd)
}

// GetPodName returns the pod name from a selector, if there is more than one pod, it will return an error
func GetPodName(ns, selector string) (string, error) {
	podList, err := GetPodsNames(ns, selector)
	if err != nil {
		return "", err
	}
	if len(podList) > 1 {
		return "", fmt.Errorf("more than one pod found with selector %s", selector)
	}
	if len(podList) == 0 {
		return "", fmt.Errorf("no pod found with selector %s", selector)
	}

	return podList[0], nil
}

// GetPodsNames returns the pods names from a given selector
func GetPodsNames(ns, selector string) ([]string, error) {
	output, err := GetPods(ns, "-l", selector, "-o name")
	if err != nil {
		return nil, err
	}
	return extractNames(output), nil
}

// GetPods returns the pods of a namespace
func GetPods(ns string, args ...string) (string, error) {
	cmd := kubectl("get pods %s %s", nsflag(ns), strings.Join(args, " "))
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting pods: %v, output: %s", err, output)
	}

	return output, nil
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

// GetDeployments returns the deployments of a namespace
func GetDeployments(ns string) ([]string, error) {
	cmd := kubectl("get deployments %s -o name", nsflag(ns))
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("error getting deployments names: %v, output: %s", err, output)
	}
	return extractNames(output), nil
}

// Delete deletes a resource based on the namespace, kind and the name
func Delete(ns, kind, name string) error {
	cmd := kubectl("delete %s %s %s", kind, name, nsflag(ns))
	_, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("error deleting deployment: %v", err)
	}

	return nil
}

// Patch patches a resource.
func Patch(ns, kind, name, patchType, patch string) error {
	cmd := kubectl(`patch %s %s %s --type=%s -p=%q`, kind, name, prepend("-n", ns), patchType, patch)
	_, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("error patching resource: %v", err)
	}
	return nil
}

// ForceDelete deletes a resource by removing its finalizers.
func ForceDelete(ns, kind, name string) error {
	if err := Patch(ns, kind, name, "json", `[{"op": "remove", "path": "/metadata/finalizers"}]`); err != nil {
		return err
	}
	return Delete(ns, kind, name)
}

// Logs returns the logs of a deployment
// Arguments:
// - ns: namespace
// - pod: the pod name, "kind/name", or "-l labelselector"
// - Since: time range
func Logs(ns, pod string, since *time.Duration) (string, error) {
	cmd := kubectl("logs %s %s %s", pod, nsflag(ns), sinceFlag(since))
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}
	return output, nil
}

func sinceFlag(since *time.Duration) string {
	if since == nil {
		return ""
	}
	return "--since=" + since.String()
}

// Exec executes a command in the pod.
func Exec(ns, pod string, command string) (string, error) {
	cmd := kubectl("exec %s %s -- %s", pod, nsflag(ns), command)
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}
	return output, nil
}

// GetDaemonSets returns the daemonsets of a namespace
// Return a list of daemonsets
func GetDaemonSets(ns string) ([]string, error) {
	cmd := kubectl("get daemonsets %s -o name", nsflag(ns))
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("error getting daemonsets names: %v, output: %s", err, output)
	}
	return extractNames(output), nil
}

// GetDaemonSetStatusField returns the status field requested of a daemonset
// Parameters examples:
// - numberAvailable
// - currentNumberScheduled
// - desiredNumberScheduled
func GetDaemonSetStatusField(ns, daemonsetName, field string) (string, error) {
	cmd := kubectl("get daemonset %s %s -o jsonpath='{.status.%s}'", daemonsetName, nsflag(ns), field)
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("error getting daemonset %s: %v", daemonsetName, err)
	}

	return output, nil
}

func extractNames(str string) []string {
	var names []string
	for _, name := range strings.Split(str, "\n") {
		if name != "" {
			// -o name return the resource name with the kind, for example: deployment.apps/istiod
			names = append(names, strings.Split(name, "/")[1])
		}
	}

	return names
}

// prepend prepends the prefix, but only if str is not empty
func prepend(prefix, str string) string {
	if str == "" {
		return str
	}
	return prefix + str
}

func nsflag(ns string) string {
	if ns == "" {
		return "--all-namespaces"
	}
	return "-n " + ns
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
