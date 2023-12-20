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

package integration

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var testConfig = common.OperatorConfig{}

const (
	istioVersion = "latest"
	pilotImage   = "maistra.io/test:latest"
)

var _ = Describe("IstioRevisionController", Ordered, func() {
	const istioName = "test-istio"
	const istioNamespace = "test"

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioNamespace,
		},
	}

	istioObjectKey := client.ObjectKey{Name: istioName}
	deploymentObjectKey := client.ObjectKey{Name: "istiod-" + istioName, Namespace: istioNamespace}
	cniObjectKey := client.ObjectKey{Name: "istio-cni-node", Namespace: operatorNamespace}
	webhookObjectKey := client.ObjectKey{Name: "istio-sidecar-injector-" + istioName + "-" + istioNamespace}

	common.Config = testConfig

	BeforeAll(func() {
		By("Creating the Namespace to perform the tests")
		err := k8sClient.Create(ctx, namespace)
		Expect(err).To(Not(HaveOccurred()))
	})

	AfterAll(func() {
		// TODO(user): Attention if you improve this code by adding other context test you MUST
		// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		By("Deleting the Namespace to perform the tests")
		_ = k8sClient.Delete(ctx, namespace)
	})

	istio := &v1.IstioRevision{}

	It("successfully reconciles the resource", func() {
		By("Creating the custom resource")
		err := k8sClient.Get(ctx, istioObjectKey, istio)
		if err != nil && errors.IsNotFound(err) {
			istio = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioVersion,
					Namespace: istioNamespace,
					Values: []byte(`{
						"global":{"istioNamespace":"` + istioNamespace + `"},
						"revision":"` + istioName + `",
						"pilot":{"image":"` + pilotImage + `"},
						"istio_cni":{"enabled":true}
					}`),
				},
			}

			ExpectSuccess(k8sClient.Create(ctx, istio))
		}

		By("Checking if the resource was successfully created")
		Eventually(func() error {
			found := &v1.IstioRevision{}
			return k8sClient.Get(ctx, istioObjectKey, found)
		}, time.Minute, time.Second).Should(Succeed())

		istiodDeployment := &appsv1.Deployment{}
		By("Checking if Deployment was successfully created in the reconciliation")
		Eventually(func() error {
			return k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
		}, time.Minute, time.Second).Should(Succeed())
		Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
		Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(istio)))

		By("Checking if the status is updated")
		Eventually(func() int64 {
			ExpectSuccess(k8sClient.Get(ctx, istioObjectKey, istio))
			return istio.Status.ObservedGeneration
		}, time.Minute, time.Second).Should(Equal(istio.ObjectMeta.Generation))
	})

	When("istiod and istio-cni-node readiness changes", func() {
		It("marks updates the status of the istio resource", func() {
			By("setting the Ready condition status to true when both are ready", func() {
				istiodDeployment := &appsv1.Deployment{}
				ExpectSuccess(k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment))
				istiodDeployment.Status.Replicas = 1
				istiodDeployment.Status.ReadyReplicas = 1
				ExpectSuccess(k8sClient.Status().Update(ctx, istiodDeployment))

				cniDaemonSet := &appsv1.DaemonSet{}
				ExpectSuccess(k8sClient.Get(ctx, cniObjectKey, cniDaemonSet))
				cniDaemonSet.Status.CurrentNumberScheduled = 3
				cniDaemonSet.Status.NumberReady = 3
				ExpectSuccess(k8sClient.Status().Update(ctx, cniDaemonSet))

				Eventually(func() metav1.ConditionStatus {
					ExpectSuccess(k8sClient.Get(ctx, istioObjectKey, istio))
					return istio.Status.GetCondition(v1.IstioRevisionConditionTypeReady).Status
				}, time.Minute, time.Second).Should(Equal(metav1.ConditionTrue))
			})

			By("setting the Ready condition status to false when istiod isn't ready", func() {
				istiodDeployment := &appsv1.Deployment{}
				ExpectSuccess(k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment))

				istiodDeployment.Status.ReadyReplicas = 0
				ExpectSuccess(k8sClient.Status().Update(ctx, istiodDeployment))

				Eventually(func() metav1.ConditionStatus {
					ExpectSuccess(k8sClient.Get(ctx, istioObjectKey, istio))
					return istio.Status.GetCondition(v1.IstioRevisionConditionTypeReady).Status
				}, time.Minute, time.Second).Should(Equal(metav1.ConditionFalse))
			})
		})
	})

	When("an owned namespaced resource is deleted", func() {
		It("recreates the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentObjectKey.Name,
					Namespace: deploymentObjectKey.Namespace,
				},
			}
			ExpectSuccess(k8sClient.Delete(ctx, istiodDeployment, client.PropagationPolicy(metav1.DeletePropagationForeground)))

			Eventually(func() error {
				return k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
			}, time.Minute, time.Second).Should(Succeed())

			Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
			Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(istio)))
		})
	})

	When("an owned cluster-scoped resource is deleted", func() {
		It("recreates the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: webhookObjectKey.Name,
				},
			}
			ExpectSuccess(k8sClient.Delete(ctx, webhook, client.PropagationPolicy(metav1.DeletePropagationForeground)))

			Eventually(func() error {
				err := k8sClient.Get(ctx, webhookObjectKey, webhook)
				return err
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	When("an owned namespaced resource is modified", func() {
		It("reverts the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{}
			ExpectSuccess(k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment))

			originalImage := istiodDeployment.Spec.Template.Spec.Containers[0].Image
			istiodDeployment.Spec.Template.Spec.Containers[0].Image = "user-supplied-image"
			ExpectSuccess(k8sClient.Update(ctx, istiodDeployment))

			Eventually(func() string {
				ExpectSuccess(k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment))
				return istiodDeployment.Spec.Template.Spec.Containers[0].Image
			}, time.Minute, time.Second).Should(Equal(originalImage))
		})
	})

	When("an owned cluster-scoped resource is modified", func() {
		It("reverts the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{}
			ExpectSuccess(k8sClient.Get(ctx, webhookObjectKey, webhook))

			origWebhooks := webhook.Webhooks
			webhook.Webhooks = []admissionv1.MutatingWebhook{}
			ExpectSuccess(k8sClient.Update(ctx, webhook))

			Eventually(func() []admissionv1.MutatingWebhook {
				ExpectSuccess(k8sClient.Get(ctx, webhookObjectKey, webhook))
				return webhook.Webhooks
			}, time.Minute, time.Second).Should(Equal(origWebhooks))
		})
	})

	It("supports concurrent deployment of two control planes", func() {
		rev2Name := istioName + "2"
		rev2ObjectKey := client.ObjectKey{Name: rev2Name}
		deployment2ObjectKey := client.ObjectKey{Name: "istiod-" + rev2Name, Namespace: istioNamespace}

		rev2 := &v1.IstioRevision{}

		By("Creating the second IstioRevision instance")
		err := k8sClient.Get(ctx, rev2ObjectKey, rev2)
		if err != nil && errors.IsNotFound(err) {
			rev2 = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: rev2ObjectKey.Name,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioVersion,
					Namespace: istioNamespace,
					Values: []byte(`{
						"global":{"istioNamespace":"` + istioNamespace + `"},
						"revision": "` + rev2ObjectKey.Name + `",
						"pilot":{"image":"` + pilotImage + `"}
					}`),
				},
			}

			ExpectSuccess(k8sClient.Create(ctx, rev2))
		}

		By("Checking if the resource was successfully created")
		Eventually(func() error {
			return k8sClient.Get(ctx, rev2ObjectKey, &v1.IstioRevision{})
		}, time.Minute, time.Second).Should(Succeed())

		By("Checking if the status is updated")
		Eventually(func() int64 {
			ExpectSuccess(k8sClient.Get(ctx, rev2ObjectKey, rev2))
			return rev2.Status.ObservedGeneration
		}, time.Minute, time.Second).Should(Equal(rev2.ObjectMeta.Generation))

		istiodDeployment := &appsv1.Deployment{}
		By("Checking if Deployment was successfully created in the reconciliation")
		Eventually(func() error {
			return k8sClient.Get(ctx, deployment2ObjectKey, istiodDeployment)
		}, time.Minute, time.Second).Should(Succeed())
		Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
		Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(rev2)))
	})
})

func expectedOwnerReference(istio *v1.IstioRevision) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioRevisionKind,
		Name:               istio.Name,
		UID:                istio.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}
}

func ExpectSuccess(err error) {
	GinkgoHelper()
	Expect(err).NotTo(HaveOccurred())
}
