package controllers

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

var _ = Describe("IstioController", Ordered, func() {
	const istioName = "test-istio"
	const istioNamespace = "test"

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioNamespace,
		},
	}

	istioObjectKey := client.ObjectKey{Name: istioName, Namespace: istioNamespace}
	deploymentObjectKey := client.ObjectKey{Name: "istiod", Namespace: istioNamespace}
	cniObjectKey := client.ObjectKey{Name: "istio-cni-node", Namespace: operatorNamespace}
	webhookObjectKey := client.ObjectKey{Name: "istio-sidecar-injector-" + istioNamespace}

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

	istio := &v1.Istio{}

	It("successfully reconciles the resource", func() {
		By("Creating the custom resource")
		err := k8sClient.Get(ctx, istioObjectKey, istio)
		if err != nil && errors.IsNotFound(err) {
			istio = &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name:      istioName,
					Namespace: istioNamespace,
				},
				Spec: v1.IstioSpec{
					Version: "v3.0",
				},
			}

			err = k8sClient.Create(ctx, istio)
			Expect(err).NotTo(HaveOccurred())
		}

		By("Checking if the resource was successfully created")
		Eventually(func() error {
			found := &v1.Istio{}
			return k8sClient.Get(ctx, istioObjectKey, found)
		}, time.Minute, time.Second).Should(Succeed())

		istiodDeployment := &appsv1.Deployment{}
		By("Checking if Deployment was successfully created in the reconciliation")
		Eventually(func() error {
			return k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
		}, time.Minute, time.Second).Should(Succeed())
		Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(testConfig.Images3_0.Istiod))
		Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(istio)))

		By("Checking if the status is updated")
		Eventually(func() int64 {
			err := k8sClient.Get(ctx, istioObjectKey, istio)
			Expect(err).NotTo(HaveOccurred())
			return istio.Status.ObservedGeneration
		}, time.Minute, time.Second).Should(Equal(istio.ObjectMeta.Generation))

		By("Checking if the appliedValues are written properly")
		Eventually(func() string {
			err := k8sClient.Get(ctx, istioObjectKey, istio)
			Expect(err).NotTo(HaveOccurred())

			vals := istio.Status.GetAppliedValues()
			imageName, _, err := unstructured.NestedString(vals, "pilot", "image")
			Expect(err).NotTo(HaveOccurred())
			return imageName
		}, time.Minute, time.Second).Should(Equal(testConfig.Images3_0.Istiod))
	})

	When("istiod and istio-cni-node readiness changes", func() {
		It("marks updates the status of the istio resource", func() {
			By("setting the Ready condition status to true when both are ready", func() {
				istiodDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())
				istiodDeployment.Status.Replicas = 1
				istiodDeployment.Status.ReadyReplicas = 1
				err = k8sClient.Status().Update(ctx, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())

				cniDaemonSet := &appsv1.DaemonSet{}
				err = k8sClient.Get(ctx, cniObjectKey, cniDaemonSet)
				Expect(err).NotTo(HaveOccurred())
				cniDaemonSet.Status.CurrentNumberScheduled = 3
				cniDaemonSet.Status.NumberReady = 3
				err = k8sClient.Status().Update(ctx, cniDaemonSet)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() metav1.ConditionStatus {
					err := k8sClient.Get(ctx, istioObjectKey, istio)
					Expect(err).NotTo(HaveOccurred())
					return istio.Status.GetCondition(v1.ConditionTypeReady).Status
				}, time.Minute, time.Second).Should(Equal(metav1.ConditionTrue))
			})

			By("setting the Ready condition status to false when istiod isn't ready", func() {
				istiodDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())

				istiodDeployment.Status.ReadyReplicas = 0
				err = k8sClient.Status().Update(ctx, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() metav1.ConditionStatus {
					err := k8sClient.Get(ctx, istioObjectKey, istio)
					Expect(err).NotTo(HaveOccurred())
					return istio.Status.GetCondition(v1.ConditionTypeReady).Status
				}, time.Minute, time.Second).Should(Equal(metav1.ConditionFalse))
			})
		})
	})

	When("an owned namespaced resource is deleted", func() {
		It("recreates the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istiod",
					Namespace: istioNamespace,
				},
			}
			err := k8sClient.Delete(ctx, istiodDeployment, client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				return k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
			}, time.Minute, time.Second).Should(Succeed())

			Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(testConfig.Images3_0.Istiod))
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
			err := k8sClient.Delete(ctx, webhook, client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				err := k8sClient.Get(ctx, webhookObjectKey, webhook)
				return err
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	When("an owned namespaced resource is modified", func() {
		It("reverts the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
			Expect(err).NotTo(HaveOccurred())

			originalImage := istiodDeployment.Spec.Template.Spec.Containers[0].Image
			istiodDeployment.Spec.Template.Spec.Containers[0].Image = "user-supplied-image"
			err = k8sClient.Update(ctx, istiodDeployment)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())
				return istiodDeployment.Spec.Template.Spec.Containers[0].Image
			}, time.Minute, time.Second).Should(Equal(originalImage))
		})
	})

	When("an owned cluster-scoped resource is modified", func() {
		It("reverts the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{}
			err := k8sClient.Get(ctx, webhookObjectKey, webhook)
			Expect(err).NotTo(HaveOccurred())

			origWebhooks := webhook.Webhooks
			webhook.Webhooks = []admissionv1.MutatingWebhook{}
			err = k8sClient.Update(ctx, webhook)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() []admissionv1.MutatingWebhook {
				err := k8sClient.Get(ctx, webhookObjectKey, webhook)
				Expect(err).NotTo(HaveOccurred())
				return webhook.Webhooks
			}, time.Minute, time.Second).Should(Equal(origWebhooks))
		})
	})
})

func expectedOwnerReference(istio *v1.Istio) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioKind,
		Name:               istio.Name,
		UID:                istio.UID,
		Controller:         pointer.Bool(true),
		BlockOwnerDeletion: pointer.Bool(true),
	}
}

func TestDeriveState(t *testing.T) {
	testCases := []struct {
		name                string
		reconciledCondition v1.IstioCondition
		readyCondition      v1.IstioCondition
		expectedState       v1.IstioConditionReason
	}{
		{
			name:                "healthy",
			reconciledCondition: newCondition(v1.ConditionTypeReconciled, true, ""),
			readyCondition:      newCondition(v1.ConditionTypeReady, true, ""),
			expectedState:       v1.ConditionReasonHealthy,
		},
		{
			name:                "not reconciled",
			reconciledCondition: newCondition(v1.ConditionTypeReconciled, false, v1.ConditionReasonReconcileError),
			readyCondition:      newCondition(v1.ConditionTypeReady, true, ""),
			expectedState:       v1.ConditionReasonReconcileError,
		},
		{
			name:                "not ready",
			reconciledCondition: newCondition(v1.ConditionTypeReconciled, true, ""),
			readyCondition:      newCondition(v1.ConditionTypeReady, false, v1.ConditionReasonIstiodNotReady),
			expectedState:       v1.ConditionReasonIstiodNotReady,
		},
		{
			name:                "not reconciled nor ready",
			reconciledCondition: newCondition(v1.ConditionTypeReconciled, false, v1.ConditionReasonReconcileError),
			readyCondition:      newCondition(v1.ConditionTypeReady, false, v1.ConditionReasonIstiodNotReady),
			expectedState:       v1.ConditionReasonReconcileError, // reconcile reason takes precedence over ready reason
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := deriveState(tc.reconciledCondition, tc.readyCondition)
			if result != tc.expectedState {
				t.Errorf("Expected reason %s, but got %s", tc.expectedState, result)
			}
		})
	}
}

func newCondition(conditionType v1.IstioConditionType, status bool, reason v1.IstioConditionReason) v1.IstioCondition {
	st := metav1.ConditionFalse
	if status {
		st = metav1.ConditionTrue
	}
	return v1.IstioCondition{
		Type:   conditionType,
		Status: st,
		Reason: reason,
	}
}
