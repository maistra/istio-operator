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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"maistra.io/istio-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("IstioRevision resource", Ordered, func() {
	const (
		revName        = "test-istiorevision"
		istioNamespace = "istiorevision-test"

		istioVersion = "v1.20.0" // TODO: get this from versions.yaml

		pilotImage = "maistra.io/test:latest"
	)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(30 * time.Second)

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioNamespace,
		},
	}

	revKey := client.ObjectKey{Name: revName}
	istiodKey := client.ObjectKey{Name: "istiod-" + revName, Namespace: istioNamespace}
	cniKey := client.ObjectKey{Name: "istio-cni-node", Namespace: operatorNamespace}
	webhookKey := client.ObjectKey{Name: "istio-sidecar-injector-" + revName + "-" + istioNamespace}

	BeforeAll(func() {
		Step("Creating the Namespace to perform the tests")
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	})

	AfterAll(func() {
		// TODO(user): Attention if you improve this code by adding other context test you MUST
		// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		Step("Deleting the Namespace to perform the tests")
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())

		Expect(k8sClient.DeleteAllOf(ctx, &v1alpha1.IstioRevision{})).To(Succeed())
		Eventually(func(g Gomega) {
			list := &v1alpha1.IstioRevisionList{}
			g.Expect(k8sClient.List(ctx, list)).To(Succeed())
			g.Expect(list.Items).To(BeEmpty())
		}).Should(Succeed())
	})

	rev := &v1alpha1.IstioRevision{}

	It("successfully reconciles the resource", func() {
		Step("Creating the custom resource")
		rev = &v1alpha1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: revName,
			},
			Spec: v1alpha1.IstioRevisionSpec{
				Version:   istioVersion,
				Namespace: istioNamespace,
				Values: []byte(`{
						"global":{"istioNamespace":"` + istioNamespace + `"},
						"revision":"` + revName + `",
						"pilot":{"image":"` + pilotImage + `"},
						"istio_cni":{"enabled":true}
					}`),
			},
		}

		Expect(k8sClient.Create(ctx, rev)).To(Succeed())

		Step("Checking if the resource was successfully created")
		Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())

		istiod := &appsv1.Deployment{}
		Step("Checking if Deployment was successfully created in the reconciliation")
		Eventually(k8sClient.Get).WithArguments(ctx, istiodKey, istiod).Should(Succeed())
		Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
		Expect(istiod.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(rev)))

		Step("Checking if the status is updated")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
			g.Expect(rev.Status.ObservedGeneration).To(Equal(rev.ObjectMeta.Generation))
		}).Should(Succeed())
	})

	When("istiod and istio-cni-node readiness changes", func() {
		It("updates the status of the IstioRevision resource", func() {
			By("setting the Ready condition status to true when both are ready", func() {
				istiod := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())
				istiod.Status.Replicas = 1
				istiod.Status.ReadyReplicas = 1
				Expect(k8sClient.Status().Update(ctx, istiod)).To(Succeed())

				cni := &appsv1.DaemonSet{}
				Expect(k8sClient.Get(ctx, cniKey, cni)).To(Succeed())
				cni.Status.CurrentNumberScheduled = 3
				cni.Status.NumberReady = 3
				Expect(k8sClient.Status().Update(ctx, cni)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
					readyCondition := rev.Status.GetCondition(v1alpha1.IstioRevisionConditionTypeReady)
					g.Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				}).Should(Succeed())
			})

			By("setting the Ready condition status to false when istiod isn't ready", func() {
				istiod := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())

				istiod.Status.ReadyReplicas = 0
				Expect(k8sClient.Status().Update(ctx, istiod)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
					readyCondition := rev.Status.GetCondition(v1alpha1.IstioRevisionConditionTypeReady)
					g.Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				}).Should(Succeed())
			})
		})
	})

	When("an owned namespaced resource is deleted", func() {
		It("recreates the owned resource", func() {
			istiod := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      istiodKey.Name,
					Namespace: istiodKey.Namespace,
				},
			}
			Expect(k8sClient.Delete(ctx, istiod)).To(Succeed())

			Eventually(k8sClient.Get).WithArguments(ctx, istiodKey, istiod).Should(Succeed())
			Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
			Expect(istiod.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(rev)))
		})
	})

	When("an owned cluster-scoped resource is deleted", func() {
		It("recreates the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: webhookKey.Name,
				},
			}
			Expect(k8sClient.Delete(ctx, webhook)).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, webhookKey, webhook).Should(Succeed())
		})
	})

	When("an owned namespaced resource is modified", func() {
		istiod := &appsv1.Deployment{}
		var originalImage string

		BeforeAll(func() {
			Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())
			originalImage = istiod.Spec.Template.Spec.Containers[0].Image

			istiod.Spec.Template.Spec.Containers[0].Image = "user-supplied-image"
			Expect(k8sClient.Update(ctx, istiod)).To(Succeed())
		})

		It("reverts the owned resource", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())
				g.Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(originalImage))
			}).Should(Succeed())
		})
	})

	When("an owned cluster-scoped resource is modified", func() {
		webhook := &admissionv1.MutatingWebhookConfiguration{}
		var origWebhooks []admissionv1.MutatingWebhook

		BeforeAll(func() {
			Expect(k8sClient.Get(ctx, webhookKey, webhook)).To(Succeed())
			origWebhooks = webhook.Webhooks

			webhook.Webhooks = []admissionv1.MutatingWebhook{}
			Expect(k8sClient.Update(ctx, webhook)).To(Succeed())
		})

		It("reverts the owned resource", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, webhookKey, webhook)).To(Succeed())
				g.Expect(webhook.Webhooks).To(Equal(origWebhooks))
			}).Should(Succeed())
		})
	})

	It("supports concurrent deployment of two control planes", func() {
		rev2Name := revName + "2"
		rev2Key := client.ObjectKey{Name: rev2Name}
		istiod2Key := client.ObjectKey{Name: "istiod-" + rev2Name, Namespace: istioNamespace}

		Step("Creating the second IstioRevision instance")
		rev2 := &v1alpha1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: rev2Key.Name,
			},
			Spec: v1alpha1.IstioRevisionSpec{
				Version:   istioVersion,
				Namespace: istioNamespace,
				Values: []byte(`{
						"global":{"istioNamespace":"` + istioNamespace + `"},
						"revision": "` + rev2Key.Name + `",
						"pilot":{"image":"` + pilotImage + `"}
					}`),
			},
		}
		Expect(k8sClient.Create(ctx, rev2)).To(Succeed())

		Step("Checking if the resource was successfully created")
		Eventually(k8sClient.Get).WithArguments(ctx, rev2Key, rev2).Should(Succeed())

		Step("Checking if the status is updated")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, rev2Key, rev2)).To(Succeed())
			g.Expect(rev2.Status.ObservedGeneration).To(Equal(rev2.ObjectMeta.Generation))
		}).Should(Succeed())

		Step("Checking if Deployment was successfully created in the reconciliation")
		istiod := &appsv1.Deployment{}
		Eventually(k8sClient.Get).WithArguments(ctx, istiod2Key, istiod).Should(Succeed())
		Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
		Expect(istiod.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(rev2)))
	})
})

func expectedOwnerReference(rev *v1alpha1.IstioRevision) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         v1alpha1.GroupVersion.String(),
		Kind:               v1alpha1.IstioRevisionKind,
		Name:               rev.Name,
		UID:                rev.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}
}
