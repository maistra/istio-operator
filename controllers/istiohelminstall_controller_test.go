package controllers

import (
	"context"
	"path"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	v1 "maistra.io/istio-operator/api/v1"
	"maistra.io/istio-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var testConfig = common.OperatorConfig{
	Images3_0: common.ImageConfig3_0{
		Istiod: "maistra.io/test:latest",
	},
}

var _ = Describe("IstioHelmInstallController", func() {
	Context("Controller Test", func() {
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

		common.Config = testConfig

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			// TODO(user): Attention if you improve this code by adding other context test you MUST
			// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})
		It("should successfully reconcile the IHI", func() {
			By("Creating the custom resource")
			ihi := &v1.IstioHelmInstall{}
			err := k8sClient.Get(ctx, ihiNamespacedName, ihi)
			if err != nil && errors.IsNotFound(err) {
				ihi := &v1.IstioHelmInstall{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ihiName,
						Namespace: namespaceName,
					},
					Spec: v1.IstioHelmInstallSpec{
						Version: "v3.0",
					},
				}

				err = k8sClient.Create(ctx, ihi)
				Expect(err).NotTo(HaveOccurred())
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &v1.IstioHelmInstall{}
				return k8sClient.Get(ctx, ihiNamespacedName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			reconciler := &IstioHelmInstallReconciler{
				ResourceDirectory: path.Join(common.RepositoryRoot, "resources"),
				Config:            cfg,
				Client:            k8sClient,
				Scheme:            k8sClient.Scheme(),
			}

			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: ihiNamespacedName,
			})
			Expect(err).To(Not(HaveOccurred()))

			istiodDeployment := &appsv1.Deployment{}
			By("Checking if Deployment was successfully created in the reconciliation")
			Eventually(func() error {
				return k8sClient.Get(ctx, deploymentNamespacedName, istiodDeployment)
			}, time.Minute, time.Second).Should(Succeed())
			Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(testConfig.Images3_0.Istiod))

			By("Checking if the status was written properly")
			Eventually(func() error {
				return k8sClient.Get(ctx, ihiNamespacedName, ihi)
			}, time.Minute, time.Second).Should(Succeed())

			vals := ihi.Status.GetAppliedValues()
			imageName, _, err := unstructured.NestedString(vals, "pilot", "image")
			Expect(err).NotTo(HaveOccurred())
			Expect(imageName).To(Equal(testConfig.Images3_0.Istiod))
		})
	})
})
