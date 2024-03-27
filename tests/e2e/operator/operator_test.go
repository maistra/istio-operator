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

package operator

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/test/util/supportedversion"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/operator/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/operator/util/helm"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/operator/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var (
	istiodVersionRegex = regexp.MustCompile(`Version:"(\d+\.\d+(\.\d+|-\w+))`)
	deployment         = &appsv1.Deployment{}
	crd                = &apiextensionsv1.CustomResourceDefinition{}
	crdList            = &apiextensionsv1.CustomResourceDefinitionList{}
	daemonset          = &appsv1.DaemonSet{}
	cni                = &v1alpha1.IstioCNI{}
	istio              = &v1alpha1.Istio{}
	sailCRDs           = []string{
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

var _ = Describe("Operator", Ordered, func() {
	SetDefaultEventuallyTimeout(120 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

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

		It("deploys all the CRDs", func(ctx SpecContext) {
			Eventually(func(g Gomega) {
				g.Expect(cl.List(ctx, crdList)).To(Succeed(), "Error getting CRDs list from the cluster")
				crdNames := getCRDsName()
				g.Expect(crdNames).To(ContainElements(sailCRDs), "Istio CRDs are not present; expected list to contain all elements")
			}).Should(Succeed(), "Unexpected error getting CRDs from the cluster")
			Success("Istio CRDs are present")
		})

		It("updates the CRDs status to Established", func(ctx SpecContext) {
			for _, crdName := range sailCRDs {
				Eventually(getObject).WithArguments(ctx, cl, key(crdName), crd).
					Should(HaveCondition(apiextensionsv1.Established, metav1.ConditionTrue), "Error getting Istio CRD")
			}
			Success("CRDs are Established")
		})

		Specify("istio crd is present", func(ctx SpecContext) {
			// When the operator runs in OCP cluster, the CRD is created but not available at the moment
			Eventually(cl.Get).WithArguments(ctx, key("istios.operator.istio.io"), crd).
				Should(Succeed(), "Error getting Istio CRD")
			Success("Istio CRD is present")
		})

		It("starts successfully", func(ctx SpecContext) {
			Eventually(getObject).WithArguments(ctx, cl, key(deploymentName, namespace), deployment).
				Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
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

					It("deploys the CNI DaemonSet", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							g.Expect(cl.Get(ctx, key("istio-cni-node", istioCniNamespace), daemonset)).To(Succeed(), "Error getting IstioCNI DaemonSet")
							g.Expect(daemonset.Status.NumberAvailable).
								To(Equal(daemonset.Status.CurrentNumberScheduled), "CNI DaemonSet Pods not Available; expected numberAvailable to be equal to currentNumberScheduled")
						}).Should(Succeed(), "CNI DaemonSet Pods are not Available")
						Success("CNI DaemonSet is deployed in the namespace and Running")
					})

					It("updates the status to Reconciled", func(ctx SpecContext) {
						Eventually(getObject).WithArguments(ctx, cl, key(istioCniName), cni).
							Should(HaveCondition(v1alpha1.IstioCNIConditionTypeReconciled, metav1.ConditionTrue), "IstioCNI is not Reconciled; unexpected Condition")
						Success("IstioCNI is Reconciled")
					})

					It("updates the status to Ready", func(ctx SpecContext) {
						Eventually(getObject).WithArguments(ctx, cl, key(istioCniName), cni).
							Should(HaveCondition(v1alpha1.IstioCNIConditionTypeReady, metav1.ConditionTrue), "IstioCNI is not Ready; unexpected Condition")
						Success("IstioCNI is Ready")
					})

					It("doesn't continuously reconcile the IstioCNI CR", func() {
						Eventually(kubectl.Logs).WithArguments(namespace, "deploy/"+deploymentName, ptr.Of(30*time.Second)).
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

					It("updates the Istio CR status to Reconciled", func(ctx SpecContext) {
						Eventually(getObject).WithArguments(ctx, cl, key(istioName), istio).
							Should(HaveCondition(v1alpha1.IstioConditionTypeReconciled, metav1.ConditionTrue), "Istio is not Reconciled; unexpected Condition")
						Success("Istio CR is Reconciled")
					})

					It("updates the Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(getObject).WithArguments(ctx, cl, key(istioName), istio).
							Should(HaveCondition(v1alpha1.IstioConditionTypeReady, metav1.ConditionTrue), "Istio is not Ready; unexpected Condition")
						Success("Istio CR is Ready")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(getObject).WithArguments(ctx, cl, key("istiod", controlPlaneNamespace), deployment).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available; unexpected Condition")
						Expect(getVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running")
					})

					It("doesn't continuously reconcile the Istio CR", func() {
						Eventually(kubectl.Logs).WithArguments(namespace, "deploy/"+deploymentName, ptr.Of(30*time.Second)).
							ShouldNot(ContainSubstring("Reconciliation done"), "Istio Operator is continuously reconciling")
						Success("Istio Operator stopped reconciling")
					})
				})

				When("the Istio CR is deleted", func() {
					BeforeEach(func() {
						Expect(kubectl.Delete(controlPlaneNamespace, "istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Success("Istio CR deleted")
					})

					It("removes everything from the namespace", func(ctx SpecContext) {
						Eventually(cl.Get).WithArguments(ctx, key("istiod", controlPlaneNamespace), deployment).
							Should(ReturnNotFoundError(), "Istiod should not exist anymore")
						checkNamespaceEmpty(ctx, controlPlaneNamespace)
						Success("Namespace is empty")
					})
				})

				When("the IstioCNI CR is deleted", func() {
					BeforeEach(func() {
						Expect(kubectl.Delete(istioCniNamespace, "istiocni", istioCniName)).To(Succeed(), "IstioCNI CR failed to be deleted")
						Success("IstioCNI deleted")
					})

					It("removes everything from the CNI namespace", func(ctx SpecContext) {
						Eventually(cl.Get).WithArguments(ctx, key("istio-cni-node", istioCniNamespace), daemonset).
							Should(ReturnNotFoundError(), "IstioCNI DaemonSet should not exist anymore")
						checkNamespaceEmpty(ctx, istioCniNamespace)
						Success("CNI namespace is empty")
					})
				})
			})
		}

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				LogDebugInfo()
			}

			By("Cleaning up the Istio namespace")
			Expect(cl.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNamespace}})).To(Succeed(), "Istio Namespace failed to be deleted")

			By("Cleaning up the IstioCNI namespace")
			Expect(cl.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: istioCniNamespace}})).To(Succeed(), "IstioCNI Namespace failed to be deleted")

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
		Expect(kubectl.DeleteCRDs(sailCRDs)).To(Succeed(), "CRDs failed to be deleted")
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

func getCRDsName() []string {
	var crdNames []string
	for _, crd := range crdList.Items {
		crdNames = append(crdNames, crd.ObjectMeta.Name)
	}
	return crdNames
}

func LogDebugInfo() {
	// General debugging information to help diagnose the failure
	// TODO: Add more debugging information for others resources
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

// key returns the client.ObjectKey for the given name and namespace. If no namespace is provided, it returns a key cluster scoped
func key(name string, namespace ...string) client.ObjectKey {
	if len(namespace) > 1 {
		panic("you can only provide one namespace")
	} else if len(namespace) == 1 {
		return client.ObjectKey{Name: name, Namespace: namespace[0]}
	}
	return client.ObjectKey{Name: name}
}

// getObject returns the object with the given key
func getObject(ctx context.Context, cl client.Client, key client.ObjectKey, obj client.Object) (client.Object, error) {
	err := cl.Get(ctx, key, obj)
	return obj, err
}

// checkNamespaceEmpty checks if the given namespace is empty
func checkNamespaceEmpty(ctx SpecContext, ns string) {
	// TODO: Check to add more validations
	Eventually(func() ([]corev1.Pod, error) {
		podList := &corev1.PodList{}
		err := cl.List(ctx, podList, client.InNamespace(ns))
		if err != nil {
			return nil, err
		}
		return podList.Items, nil
	}).Should(HaveLen(0), "No pods should be present in the namespace")

	Eventually(func() ([]appsv1.Deployment, error) {
		deploymentList := &appsv1.DeploymentList{}
		err := cl.List(ctx, deploymentList, client.InNamespace(ns))
		if err != nil {
			return nil, err
		}
		return deploymentList.Items, nil
	}).Should(HaveLen(0), "No Deployments should be present in the namespace")

	Eventually(func() ([]appsv1.DaemonSet, error) {
		daemonsetList := &appsv1.DaemonSetList{}
		err := cl.List(ctx, daemonsetList, client.InNamespace(ns))
		if err != nil {
			return nil, err
		}
		return daemonsetList.Items, nil
	}).Should(HaveLen(0), "No DaemonSets should be present in the namespace")

	Eventually(func() ([]corev1.Service, error) {
		serviceList := &corev1.ServiceList{}
		err := cl.List(ctx, serviceList, client.InNamespace(ns))
		if err != nil {
			return nil, err
		}
		return serviceList.Items, nil
	}).Should(HaveLen(0), "No Services should be present in the namespace")
}
