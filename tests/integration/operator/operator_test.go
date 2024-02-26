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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Operator", func() {
	BeforeEach(func() {
		// Add there code to run before each test if needed
	})

	When("a fresh cluster exist", func() {
		It("the operator can be installed", func() {
			By("using the helm chart with default values")
			GinkgoWriter.Println("Deploying Operator using default helm charts located in /chart folder")

			// This is only for downstream testing, because the operator will be installed previously in CI pipeline
			if skipDeploy == "true" {
				Skip("Skipping the deployment of the operator and the tests")
			} else {
				deployOperator()
			}

			Expect(operatorIsRunning()).To(Equal(true))
		})
	})

	When("the operator is installed", func() {
		Context("a control plane can be installed and uninstalled", func() {
			istioVersions, err := getIstioVersions("/work/versions.yaml")
			if err != nil {
				Fail(fmt.Sprintf("Error getting istio versions from version.yaml file: %v", err))
			}

			It("for every istio version in version.yaml file", func() {
				for _, version := range istioVersions {
					fmt.Print("\nDeploying Istio Control Plane for version: ", version)
					deployIstioControlPlane(version)

					Expect(istioControlPlaneIsInstalledAndRunning(version)).To(Equal(true))
					Expect(checkOnlyIstioIsDeployed(controlPlaneNamespace)).To(Equal(true))

					if ocp == "true" {
						// CNI Daemon is deployed only in OCP clusters
						Expect(cniDaemonIsDeployed(namespace)).To(Equal(true))
					} else {
						Expect(cniDaemonIsDeployed(namespace)).To(Equal(false))
					}

					undeployIstioControlPlane(version)

					Expect(checkNamespaceEmpty(controlPlaneNamespace)).To(Equal(true))

					// Delete the namespace and check if deleted to be able to install the next version
					// TODO: check if this can be moved to a after each test
					Expect(deleteAndCheckNamespaceIsDeleted()).To(Equal(true))
				}
			})
		})
	})

	When("the operator is installed", func() {
		It("can be uninstalled", func() {
			GinkgoWriter.Println("Un-Deploying Operator by using helm templates generated")

			undeployOperator()

			Expect(operatorIsRunning()).To(Equal(false))
		})
	})
})
