package test

import (
	"path"

	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	v1 "maistra.io/istio-operator/api/v1"
	"maistra.io/istio-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func SetupEnv() (*envtest.Environment, client.Client, *rest.Config) {
	log.SetLogger(zap.New(zap.UseDevMode(true)))

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{path.Join(common.RepositoryRoot, "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	if err != nil || cfg == nil {
		panic(err)
	}

	err = v1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		panic(err)
	}

	return testEnv, k8sClient, cfg
}
