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
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"testing"

	"github.com/istio-ecosystem/sail-operator/tests/e2e/operator/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
)

var (
	k8sclient          *kubernetes.Clientset
	k8sclientextension *apiextensionsclient.Clientset
	err                error
)

func TestInstall(t *testing.T) {
	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Install Operator Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Initializing k8s client")
	k8sclient, k8sclientextension, err = kubectl.InitK8sClients()
	Expect(err).NotTo(HaveOccurred())

	if ocp {
		GinkgoWriter.Println("Running on OCP cluster")
		GinkgoWriter.Printf("Absolute Path: %s\n", wd)
	} else {
		GinkgoWriter.Println("Running on Kubernetes")
		GinkgoWriter.Printf("Absolute Path: %s\n", wd)
	}
}
