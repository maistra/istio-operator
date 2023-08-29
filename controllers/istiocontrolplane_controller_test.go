package controllers

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	v1 "maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var testConfig = common.OperatorConfig{
	Images3_0: common.ImageConfig3_0{
		Istiod: "maistra.io/test:latest",
	},
}

var _ = Describe("IstioControlPlaneController", Ordered, func() {
	const ihiName = "test-istio"
	const namespaceName = "test"

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}

	ihiNamespacedName := types.NamespacedName{Name: ihiName, Namespace: namespaceName}
	deploymentNamespacedName := types.NamespacedName{Name: "istiod", Namespace: namespaceName}
	webhookNamespacedName := types.NamespacedName{Name: "istio-sidecar-injector-" + namespaceName}

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

	ihi := &v1.IstioControlPlane{}

	It("successfully reconciles the IHI", func() {
		By("Creating the custom resource")
		err := k8sClient.Get(ctx, ihiNamespacedName, ihi)
		if err != nil && errors.IsNotFound(err) {
			ihi = &v1.IstioControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ihiName,
					Namespace: namespaceName,
				},
				Spec: v1.IstioControlPlaneSpec{
					Version: "v3.0",
				},
			}

			err = k8sClient.Create(ctx, ihi)
			Expect(err).NotTo(HaveOccurred())
		}

		By("Checking if the custom resource was successfully created")
		Eventually(func() error {
			found := &v1.IstioControlPlane{}
			return k8sClient.Get(ctx, ihiNamespacedName, found)
		}, time.Minute, time.Second).Should(Succeed())

		istiodDeployment := &appsv1.Deployment{}
		By("Checking if Deployment was successfully created in the reconciliation")
		Eventually(func() error {
			return k8sClient.Get(ctx, deploymentNamespacedName, istiodDeployment)
		}, time.Minute, time.Second).Should(Succeed())
		Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(testConfig.Images3_0.Istiod))
		Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(ihi)))

		By("Checking if the status was written properly")
		Eventually(func() string {
			err := k8sClient.Get(ctx, ihiNamespacedName, ihi)
			Expect(err).NotTo(HaveOccurred())

			vals := ihi.Status.GetAppliedValues()
			imageName, _, err := unstructured.NestedString(vals, "pilot", "image")
			Expect(err).NotTo(HaveOccurred())
			return imageName
		}, time.Minute, time.Second).Should(Equal(testConfig.Images3_0.Istiod))
	})

	When("an owned namespaced resource is deleted", func() {
		It("recreates the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istiod",
					Namespace: namespaceName,
				},
			}
			err := k8sClient.Delete(ctx, istiodDeployment, client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				return k8sClient.Get(ctx, deploymentNamespacedName, istiodDeployment)
			}, time.Minute, time.Second).Should(Succeed())

			Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(testConfig.Images3_0.Istiod))
			Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(ihi)))
		})
	})

	When("an owned cluster-scoped resource is deleted", func() {
		It("recreates the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: webhookNamespacedName.Name,
				},
			}
			err := k8sClient.Delete(ctx, webhook, client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				err := k8sClient.Get(ctx, webhookNamespacedName, webhook)
				return err
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	When("an owned namespaced resource is modified", func() {
		It("reverts the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, deploymentNamespacedName, istiodDeployment)
			Expect(err).NotTo(HaveOccurred())

			originalImage := istiodDeployment.Spec.Template.Spec.Containers[0].Image
			istiodDeployment.Spec.Template.Spec.Containers[0].Image = "user-supplied-image"
			err = k8sClient.Update(ctx, istiodDeployment)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				err := k8sClient.Get(ctx, deploymentNamespacedName, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())
				return istiodDeployment.Spec.Template.Spec.Containers[0].Image
			}, time.Minute, time.Second).Should(Equal(originalImage))
		})
	})

	When("an owned cluster-scoped resource is modified", func() {
		It("reverts the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{}
			err := k8sClient.Get(ctx, webhookNamespacedName, webhook)
			Expect(err).NotTo(HaveOccurred())

			origWebhooks := webhook.Webhooks
			webhook.Webhooks = []admissionv1.MutatingWebhook{}
			err = k8sClient.Update(ctx, webhook)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() []admissionv1.MutatingWebhook {
				err := k8sClient.Get(ctx, webhookNamespacedName, webhook)
				Expect(err).NotTo(HaveOccurred())
				return webhook.Webhooks
			}, time.Minute, time.Second).Should(Equal(origWebhooks))
		})
	})
})

func expectedOwnerReference(ihi *v1.IstioControlPlane) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioControlPlaneKind,
		Name:               ihi.Name,
		UID:                ihi.UID,
		Controller:         pointer.Bool(true),
		BlockOwnerDeletion: pointer.Bool(true),
	}
}
