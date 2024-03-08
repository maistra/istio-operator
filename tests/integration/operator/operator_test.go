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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"maistra.io/istio-operator/api/v1alpha1"
	. "maistra.io/istio-operator/pkg/util/tests/ginkgo"
	"maistra.io/istio-operator/pkg/util/tests/helm"
	"maistra.io/istio-operator/pkg/util/tests/kubectl"
	resourcecondition "maistra.io/istio-operator/pkg/util/tests/types"
	"sigs.k8s.io/yaml"
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

		istioCR v1alpha1.Istio

		istioCRYAML []byte

		istioResources = []string{
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
		When("installed via helm install", func() {
			BeforeAll(func() {
				if skipDeploy == "true" {
					Skip("Skipping the deployment of the operator and the tests")
				}

				Expect(kubectl.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created; unexpected error")

				extraArg := ""
				if ocp == "true" {
					extraArg = "--set=platform=openshift"
				}

				installArgs := fmt.Sprintf("--namespace %s --set=image=%s %s", namespace, image, extraArg)
				Eventually(helm.Install).
					WithArguments("sail-operator", filepath.Join(baseDir, "chart"), installArgs).
					Should(Succeed(), "Operator failed to be deployed; unexpected error")
			})

			It("starts successfully", func() {
				Eventually(kubectl.GetJSONCondition).
					WithArguments(namespace, "deployment", deploymentName).
					Should(ContainElement(resourceAvailable))
				Success("Operator deployment is Available")

				Expect(kubectl.GetPodPhase(namespace, "control-plane=istio-operator")).Should(Equal("Running"), "Operator failed to start; unexpected pod Phase")
				Success("Istio-operator pod is Running")
			})

			It("deploys all the CRDs", func() {
				Eventually(kubectl.GetCRDs).
					Should(ContainElements(istioResources), "Istio CRDs are not present; expected list to contain all elements")
				Success("Istio CRDs are present")
			})
		})
	})

	Describe("installation and unistallation of the istio resource", func() {
		for _, version := range istioVersions {
			// Note: This var version is needed to avoid the closure of the loop
			version := version
			var err error

			Context(fmt.Sprintf("version %s", version), func() {
				BeforeAll(func() {
					Expect(kubectl.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be created; unexpected error")
					istioCR = v1alpha1.Istio{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "operator.istio.io/v1alpha1",
							Kind:       "Istio",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: istioName,
						},
						Spec: v1alpha1.IstioSpec{
							Namespace: controlPlaneNamespace,
							Version:   version,
						},
					}

					istioCRYAML, err = yaml.Marshal(istioCR)
					Expect(err).ToNot(HaveOccurred(), "Istio CR failed to be created; unexpected error")
				})

				When("the resource is created", func() {
					Specify("successfully", func() {
						Eventually(kubectl.ApplyString).
							WithArguments(string(istioCRYAML)).
							Should(Succeed(), "Istio CR failed to be created; unexpected error")
						Success("Istio CR created")
					})

					It("updates the Istio resource status to Reconcilied and Ready", func() {
						Eventually(kubectl.GetJSONCondition).
							WithArguments(controlPlaneNamespace, "istio", istioName).
							Should(ContainElement(resourceReconcilied), "Istio it's not Reconcilied; unexpected Condition")

						Eventually(kubectl.GetJSONCondition).
							WithArguments(controlPlaneNamespace, "istio", istioName).
							Should(ContainElement(resourceReady), "Istio it's not Ready; unexpected Condition")

						Eventually(kubectl.GetPodPhase).
							WithArguments(controlPlaneNamespace, "app=istiod").
							Should(Equal("Running"), "Istiod is not running; unexpected pod Phase")
						Success("Istio resource is Reconcilied and Ready")
					})

					It("deploys istiod", func() {
						Expect(kubectl.GetDeploymentNames(controlPlaneNamespace)).
							To(Equal([]string{"istiod"}), "Istiod deployment is not present; expected list to be equal")
						Success("Istiod deployment is present")
					})

					It("deploys correct istiod image tag according to the version in the Istio CR", func() {
						// TODO: we need to add a function to get the istio version from the control panel directly
						// and compare it with the applied version
						// This is a TODO because actual version.yaml contains for example latest and not the actual version
						// Posible solution is to add actual version to the version.yaml
					})

					It("deploys the CNI DaemonSet when running on OpenShift", func() {
						if ocp == "true" {
							Eventually(kubectl.GetDaemonSetNames).
								WithArguments(namespace).
								Should(ContainElement("istio-cni-node"), "CNI DaemonSet is not deployed; expected list to contain element")

							Expect(kubectl.GetPodPhase(namespace, "k8s-app=istio-cni-node")).Should(Equal("Running"), "CNI Daemon is not running; unexpected Phase")
							Success("CNI DaemonSet is deployed in the namespace and Running")
						} else {
							Consistently(kubectl.GetDaemonSetNames).
								WithArguments(namespace).WithTimeout(30*time.Second).
								Should(BeEmpty(), "CNI DaemonSet it's present; expected list to be empty")
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
						Expect(kubectl.DeleteString(string(istioCRYAML))).Should(Succeed(), "Istio CR failed to be deleted; unexpected error")
						Success("Istio CR deleted")
					})

					It("removes everything from the namespace", func() {
						Eventually(kubectl.GetJSONList).
							WithArguments(controlPlaneNamespace).
							Should(Equal(kubectl.EmptyResourceList), "Namespace should be empty")
						Success("Namespace is empty")
					})
				})
			})
		}

		AfterAll(func() {
			By("Cleaning up the namespace")
			Eventually(kubectl.DeleteNamespace).
				WithArguments(controlPlaneNamespace).
				Should(Succeed(), "Namespace failed to be deleted; unexpected error")

			Eventually(kubectl.CheckNamespaceExist).
				WithArguments(controlPlaneNamespace).
				Should(MatchError(kubectl.ErrNotFound), "Namespace should not exist; unexpected error")
			Success("Cleanup done")
		})
	})

	AfterAll(func() {
		By("Cleaning up the operator")
		Eventually(helm.Uninstall).
			WithArguments(namespace, "sail-operator").
			Should(Succeed(), "Operator failed to be deleted; unexpected error")
		Expect(kubectl.DeleteCRDs(istioResources)).Should(Succeed(), "CRDs failed to be deleted; unexpected error")
		Success("Operator is deleted")
	})
})
