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
	"path/filepath"
	"time"

	. "github.com/istio-ecosystem/sail-operator/pkg/util/tests/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/util/tests/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/util/tests/kubectl"
	r "github.com/istio-ecosystem/sail-operator/pkg/util/tests/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Operator", Ordered, func() {
	SetDefaultEventuallyTimeout(120 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	var (
		resourceAvailable = r.Condition{
			Type:   "Available",
			Status: "True",
		}
		resourceReconcilied = r.Condition{
			Type:   "Reconciled",
			Status: "True",
		}

		resourceReady = r.Condition{
			Type:   "Ready",
			Status: "True",
		}

		crds = []string{
			// TODO: Find an alternative to this list
			"authorizationpolicies.security.istio.io",
			"destinationrules.networking.istio.io",
			"envoyfilters.networking.istio.io",
			"gateways.networking.istio.io",
			"istiorevisions.operator.istio.io",
			"istios.operator.istio.io",
			"peerauthentications.security.istio.io",
			"proxyconfigs.networking.istio.io",
			"requestauthentications.security.istio.io",
			"serviceentries.networking.istio.io",
			"sidecars.networking.istio.io",
			"telemetries.telemetry.istio.io",
			"virtualservices.networking.istio.io",
			"wasmplugins.extensions.istio.io",
			"workloadentries.networking.istio.io",
			"workloadgroups.networking.istio.io",
		}
	)

	Describe("installation", func() {
		// TODO: we  need to support testing both types of deployment for the operator, helm and olm via subscription.
		// Discuss with the team if we should add a flag to the test to enable the olm deployment and don't do that deployment in different step
		if skipDeploy {
			Skip("Skipping the deployment of the operator")
		}

		When("installed via helm install", func() {
			BeforeAll(func() {
				Expect(kubectl.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created")

				extraArg := ""
				if ocp {
					extraArg = "--set=platform=openshift"
				}

				Expect(helm.Install("sail-operator", filepath.Join(baseDir, "chart"), "--namespace "+namespace, "--set=image="+image, extraArg)).
					Should(Succeed(), "Operator failed to be deployed")
			})

			It("starts successfully", func() {
				Eventually(kubectl.GetConditions).
					WithArguments(namespace, "deployment", deploymentName).
					Should(ContainElement(resourceAvailable), "Operator deployment is not Available; unexpected Condition")
				Success("Operator deployment is Available")

				Expect(kubectl.GetPodPhase(namespace, "control-plane=sail-operator")).Should(Equal("Running"), "Operator failed to start; unexpected pod Phase")
				Success("sail-operator pod is Running")
			})

			It("deploys all the CRDs", func() {
				Eventually(kubectl.GetCRDs).
					Should(ContainElements(crds), "Istio CRDs are not present; expected list to contain all elements")
				Success("Istio CRDs are present")
			})

			AfterAll(func() {
				if CurrentSpecReport().Failed() {
					LogFailure()
				}
			})
		})
	})

	Describe("installation and unistallation of the istio resource", func() {
		for _, version := range istioVersions {
			// Note: This var version is needed to avoid the closure of the loop
			version := version

			Context(fmt.Sprintf("version %s", version), func() {
				BeforeAll(func() {
					Expect(kubectl.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be created")
				})

				When("the resource is created", func() {
					BeforeAll(func() {
						istioYAML := `
apiVersion: operator.istio.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
						istioYAML = fmt.Sprintf(istioYAML, version, controlPlaneNamespace)
						fmt.Printf("Istio CR YAML: %s\n", istioYAML)
						Expect(kubectl.ApplyString(istioYAML)).
							Should(Succeed(), "Istio CR failed to be created")
						Success("Istio CR created")
					})

					It("updates the Istio resource status to Reconcilied and Ready", func() {
						Eventually(kubectl.GetConditions).
							WithArguments(controlPlaneNamespace, "istio", istioName).
							Should(ContainElement(resourceReconcilied), "Istio is not Reconcilied; unexpected Condition")

						Eventually(kubectl.GetConditions).
							WithArguments(controlPlaneNamespace, "istio", istioName).
							Should(ContainElement(resourceReady), "Istio is not Ready; unexpected Condition")

						Success("Istio resource is Reconcilied and Ready")
					})

					It("deploys istiod", func() {
						Expect(kubectl.GetDeployments(controlPlaneNamespace)).
							To(Equal([]string{"istiod"}), "Istiod deployment is not present; unexpected list of deployments")
						Success("Istiod deployment is present")

						// TODO: we need to add a function to get the istio version from the control panel directly
						// and compare it with the applied version
						// This is a TODO because actual version.yaml contains for example latest and not the actual version
						// Posible solution is to add actual version to the version.yaml
					})

					It("deploys the CNI DaemonSet when running on OpenShift", func() {
						if ocp {
							Eventually(kubectl.GetDaemonSets).
								WithArguments(namespace).
								Should(ContainElement("istio-cni-node"), "CNI DaemonSet is not deployed; expected list to contain element")

							Expect(kubectl.GetPodPhase(namespace, "k8s-app=istio-cni-node")).Should(Equal("Running"), "CNI Daemon is not running; unexpected Phase")
							Success("CNI DaemonSet is deployed in the namespace and Running")
						} else {
							Consistently(kubectl.GetDaemonSets).
								WithArguments(namespace).WithTimeout(30*time.Second).
								Should(BeEmpty(), "CNI DaemonSet is present; expected list to be empty")
							Success("CNI DaemonSet is not deployed in the namespace because it's not OpenShift")
						}
					})

					It("doesn't continuously reconcile the istio resource", func() {
						Eventually(kubectl.Logs).
							WithArguments(controlPlaneNamespace, "app=istiod", 30*time.Second).
							ShouldNot(ContainSubstring("Reconciliation done"), "Istio Operator is continuously reconciling")
						Success("Istio Operator stopped reconciling")
					})
				})

				When("the Istio CR is deleted", func() {
					BeforeEach(func() {
						Expect(kubectl.Delete(controlPlaneNamespace, "istio", istioName)).To(Succeed(), "Istiod deployment failed to be deleted")
						Success("Istio CR deleted")
					})

					It("removes everything from the namespace", func() {
						Eventually(kubectl.GetResourceList).
							WithArguments(controlPlaneNamespace).
							Should(Equal(kubectl.EmptyResourceList), "Namespace should be empty")
						Success("Namespace is empty")
					})
				})
			})
		}

		AfterAll(func() {
			if CurrentSpecReport().Failed() {
				LogFailure()
			}
			By("Cleaning up the namespace")
			Expect(kubectl.DeleteNamespace(controlPlaneNamespace)).
				Should(Succeed(), "Namespace failed to be deleted")

			Eventually(kubectl.CheckNamespaceExist).
				WithArguments(controlPlaneNamespace).
				Should(MatchError(kubectl.ErrNotFound), "Namespace should not exist")
			Success("Cleanup done")
		})
	})

	AfterAll(func() {
		By("Cleaning up the operator")
		Expect(helm.Uninstall("sail-operator", "--namespace "+namespace)).
			Should(Succeed(), "Operator failed to be deleted")
		Expect(kubectl.DeleteCRDs(crds)).To(Succeed(), "CRDs failed to be deleted")
		Success("Operator is deleted")
	})
})
