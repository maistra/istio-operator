package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/maistra/istio-operator/pkg/apis"
	"github.com/maistra/istio-operator/pkg/controller"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/version"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/ready"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

var log = logf.Log.WithName("cmd")
var discoveryCacheDir string

func main() {
	flag.StringVar(&discoveryCacheDir, "discoveryCacheDir", "/home/istio-operator/.kube/cache/discovery", "The location where cached discovery information used by the REST client is stored.")
	flag.StringVar(&common.ResourceDir, "resourceDir", "/usr/local/share/istio-operator", "The location of the resources - helm charts, templates, etc.")

	logConfig := "production"
	flag.StringVar(&logConfig, "logConfig", logConfig, "Whether to configure logging for production use (json, info level, w/ log sampling) or development (plain-text, debug level, w/o log sampling)")

	printVersion := false
	flag.BoolVar(&printVersion, "version", printVersion, "Prints version information and exits")

	flag.Parse()
	if printVersion {
		fmt.Printf("%s\n", version.Info)
		os.Exit(0)
	}

	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(logf.ZapLogger(logConfig != "production"))

	log.Info(fmt.Sprintf("Starting Istio Operator %s", version.Info))

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Info("watching all namespaces")
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

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{Namespace: namespace, MapperProvider: NewDeferredDiscoveryRESTMapper})
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
	//metrics.ExposeMetricsPort()

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "manager exited non-zero")
		os.Exit(1)
	}
}

// NewDeferredDiscoveryRESTMapper constructs a new DeferredDiscoveryRESTMapper
// based on discovery information fetched by a new client with the given config.
func NewDeferredDiscoveryRESTMapper(c *rest.Config) (meta.RESTMapper, error) {
	// Get a mapper
	dc, err := discovery.NewCachedDiscoveryClientForConfig(c, discoveryCacheDir, "", 10*time.Minute)
	if err != nil {
		panic(err)
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(dc), nil
}
