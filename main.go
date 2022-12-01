package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	"github.com/magiconair/properties"
	"github.com/mitchellh/mapstructure"
	"github.com/operator-framework/operator-lib/leader"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/intstr"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/maistra/istio-operator/apis"
	controller "github.com/maistra/istio-operator/controllers"
	"github.com/maistra/istio-operator/controllers/common"
	"github.com/maistra/istio-operator/version"
)

const (
	watchNamespaceEnvVar = "WATCH_NAMESPACE"
)

// Change below variables to serve metrics on different host or port.
var (
	metricsHost                   = "0.0.0.0"
	metricsPort             int32 = 8080
	operatorMetricsPort     int32 = 8686
	admissionControllerPort       = 11999
)
var log = logf.Log.WithName("cmd")

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)

	// number of concurrent reconciler for each controller
	flag.Int("controlPlaneReconcilers", 1, "The number of concurrent reconcilers for ServiceMeshControlPlane resources")
	flag.Int("memberRollReconcilers", 1, "The number of concurrent reconcilers for ServiceMeshMemberRoll resources")
	flag.Int("memberReconcilers", 10, "The number of concurrent reconcilers for ServiceMeshMember resources")

	// flags to configure API request throttling
	flag.Int("apiBurst", 50, "The number of API requests the operator can make before throttling is activated")
	flag.Float64("apiQPS", 25, "The max rate of API requests when throttling is active")

	// custom flags for istio operator
	flag.String("resourceDir", "/usr/local/share/istio-operator", "The location of the resources - helm charts, templates, etc.")
	flag.String("chartsDir", "", "The root location of the helm charts.")
	flag.String("defaultTemplatesDir", "", "The root location of the default templates.")
	flag.String("userTemplatesDir", "", "The root location of the user supplied templates.")

	var logAPIRequests bool
	flag.BoolVar(&logAPIRequests, "logAPIRequests", false, "Log API requests performed by the operator.")

	// config file
	configFile := ""
	flag.StringVar(&configFile, "config", "/etc/istio-operator/config.properties", "The root location of the user supplied templates.")

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
	logf.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	log.Info(fmt.Sprintf("Starting Istio Operator %s", version.Info))

	if err := initializeConfiguration(configFile); err != nil {
		log.Error(err, "error initializing operator configuration")
		os.Exit(1)
	}

	namespace, err := getWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	cfg.Burst = common.Config.Controller.APIBurst
	cfg.QPS = common.Config.Controller.APIQPS
	log.Info("Client-side rate limiting configured", "cfg.Burst", cfg.Burst, "cfg.QPS", cfg.QPS)

	if logAPIRequests {
		cfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
			return requestLogger{
				rt: rt,
			}
		})
	}

	ctx := context.Background()
	// Become the leader before proceeding
	err = leader.Become(ctx, "istio-operator-lock")

	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Set default manager options
	options := manager.Options{
		Namespace:          namespace,
		Port:               admissionControllerPort,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	}

	// Add support for MultiNamespace set in WATCH_NAMESPACE (e.g ns1,ns2)
	// Note that this is not intended to be used for excluding namespaces, this is better done via a Predicate
	// Also note that you may face performance issues when using this with a high number of namespaces.
	// More Info: https://godoc.org/github.com/kubernetes-sigs/controller-runtime/pkg/cache#MultiNamespacedCacheBuilder
	if strings.Contains(namespace, ",") {
		options.Namespace = ""
		options.NewCache = cache.MultiNamespacedCacheBuilder(strings.Split(namespace, ","))
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, options)
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and Start components
	enhancedMgr, err := manager.New(cfg, manager.Options{
		Namespace:              namespace,
		MetricsBindAddress:     fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		HealthProbeBindAddress: "0.0.0.0:8282",
	})
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
	if err := controller.AddToManager(enhancedMgr); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

// getWatchNamespace returns the namespace the operator should be watching for changes.
func getWatchNamespace() (string, error) {
	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}
	return ns, nil
}



func intOrStringFromInt32(val int32) intstr.IntOrString {
	return intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: val,
	}
}

func initializeConfiguration(configFile string) error {
	v, err := common.NewViper()
	if err != nil {
		return err
	}

	// map flags to config structure
	// controller settings
	v.RegisterAlias("controller.controlPlaneReconcilers", "controlPlaneReconcilers")
	v.RegisterAlias("controller.memberRollReconcilers", "memberRollReconcilers")
	v.RegisterAlias("controller.memberReconcilers", "memberReconcilers")
	v.RegisterAlias("controller.apiBurst", "apiBurst")
	v.RegisterAlias("controller.apiQPS", "apiQPS")
	v.RegisterAlias("controller.webhookManagementEnabled", "webhookManagementEnabled")

	// rendering settings
	v.RegisterAlias("rendering.resourceDir", "resourceDir")
	v.RegisterAlias("rendering.chartsDir", "chartsDir")
	v.RegisterAlias("rendering.defaultTemplatesDir", "defaultTemplatesDir")
	v.RegisterAlias("rendering.userTemplatesDir", "userTemplatesDir")

	if err := v.BindPFlags(pflag.CommandLine); err != nil {
		return err
	}
	v.AutomaticEnv()
	props, err := patchProperties(configFile)
	if err != nil {
		return err
	}
	if err := v.MergeConfigMap(props); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	if err := v.Unmarshal(common.Config, func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "json"
	}); err != nil {
		return err
	}
	log.Info("configuration successfully initialized", "config", common.Config)
	return nil
}

// downward api quotes values in the file (fmt.Sprintf("%q")), so we need to Unquote() them
func patchProperties(file string) (map[string]interface{}, error) {
	loader := properties.Loader{Encoding: properties.UTF8, IgnoreMissing: true, DisableExpansion: true}
	props, err := loader.LoadFile(file)
	if err != nil {
		return nil, err
	}
	retVal := make(map[string]interface{})
	for k, v := range props.Map() {
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
			// the properties reader will have already processed most special
			// characters, so all we need to do is remove the leading and trailing quotes
			v = v[1 : len(v)-1]
		}
		retVal[k] = v
	}
	return retVal, nil
}

type requestLogger struct {
	rt http.RoundTripper
}

func (rl requestLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	log := common.LogFromContext(req.Context())
	log.Info("Performing API request", "method", req.Method, "URL", req.URL)
	return rl.rt.RoundTrip(req)
}

var _ http.RoundTripper = requestLogger{}
