package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/maistra/istio-operator/pkg/apis"
	"github.com/maistra/istio-operator/pkg/bootstrap"
	"github.com/maistra/istio-operator/pkg/controller"
	"github.com/maistra/istio-operator/pkg/controller/controlplane"
	"github.com/maistra/istio-operator/pkg/controller/installation"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	"github.com/operator-framework/operator-sdk/pkg/ready"
	sdkVersion "github.com/operator-framework/operator-sdk/version"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("operator-sdk Version: %v", sdkVersion.Version))
}

func main() {
	// Installation
	handler := &installation.Handler{}
	installation.RegisterHandler(handler)

	flag.StringVar(&handler.OpenShiftRelease, "release", "v3.10", "The OpenShift release")
	flag.StringVar(&handler.MasterPublicURL, "masterPublicURL", "", "The public URL of the master when using Launcher")
	flag.StringVar(&handler.IstioPrefix, "istioPrefix", "", "The default istio prefix for images")
	flag.StringVar(&handler.IstioVersion, "istioVersion", "", "The default istio version for images")
	flag.StringVar(&handler.DeploymentType, "deploymentType", "", "The default deployment type")
	flag.BoolVar(&handler.AlwaysPull, "alwaysPull", false, "Whether to always pull the installer container")
	flag.BoolVar(&handler.Enable3Scale, "enable3scale", false, "Whether to enable the 3scale adapter")

	// ControlPlane
	flag.StringVar(&controlplane.ChartPath, "chartPath", "/etc/istio-operator/1.1.0/helm", "The location of the Helm charts.  The charts will be rendered using $chartPath/istio (similar layout to istio.io/istio/install/kubernetes/helm).")

	flag.Parse()

	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(logf.ZapLogger(false))

	printVersion()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Error(err, "failed to get watch namespace")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Become the leader before proceeding
	leader.Become(context.TODO(), "istio-operator-lock")

	r := ready.NewFileReady()
	err = r.Set()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
	defer r.Unset()

	syncPeriod := 5 * time.Minute
	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{Namespace: namespace, SyncPeriod: &syncPeriod})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Create Service object to expose the metrics port.
	metrics.ExposeMetricsPort()

	// Enusure CRDs are installed
	if err := bootstrap.InstallCRDs(mgr.GetClient()); err != nil {
		log.Error(err, "failed to install CRDs")
		os.Exit(1)
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "manager exited non-zero")
		os.Exit(1)
	}
}
