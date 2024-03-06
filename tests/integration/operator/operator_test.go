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
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integrationoperator

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"maistra.io/istio-operator/pkg/util/tests/kubectl"
	resourcecondition "maistra.io/istio-operator/pkg/util/tests/types"
)

var _ = Describe("Operator", Ordered, func() {
	SetDefaultEventuallyTimeout(120 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	var (
		resourceAvailable = resourcecondition.Condition{
			Type:   "Available",
			Status: "True",
		}
		resourceReconcilied = resourcecondition.Condition{
			Type:   "Reconciled",
			Status: "True",
		}

		resourceReady = resourcecondition.Condition{
			Type:   "Ready",
			Status: "True",
		}
	)

	Describe("operator installation", func() {
		// TODO: we  need to support testing both types of deployment for the operator, helm and olm via subscription.
		// Discuss with the team if we should add a flag to the test to enable the olm deployment and don't do that deployment in different step
		When("default helm manifest are applied", func() {
			BeforeAll(func() {
				if skipDeploy == "true" {
					Skip("Skipping the deployment of the operator and the tests")
				}
				GinkgoWriter.Println("Deploying Operator using default helm charts located in /chart folder")
				Eventually(deployOperator).Should(Succeed(), "Operator deployment should be successful")
			})

			Specify("the operator is running", func() {
				Eventually(kubectl.GetResourceCondition).WithArguments(namespace, "deployment", deploymentName).Should(ContainElement(resourceAvailable))
				GinkgoWriter.Println("Operator deployment is Available")

				Eventually(kubectl.GetPodPhase).WithArguments(namespace, "control-plane=istio-operator").Should(Equal("Running"), "Istio-operator pod should be Running")
				GinkgoWriter.Println("Istio-operator pod is Running")
			})
		})
	})

	Describe("installation and unistallation of the istio resource", func() {
		for _, version := range istioVersions {
			// Note: This var version is needed to avoid the closure of the loop
			version := version

			Context(fmt.Sprintf("is applied the istio resource with version %s", version), func() {
				BeforeAll(func() {
					Expect(kubectl.CreateNamespace(controlPlaneNamespace)).To(Succeed())
					Eventually(createIstioCR).WithArguments(version).Should(Succeed(), "Istio CR should be created")
					GinkgoWriter.Println("Istio CR created")
				})

				AfterAll(func() {
					GinkgoWriter.Println("Cleaning up")
					Eventually(kubectl.DeleteNamespace).WithArguments(controlPlaneNamespace).Should(Succeed(), "Namespace should be deleted")
					Eventually(kubectl.CheckNamespaceExist).WithArguments(controlPlaneNamespace).Should(MatchError(kubectl.ErrNotFound), "Namespace should be deleted")
					GinkgoWriter.Println("Cleanup done")
				})

				When("the Istio resource is created", func() {
					It("updates the Istio resource status to Ready and Running", func() {
						Eventually(kubectl.GetResourceCondition).WithArguments(controlPlaneNamespace, "istio", istioName).Should(ContainElement(resourceReconcilied))
						Eventually(kubectl.GetResourceCondition).WithArguments(controlPlaneNamespace, "istio", istioName).Should(ContainElement(resourceReady))
						Eventually(kubectl.GetPodPhase).WithArguments(controlPlaneNamespace, "app=istiod").Should(Equal("Running"), "Istiod should be Running")
					})

					Specify("the istio resource version match the applied version", func() {
						// TODO: we need to add a function to get the istio version from the control panel directly
						// and compare it with the applied version
						// This is a TODO because actual version.yaml contains for example latest and not the actual version
						// Posible solution is to add actual version to the version.yaml
					})

					Specify("istio resource stopped reconciling", func() {
						istiodPodName, _ := kubectl.GetPodFromLabel(controlPlaneNamespace, "app=istiod")
						Eventually(kubectl.GetPodLogs).WithArguments(controlPlaneNamespace, istiodPodName, "30s").ShouldNot(ContainSubstring("Reconciliation done"))
						GinkgoWriter.Println("Istio Operator stopped reconciling")
					})

					It("deploys istiod", func() {
						expectedDeployments := []string{"istiod"}

						deploymentsList, err := kubectl.GetDeployments(controlPlaneNamespace)
						if err != nil {
							Fail("Error getting deployments")
						}

						Expect(deploymentsList).To(Equal(expectedDeployments), "Deployments List should contains only istiod")
					})

					It("deploys the CNI DaemonSet when running on OpenShift", func() {
						if ocp == "true" {
							Eventually(kubectl.GetDaemonSets).WithArguments(namespace).Should(ContainElement("istio-cni-node"), "CNI DaemonSet should be deployed")
							Eventually(kubectl.GetPodPhase).WithArguments(namespace, "k8s-app=istio-cni-node").Should(Equal("Running"), "CNI DaemonSet should be Running")
							GinkgoWriter.Println("CNI DaemonSet is deployed in the namespace and Running")
						} else {
							Eventually(kubectl.GetDaemonSets).WithArguments(namespace).Should(BeEmpty(), "CNI DaemonSet should not be deployed on OpenShift")
							GinkgoWriter.Println("CNI DaemonSet is not deployed in the namespace because it's not OpenShift")
						}
					})
				})

				When("the Istio CR is deleted", func() {
					BeforeEach(func() {
						Eventually(deleteIstioCR).WithArguments(version).Should(Succeed(), "Istio CR should be deleted")
						GinkgoWriter.Println("Istio CR's deleted")
					})

					Specify("the namespace is empty", func() {
						Eventually(kubectl.GetAllResources).WithArguments(controlPlaneNamespace).Should(Equal(kubectl.EmptyResourceList), "Namespace should be empty")
					})
				})
			})
		}
	})

	Describe("operator uninstallation", func() {
		When("is deleted the operator manifest from the cluster", func() {
			BeforeEach(func() {
				Eventually(undeployOperator).Should(Succeed(), "Operator deployment should be deleted")
				GinkgoWriter.Println("Operator deployment is deleted")
			})

			Specify("the operator it's deleted", func() {
				Eventually(kubectl.GetPodFromLabel).WithArguments(namespace, "control-plane=istio-operator").Should(BeEmpty(), "Istio-operator pod should be deleted")
				GinkgoWriter.Println("Istio-operator pod is deleted")
			})
		})
	})
})
