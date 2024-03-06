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
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/util/tests/kubectl"
	resourcecondition "maistra.io/istio-operator/pkg/util/tests/types"
)

var _ = Describe("Operator", Ordered, func() {
	SetDefaultEventuallyTimeout(120 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	var resourceAvailable, resourceReconcilied, resourceReady resourcecondition.Conditions

	Describe("operator installation", func() {
		// TODO: we  need to support testing both types of deployment for the operator, helm and olm via subscription.
		// Discuss with the team if we should add a flag to the test to enable the olm deployment and don't do that deployment in different step
		When("default helm manifest are applied", func() {
			BeforeEach(func() {
				resourceAvailable = resourcecondition.Conditions{
					Type:   "Available",
					Status: "True",
				}

				if skipDeploy == "true" {
					Skip("Skipping the deployment of the operator and the tests")
				}
				GinkgoWriter.Println("Deploying Operator using default helm charts located in /chart folder")
				Eventually(deployOperator).Should(Succeed(), "Operator deployment should be successful")
			})

			Specify("the operator is running", func() {
				Eventually(kubectl.GetResourceCondition).WithArguments(namespace, "deployment", deploymentName).Should(ContainElement(resourceAvailable), "Operator deployment should be Available")
				GinkgoWriter.Println("Operator deployment is Available")

				podName, err := kubectl.GetPodFromLabel(namespace, "control-plane=istio-operator")
				GinkgoWriter.Println("Istio Operator pod name: ", podName)
				if err != nil {
					Fail("Error getting pods from label")
				}

				Eventually(kubectl.GetPodPhase).WithArguments(namespace, podName).Should(Equal("Running"), "Istio-operator pod should be Running")
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

					resourceReconcilied = resourcecondition.Conditions{
						Type:   "Reconciled",
						Status: "True",
					}

					resourceReady = resourcecondition.Conditions{
						Type:   "Ready",
						Status: "True",
					}
				})

				AfterAll(func() {
					Eventually(kubectl.DeleteNamespace).WithArguments(controlPlaneNamespace).Should(Succeed(), "Namespace should be deleted")
					Eventually(kubectl.GetNamespace).WithArguments(controlPlaneNamespace).Should(Equal(fmt.Sprintf("namespace %s not found", controlPlaneNamespace)))
					GinkgoWriter.Println("Cleanup done")
				})

				When("the Istio resource is created", func() {
					It("updates the Istio resource status to Ready and Running", func() {
						Eventually(func(g Gomega) {
							g.Expect(kubectl.GetResourceCondition(controlPlaneNamespace, "istio", istioName)).Should(ContainElement(resourceReconcilied))
						}).Should(Succeed(), "Istio CR should be reconciled")

						Eventually(func(g Gomega) {
							g.Expect(kubectl.GetResourceCondition(controlPlaneNamespace, "istio", istioName)).Should(ContainElement(resourceReady))
						}).Should(Succeed(), "Istio CR should be Ready")

						istiodPodName, _ := kubectl.GetPodFromLabel(controlPlaneNamespace, "app=istiod")
						GinkgoWriter.Println("Istiod pod name: ", istiodPodName)

						Eventually(kubectl.GetPodPhase).WithArguments(controlPlaneNamespace, istiodPodName).Should(Equal("Running"), "Istiod should be Running")
					})

					Specify("the istio resource version match the applied version", func() {
						var istio v1alpha1.Istio
						output, _ := kubectl.GetResource(controlPlaneNamespace, "istio", istioName)

						err := json.Unmarshal([]byte(output), &istio)
						if err != nil {
							Fail("Error unmarshalling the IstioCr")
						}

						Expect(istio.Spec.Version).To(Equal(version), "Istio CR version should match the applied version")
					})

					It("istio resource stopped reconciling", func() {
						istiodPodName, _ := kubectl.GetPodFromLabel(controlPlaneNamespace, "app=istiod")
						Eventually(kubectl.GetPodLogs).WithArguments(controlPlaneNamespace, istiodPodName, "30s").ShouldNot(ContainSubstring("Reconciliation done"))
						GinkgoWriter.Println("Istio Operator stopped reconciling")
					})

					It("only istiod is deployed", func() {
						expectedDeployments := []string{"istiod"}

						deploymentsList, err := kubectl.GetDeployments(controlPlaneNamespace)
						if err != nil {
							Fail("Error getting deployments")
						}

						Expect(deploymentsList).To(Equal(expectedDeployments), "Deployments List should contains only istiod")
					})

					It("deploys the CNI DaemonSet when running on OpenShift", func() {
						daemonsetsList, err := kubectl.GetDaemonSets(namespace)
						if err != nil {
							Fail(fmt.Sprintf("Error getting daemonsets: %s", err))
						}

						if ocp == "true" {
							Expect(daemonsetsList).To(ContainElement("istio-cni-node"), "DaemonSet List should contains the CNI DaemonSet")

							podName, err := kubectl.GetPodFromLabel(namespace, "k8s-app=istio-cni-node")
							if err != nil {
								Fail("Error getting pod name from deployment")
							}

							Eventually(kubectl.GetPodPhase).WithArguments(namespace, podName).Should(Equal("Running"), "CNI DaemonSet should be Running")
							GinkgoWriter.Println("CNI DaemonSet is deployed in the namespace and Running")
						} else {
							Expect(daemonsetsList).To(BeEmpty(), "DaemonSet List should empty because it's not OpenShift")
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
						emptyResourceList := resourcecondition.ResourceList{
							APIVersion: "v1",
							Items:      []interface{}{},
							Kind:       "List",
							Metadata: struct {
								ResourceVersion string `json:"resourceVersion"`
							}{
								ResourceVersion: "",
							},
						}

						Eventually(func() error {
							output, err := kubectl.GetResource(controlPlaneNamespace, "all", "")
							if err != nil {
								return err
							}

							var resourceList resourcecondition.ResourceList
							err = json.Unmarshal([]byte(output), &resourceList)
							if err != nil {
								return err
							}

							if !reflect.DeepEqual(resourceList, emptyResourceList) {
								// Return an error to indicate the comparison failed, allowing for retry
								return fmt.Errorf("namespace should be empty")
							}

							return nil
						}).Should(Succeed(), "Namespace should be empty")
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
