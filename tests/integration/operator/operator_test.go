//go:build e2e

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
	"regexp"
	"strings"
	"time"

	. "github.com/istio-ecosystem/sail-operator/pkg/util/tests/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/util/tests/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/util/tests/kubectl"
	r "github.com/istio-ecosystem/sail-operator/pkg/util/tests/types"
	"github.com/istio-ecosystem/sail-operator/tests/integration/supportedversion"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"istio.io/istio/pkg/ptr"
)

var istiodVersionRegex = regexp.MustCompile(`Version:"(\d+\.\d+(\.\d+|-\w+))`)

var _ = Describe("Operator", Ordered, func() {
	SetDefaultEventuallyTimeout(120 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	var (
		resourceAvailable = r.Condition{
			Type:   "Available",
			Status: "True",
		}
		resourceReconciled = r.Condition{
			Type:   "Reconciled",
			Status: "True",
		}

		resourceReady = r.Condition{
			Type:   "Ready",
			Status: "True",
		}

		crdEstablished = r.Condition{
			Type:   "Established",
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

		BeforeAll(func() {
			Expect(kubectl.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created")

			extraArg := ""
			if ocp {
				extraArg = "--set=platform=openshift"
			}

			Expect(helm.Install("sail-operator", filepath.Join(baseDir, "chart"), "--namespace "+namespace, "--set=image="+image, extraArg)).
				To(Succeed(), "Operator failed to be deployed")
		})

		It("deploys all the CRDs", func() {
			Eventually(kubectl.GetCRDs).
				Should(ContainElements(crds), "Istio CRDs are not present; expected list to contain all elements")
			Success("Istio CRDs are present")
		})

		It("updates the CRDs status to Established", func() {
			for _, crd := range crds {
				Eventually(kubectl.GetConditions).WithArguments(namespace, "crd", crd).Should(ContainElement(crdEstablished), "CRD is not Established")
			}
			Success("CRDs are Established")
		})

		Specify("istio crd is present", func() {
			// When the operator runs in OCP cluster, the CRD is created but not available at the moment
			Eventually(kubectl.GetResourceList).
				WithArguments("", "istio").
				Should(Equal(kubectl.EmptyResourceList), "Istio CRD is not present; expected to not fail and return an empty list of Istio CR")
			Success("Istio CRD is present")
		})

		It("starts successfully", func() {
			Eventually(kubectl.GetConditions).
				WithArguments(namespace, "deployment", deploymentName).
				Should(ContainElement(resourceAvailable), "Operator deployment is not Available; unexpected Condition")
			Success("Operator deployment is Available")

			Expect(kubectl.GetPodPhase(namespace, "control-plane=sail-operator")).To(Equal("Running"), "Operator failed to start; unexpected pod Phase")
			Success("sail-operator pod is Running")
		})

		AfterAll(func() {
			if CurrentSpecReport().Failed() {
				LogDebugInfo()
			}
		})
	})

	Describe("given Istio version", func() {
		for _, version := range supportedversion.List {
			// Note: This var version is needed to avoid the closure of the loop
			version := version

			Context(version.Name, func() {
				BeforeAll(func() {
					Expect(kubectl.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
					Expect(kubectl.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
				})

				When("the IstioCNI CR is created", func() {
					BeforeAll(func() {
						yaml := `
apiVersion: operator.istio.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
						yaml = fmt.Sprintf(yaml, version.Name, istioCniNamespace)
						log("IstioCNI YAML:", indent(2, yaml))
						Expect(kubectl.ApplyString(yaml)).To(Succeed(), "IstioCNI creation failed")
						Success("IstioCNI created")
					})

					It("deploys the CNI DaemonSet", func() {
						Eventually(kubectl.GetDaemonSets).
							WithArguments(istioCniNamespace).
							Should(ContainElement("istio-cni-node"), "CNI DaemonSet is not deployed; expected list to contain element")

						Eventually(func(g Gomega) {
							numberAvailable, err := kubectl.GetDaemonSetStatusField(istioCniNamespace, "istio-cni-node", "numberAvailable")
							g.Expect(err).ToNot(HaveOccurred(), "Error getting numberAvailable field from istio-cni-node DaemonSet")

							currentNumberScheduled, err := kubectl.GetDaemonSetStatusField(istioCniNamespace, "istio-cni-node", "currentNumberScheduled")
							g.Expect(err).ToNot(HaveOccurred(), "Error getting currentNumberScheduled field from istio-cni-node DaemonSet")

							g.Expect(numberAvailable).
								To(Equal(currentNumberScheduled), "CNI DaemonSet Pods not Available; expected numberAvailable to be equal to currentNumberScheduled")
						}).Should(Succeed(), "CNI DaemonSet Pods are not Available")
						Success("CNI DaemonSet is deployed in the namespace and Running")
					})

					It("updates the status to Reconciled", func() {
						Eventually(kubectl.GetConditions).
							WithArguments(istioCniNamespace, "istiocni", istioCniName).
							Should(ContainElement(resourceReconciled), "IstioCNI is not Reconciled; unexpected Condition")
						Success("IstioCNI is Reconciled")
					})

					It("updates the status to Ready", func() {
						Eventually(kubectl.GetConditions).
							WithArguments(istioCniNamespace, "istiocni", istioCniName).
							Should(ContainElement(resourceReady), "IstioCNI is not Ready; unexpected Condition")
						Success("IstioCNI is Ready")
					})

					It("doesn't continuously reconcile the IstioCNI CR", func() {
						Eventually(kubectl.Logs).
							WithArguments(namespace, "deploy/"+deploymentName, ptr.Of(30*time.Second)).
							ShouldNot(ContainSubstring("Reconciliation done"), "Istio Operator is continuously reconciling")
						Success("Istio Operator stopped reconciling")
					})
				})

				When("the Istio CR is created", func() {
					BeforeAll(func() {
						istioYAML := `
apiVersion: operator.istio.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
						istioYAML = fmt.Sprintf(istioYAML, version.Name, controlPlaneNamespace)
						log("Istio YAML:", indent(2, istioYAML))
						Expect(kubectl.ApplyString(istioYAML)).
							To(Succeed(), "Istio CR failed to be created")
						Success("Istio CR created")
					})

					It("updates the Istio CR status to Reconciled", func() {
						Eventually(kubectl.GetConditions).
							WithArguments(controlPlaneNamespace, "istio", istioName).
							Should(ContainElement(resourceReconciled), "Istio is not Reconciled; unexpected Condition")
						Success("Istio CR is Reconciled")
					})

					It("updates the Istio CR status to Ready", func() {
						Eventually(kubectl.GetConditions).
							WithArguments(controlPlaneNamespace, "istio", istioName).
							Should(ContainElement(resourceReady), "Istio is not Ready; unexpected Condition")
						Success("Istio CR is Ready")
					})

					It("deploys istiod", func() {
						Expect(kubectl.GetDeployments(controlPlaneNamespace)).
							To(Equal([]string{"istiod"}), "Istiod deployment is not present; unexpected list of deployments")
						Success("Istiod deployment is present")

						Expect(getVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
					})

					It("doesn't continuously reconcile the Istio CR", func() {
						Eventually(kubectl.Logs).
							WithArguments(namespace, "deploy/"+deploymentName, ptr.Of(30*time.Second)).
							ShouldNot(ContainSubstring("Reconciliation done"), "Istio Operator is continuously reconciling")
						Success("Istio Operator stopped reconciling")
					})
				})

				When("the Istio CR is deleted", func() {
					BeforeEach(func() {
						Expect(kubectl.Delete(controlPlaneNamespace, "istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Success("Istio CR deleted")
					})

					It("removes everything from the namespace", func() {
						Eventually(kubectl.GetResourceList).
							WithArguments(controlPlaneNamespace, "all").
							Should(Equal(kubectl.EmptyResourceList), "Namespace should be empty")
						Success("Namespace is empty")
					})
				})

				When("the IstioCNI CR is deleted", func() {
					BeforeEach(func() {
						Expect(kubectl.Delete(istioCniNamespace, "istiocni", istioCniName)).To(Succeed(), "IstioCNI CR failed to be deleted")
						Success("IstioCNI deleted")
					})

					It("removes everything from the CNI namespace", func() {
						Eventually(kubectl.GetResourceList).
							WithArguments(istioCniNamespace, "all").
							Should(Equal(kubectl.EmptyResourceList), "CNI namespace isn't empty")
						Success("CNI namespace is empty")
					})
				})
			})
		}

		AfterAll(func() {
			if CurrentSpecReport().Failed() {
				LogDebugInfo()
			}
			By("Cleaning up the Istio namespace")
			Expect(kubectl.DeleteNamespace(controlPlaneNamespace)).
				To(Succeed(), "Namespace failed to be deleted")

			By("Cleaning up the IstioCNI namespace")
			Expect(kubectl.DeleteNamespace(istioCniNamespace)).
				To(Succeed(), "CNI namespace deletion failed")

			Eventually(kubectl.CheckNamespaceExist).
				WithArguments(controlPlaneNamespace).
				Should(MatchError(kubectl.ErrNotFound), "Namespace should not exist")
			Success("Cleanup done")
		})
	})

	AfterAll(func() {
		By("Deleting any left-over Istio and IstioRevision resources")
		Expect(forceDeleteIstioResources()).To(Succeed())
		Success("Resources deleted")

		By("Uninstalling the operator")
		Expect(helm.Uninstall("sail-operator", "--namespace "+namespace)).
			To(Succeed(), "Operator failed to be deleted")
		Success("Operator uninstalled")

		By("Deleting the CRDs")
		Expect(kubectl.DeleteCRDs(crds)).To(Succeed(), "CRDs failed to be deleted")
		Success("CRDs deleted")
	})
})

func getVersionFromIstiod() (string, error) {
	output, err := kubectl.Exec(controlPlaneNamespace, "deploy/istiod", "pilot-discovery version")
	if err != nil {
		return "", fmt.Errorf("error getting version from istiod: %v", err)
	}

	matches := istiodVersionRegex.FindStringSubmatch(output)
	if len(matches) > 1 && matches[1] != "" {
		return matches[1], nil
	}
	return "", fmt.Errorf("error getting version from istiod: version not found in output: %s", output)
}

func forceDeleteIstioResources() error {
	// This is a workaround to delete the Istio CRs that are left in the cluster
	// This will be improved by splitting the tests into different Nodes with their independent setups and cleanups
	err := kubectl.ForceDelete("", "istio", istioName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete %s CR: %v", "istio", err)
	}

	err = kubectl.ForceDelete("", "istiorevision", "default")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete %s CR: %v", "istiorevision", err)
	}

	err = kubectl.Delete("", "istiocni", istioCniName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete %s CR: %v", "istiocni", err)
	}

	return nil
}

func indent(level int, str string) string {
	indent := strings.Repeat(" ", level)
	return indent + strings.ReplaceAll(str, "\n", "\n"+indent)
}

func log(a ...any) {
	GinkgoWriter.Println(a...)
}

func LogDebugInfo() {
	// General debugging information to help diagnose the failure
	resource, err := kubectl.GetYAML(controlPlaneNamespace, "istio", istioName)
	if err != nil {
		GinkgoWriter.Println("Error getting Istio CR: ", err)
	}
	GinkgoWriter.Println("Istio CR: \n", resource)

	output, err := kubectl.GetPods(controlPlaneNamespace, "-o wide")
	if err != nil {
		GinkgoWriter.Println("Error getting pods: ", err)
	}
	GinkgoWriter.Println("Pods in Istio CR namespace: \n", output)

	logs, err := kubectl.Logs(namespace, "deploy/"+deploymentName, ptr.Of(120*time.Second))
	if err != nil {
		GinkgoWriter.Println("Error getting logs from the operator: ", err)
	}
	GinkgoWriter.Println("Logs from sail-operator pod: \n", logs)

	logs, err = kubectl.Logs(controlPlaneNamespace, "deploy/istiod", ptr.Of(120*time.Second))
	if err != nil {
		GinkgoWriter.Println("Error getting logs from the istiod: ", err)
	}
	GinkgoWriter.Println("Logs from istiod pod: \n", logs)
}
