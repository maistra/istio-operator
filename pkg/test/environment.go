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

package test

import (
	"path"

	"go.uber.org/zap/zapcore"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	"maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
)

func SetupEnv() (*envtest.Environment, client.Client, *rest.Config) {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.Level(zapcore.Level(-2))))

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{path.Join(common.RepositoryRoot, "chart", "crds")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil || cfg == nil {
		panic(err)
	}

	SetupScheme()

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(err)
	}

	return testEnv, k8sClient, cfg
}

func SetupScheme() {
	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(networkingv1alpha3.AddToScheme(scheme.Scheme))
}
