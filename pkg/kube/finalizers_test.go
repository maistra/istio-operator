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

package kube

import (
	"context"
	"os"
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/common"
	"github.com/istio-ecosystem/sail-operator/pkg/test"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const version = "latest"

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
	RegisterTestingT(t)
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
				Spec: v1.IstioSpec{
					Version: version,
				},
			},
			expectedResult: false,
		},
		{
			obj: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{common.FinalizerName},
				},
				Spec: v1.IstioSpec{
					Version: version,
				},
			},
			expectedResult: true,
		},
	}
	for _, tc := range testCases {
		Expect(HasFinalizer(tc.obj)).To(Equal(tc.expectedResult))
	}
}

func TestAddRemoveFinalizer(t *testing.T) {
	RegisterTestingT(t)

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
				Spec: v1.IstioSpec{
					Version: version,
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
				Spec: v1.IstioSpec{
					Version: version,
				},
			},
			resultFinalizers: []string{common.FinalizerName},
		},
	}
	for _, tc := range testCases {
		Eventually(k8sClient.Create(context.TODO(), tc.obj)).Should(Succeed())
		Expect(AddFinalizer(context.TODO(), tc.obj, k8sClient)).NotTo(HaveOccurred())
		obj := &v1.Istio{}
		Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: tc.obj.GetNamespace(), Name: tc.obj.GetName()}, obj)).To(Succeed())
		Expect(obj.ObjectMeta.Finalizers).To(Equal(tc.resultFinalizers))
		Expect(RemoveFinalizer(context.TODO(), tc.obj, k8sClient)).NotTo(HaveOccurred())
		Eventually(k8sClient.Delete(context.TODO(), tc.obj)).Should(Succeed())
	}
}
