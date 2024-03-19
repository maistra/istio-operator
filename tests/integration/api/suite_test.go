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
	"path"
	"testing"

	"github.com/istio-ecosystem/sail-operator/controllers/istio"
	"github.com/istio-ecosystem/sail-operator/controllers/istiocni"
	"github.com/istio-ecosystem/sail-operator/controllers/istiorevision"
	"github.com/istio-ecosystem/sail-operator/pkg/common"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	cfg       *rest.Config
	cancel    context.CancelFunc
)

const operatorNamespace = "sail-operator"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	testEnv, k8sClient, cfg = test.SetupEnv()
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		NewClient: func(config *rest.Config, options client.Options) (client.Client, error) {
			return k8sClient, nil
		},
	})
	if err != nil {
		panic(err)
	}

	chartManager := helm.NewChartManager(mgr.GetConfig(), "")
	resourceDir := path.Join(common.RepositoryRoot, "resources")
	defaultProfiles := []string{"default"}

	Expect(istio.NewIstioReconciler(mgr.GetClient(), mgr.GetScheme(), resourceDir, defaultProfiles).
		SetupWithManager(mgr)).To(Succeed())

	Expect(istiorevision.NewIstioRevisionReconciler(mgr.GetClient(), mgr.GetScheme(), resourceDir, chartManager).
		SetupWithManager(mgr)).To(Succeed())

	Expect(istiocni.NewIstioCNIReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), resourceDir, chartManager, defaultProfiles).
		SetupWithManager(mgr)).To(Succeed())

	// create new cancellable context
	var ctx context.Context
	ctx, cancel = context.WithCancel(context.Background())

	go func() {
		if err := mgr.Start(ctx); err != nil {
			panic(err)
		}
	}()

	operatorNs := &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: operatorNamespace}}
	Expect(k8sClient.Create(context.TODO(), operatorNs)).To(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if cancel != nil {
		cancel()
	}
	Expect(testEnv.Stop()).To(Succeed())
})
