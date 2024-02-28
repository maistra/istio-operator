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
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Operator", Ordered, func() {
	SetDefaultEventuallyTimeout(60 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	Describe("operator installation", func() {
		// TODO: we  need to support testing both types of deployment for the operator, helm and olm via subscription.
		// Discuss with the team if we should add a flag to the test to enable the olm deployment and don't do that deployment in different step
		When("helm manifest are applied", func() {
			if skipDeploy == "true" {
				Skip("Skipping the deployment of the operator and the tests")
			} else {
				Eventually(deployOperator).Should(Succeed())
			}

			It("the operator is running", func() {
				Eventually(getOperatorState).Should(Equal("Running"))
			})
		})
	})

	Describe("installation and unistallation of the istio control plane", func() {
		baseDir := filepath.Dir(filepath.Dir(filepath.Dir(wd)))
		istioVersions, err := getIstioVersions(filepath.Join(baseDir, "versions.yaml"))
		if err != nil {
			Fail(fmt.Sprintf("Error getting istio versions from version.yaml file: %v", err))
		}

		for _, version := range istioVersions {
			Context(fmt.Sprintf("for supported version %s", version), func() {
				When(fmt.Sprintf("the istio control plane is installed with version %s", version), func() {
					BeforeAll(func() {
						Eventually(createNamespaceIfNotExists).WithArguments(controlPlaneNamespace).Should(Succeed())
						Eventually(deployIstioControlPlane).WithArguments(version).Should(Succeed())

						DeferCleanup(func() {
							Eventually(deleteNamespace).WithArguments(controlPlaneNamespace).Should(Succeed())
							Eventually(namespaceIsDeleted).WithArguments(controlPlaneNamespace).Should(Equal(true))
						})
					})

					It("updates the Istio resource status to Ready and Running", func() {
						Eventually(waitForIstioCondition).WithArguments(command, istioName, "Reconciled").Should(Succeed())
						Eventually(waitForIstioCondition).WithArguments(command, istioName, "Ready").Should(Succeed())

						istiodPodName, err := getPodNameFromLabel(controlPlaneNamespace, "app=istiod")
						if err != nil {
							Fail("Error getting pod name from deployment")
						}

						Eventually(getPodPhase).WithArguments(controlPlaneNamespace, istiodPodName).Should(Equal("Running"))
					})

					It("the istio resource version match the installed version", func() {
						Expect(getInstalledIstioVersion()).Should(Equal(version))
					})

					It("istio resource stopped reconciling", func() {
						Eventually(checkIstioStoppedReconciling).WithTimeout(120 * time.Second).Should(Succeed())
					})

					It("only istiod is deployed", func() {
						Expect(checkOnlyIstioIsDeployed(controlPlaneNamespace)).To(Equal(true))
					})

					It("deploys the CNI DaemonSet when running on OpenShift", func() {
						if ocp == "true" {
							Eventually(cniDaemonIsDeployed).WithArguments(namespace).Should(Equal(true))

							podName, err := getPodNameFromLabel(namespace, "k8s-app=istio-cni-node")
							if err != nil {
								Fail("Error getting pod name from deployment")
							}
							Eventually(getPodPhase).WithArguments(namespace, podName).Should(Equal("Running"))

						} else {
							Eventually(cniDaemonIsDeployed).WithArguments(namespace).Should(Equal(false))
						}
					})

					When("the Istio CR is deleted", func() {
						BeforeEach(func() {
							Eventually(undeployIstioControlPlane).WithArguments(version).Should(Succeed())
						})

						It("the undeploys the istio control plane", func() {
							Eventually(namespaceEmpty).WithArguments(controlPlaneNamespace).Should(Equal(true))
						})
					})
				})
			})
		}
	})

	Describe("operator uninstallation", func() {
		When("is deleted the operator manifest from the cluster", func() {
			BeforeEach(func() {
				Eventually(undeployOperator).Should(Succeed())
			})

			It("the operator it's deleted", func() {
				Eventually(podExist).WithArguments(namespace, "control-plane=istio-operator").Should(Equal(false))
			})
		})
	})
})
