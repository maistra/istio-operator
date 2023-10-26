package kube

import (
	"context"
	"os"
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	v1 "maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/common"
	"maistra.io/istio-operator/pkg/test"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
)

func setup() {
	testEnv, k8sClient, cfg = test.SetupEnv()
	err := k8sClient.Create(context.TODO(), namespace)
	if err != nil {
		panic(err)
	}
}

func teardown() {
	err := testEnv.Stop()
	if err != nil {
		panic(err)
	}
	k8sClient.Delete(context.TODO(), namespace)
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func TestHasFinalizer(t *testing.T) {
	gomega.RegisterTestingT(t)
	testCases := []struct {
		obj            client.Object
		expectedResult bool
	}{
		{
			obj:            &v1.Istio{},
			expectedResult: false,
		},
		{
			obj: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"blah"},
				},
			},
			expectedResult: false,
		},
		{
			obj: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{common.FinalizerName},
				},
			},
			expectedResult: true,
		},
	}
	for _, tc := range testCases {
		gomega.Expect(HasFinalizer(tc.obj)).To(gomega.Equal(tc.expectedResult))
	}
}

func TestAddRemoveFinalizer(t *testing.T) {
	gomega.RegisterTestingT(t)

	testCases := []struct {
		obj              client.Object
		resultFinalizers []string
	}{
		{
			obj: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			},
			resultFinalizers: []string{common.FinalizerName},
		},
		{
			obj: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Namespace:  "test",
					Finalizers: []string{common.FinalizerName},
				},
			},
			resultFinalizers: []string{common.FinalizerName},
		},
	}
	for _, tc := range testCases {
		gomega.Eventually(k8sClient.Create(context.TODO(), tc.obj)).Should(gomega.Succeed())
		gomega.Expect(AddFinalizer(context.TODO(), tc.obj, k8sClient)).NotTo(gomega.HaveOccurred())
		obj := &v1.Istio{}
		gomega.Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: tc.obj.GetNamespace(), Name: tc.obj.GetName()}, obj)).To(gomega.Succeed())
		gomega.Expect(obj.ObjectMeta.Finalizers).To(gomega.Equal(tc.resultFinalizers))
		gomega.Expect(RemoveFinalizer(context.TODO(), tc.obj, k8sClient)).NotTo(gomega.HaveOccurred())
		gomega.Eventually(k8sClient.Delete(context.TODO(), tc.obj)).Should(gomega.Succeed())
	}
}
