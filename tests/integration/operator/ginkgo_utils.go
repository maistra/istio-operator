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
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/util/tests/kubectl"
	"github.com/istio-ecosystem/sail-operator/pkg/util/tests/shell"
	. "github.com/onsi/ginkgo/v2"
)

func LogFailure() {
	// General debugging information to help diagnose the failure
	GinkgoWriter.Println("********* Failed specs while running: ", CurrentSpecReport().FailureLocation())
	resource, err := kubectl.GetYAML(controlPlaneNamespace, "istio", istioName)
	if err != nil {
		GinkgoWriter.Println("Error getting Istio resource: ", err)
	}

	GinkgoWriter.Println("Istio resource: ", resource)

	output, err := shell.ExecuteCommand(fmt.Sprintf("kubectl get pods -n %s -o wide", controlPlaneNamespace))
	if err != nil {
		GinkgoWriter.Println("Error getting pods: ", err)
	}

	GinkgoWriter.Println("Pods in istio resource namespace: ", output)

	logs, err := kubectl.Logs(namespace, "control-plane=sail-operator", 120*time.Second)
	if err != nil {
		GinkgoWriter.Println("Error getting logs from the operator: ", err)
	}

	GinkgoWriter.Println("Logs from sail-operator pod: ", logs)

	logs, err = kubectl.Logs(controlPlaneNamespace, "app=istiod", 120*time.Second)
	if err != nil {
		GinkgoWriter.Println("Error getting logs from the istiod: ", err)
	}

	GinkgoWriter.Println("Logs from istiod pod: ", logs)
}
