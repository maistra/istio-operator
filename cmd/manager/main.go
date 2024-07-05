package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/magiconair/properties"
	"github.com/mitchellh/mapstructure"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/discovery"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/maistra/istio-operator/pkg/apis"
	"github.com/maistra/istio-operator/pkg/controller"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/version"
)

// Change below variables to serve metrics on different host or port.
var (
	metricsHost                   = "0.0.0.0"
	metricsPort             int32 = 8383
	admissionControllerPort       = 11999
	healthProbeBindAddress        = ":11200"
)
var log = logf.Log.WithName("cmd")

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	// number of concurrent reconciler for each controller
	pflag.Int("controlPlaneReconcilers", 1, "The number of concurrent reconcilers for ServiceMeshControlPlane resources")
	pflag.Int("memberRollReconcilers", 1, "The number of concurrent reconcilers for ServiceMeshMemberRoll resources")
	pflag.Int("memberReconcilers", 10, "The number of concurrent reconcilers for ServiceMeshMember resources")

	// flags to configure API request throttling
	pflag.Int("apiBurst", 50, "The number of API requests the operator can make before throttling is activated")
	pflag.Float32("apiQPS", 25, "The max rate of API requests when throttling is active")

	// custom flags for istio operator
	pflag.String("resourceDir", "/usr/local/share/istio-operator", "The location of the resources - helm charts, templates, etc.")
	pflag.String("chartsDir", "", "The root location of the helm charts.")
	pflag.String("defaultTemplatesDir", "", "The root location of the default templates.")
	pflag.String("userTemplatesDir", "", "The root location of the user supplied templates.")

	var logAPIRequests bool
	pflag.BoolVar(&logAPIRequests, "logAPIRequests", false, "Log API requests performed by the operator.")

	pflag.Bool("leader-elect", true, "Enable leader election for this operator. Enabling this will ensure there is only one active controller manager.")
	pflag.Duration("leader-elect-lease-duration", 15*time.Second, "The duration that non-leader candidates will wait after observing a leadership renewal "+
		"until attempting to acquire leadership of a led but unrenewed leader slot. ")
	pflag.Duration("leader-elect-renew-deadline", 10*time.Second, "The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than the lease duration. ")
	pflag.Duration("leader-elect-retry-period", 2*time.Second, "The duration the clients should wait between attempting acquisition and renewal of a leadership.")

	// config file
	configFile := ""
	pflag.StringVar(&configFile, "config", "/etc/istio-operator/config.properties", "The root location of the user supplied templates.")

	printVersion := false
	pflag.BoolVar(&printVersion, "version", printVersion, "Prints version information and exits")

	pflag.Parse()
	if printVersion {
		fmt.Printf("%s\n", version.Info)
		os.Exit(0)
	}

	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.Logger())

	log.Info(fmt.Sprintf("Starting Istio Operator %s", version.Info))

	if err := initializeConfiguration(configFile); err != nil {
		log.Error(err, "error initializing operator configuration")
		os.Exit(1)
	}

	namespace, err := k8sutil.GetWatchNamespace()
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

	// Set default manager options
	options := manager.Options{
		Namespace:              namespace,
		Port:                   admissionControllerPort,
		MetricsBindAddress:     net.JoinHostPort(metricsHost, fmt.Sprint(metricsPort)),
		HealthProbeBindAddress: healthProbeBindAddress,
		LeaderElection:         common.Config.LeaderElection.Enabled,
		LeaderElectionID:       "istio-operator-lock",
		LeaseDuration:          common.Config.LeaderElection.LeaseDuration,
		RenewDeadline:          common.Config.LeaderElection.RenewDeadline,
		RetryPeriod:            common.Config.LeaderElection.RetryPeriod,
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

	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	enhancedMgr := common.NewEnhancedManager(mgr, dc)

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

	err = mgr.Add(manager.RunnableFunc(func(stop <-chan struct{}) error {
		createMetricsResources(context.Background(), cfg, mgr.GetClient(), dc)
		return nil
	}))
	if err != nil {
		log.Error(err, "error adding metrics runnable")
		os.Exit(1)
	}

	err = mgr.AddReadyzCheck("readiness", func(req *http.Request) error {
		// no need to check anything; the readyz probe succeeds only when the
		// webhooks are running (which only happens when the serving secret is present)
		return nil
	})
	if err != nil {
		log.Error(err, "error adding readyz check")
		os.Exit(1)
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

// createMetricsResources creates the Service and ServiceMonitor resource to allow the operator export the metrics by using
// the Prometheus operator
func createMetricsResources(ctx context.Context, cfg *rest.Config, cl client.Client, dc discovery.DiscoveryInterface) {
	// Get the namespace the operator is currently deployed in.
	operatorNs, err := k8sutil.GetOperatorNamespace()
	if err != nil {
		if errors.Is(err, k8sutil.ErrRunLocal) {
			log.Info("Skipping CR metrics server creation; not running in a cluster.")
			return
		}
	}

	// Add to the below struct any other metrics ports you want to expose.
	servicePorts := []v1.ServicePort{
		{Port: metricsPort, Name: metrics.OperatorPortName, Protocol: v1.ProtocolTCP, TargetPort: intOrStringFromInt32(metricsPort)},
	}

	// Create Service object to expose the metrics port(s).
	service, err := metrics.CreateMetricsService0(ctx, cl, servicePorts)
	if err != nil {
		log.Info("Could not create metrics Service", "error", err.Error())
	}

	// CreateServiceMonitors will automatically create the prometheus-operator ServiceMonitor resources
	// necessary to configure Prometheus to scrape metrics from this operator.
	services := []*v1.Service{service}

	// The ServiceMonitor is created in the same namespace where the operator is deployed
	_, err = metrics.CreateServiceMonitors0(cfg, dc, operatorNs, services)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Info("Could not create ServiceMonitor object", "error", err.Error())
		// If this operator is deployed to a cluster without the prometheus-operator running, it will return
		// ErrServiceMonitorNotPresent, which can be used to safely skip ServiceMonitor creation.
		if err == metrics.ErrServiceMonitorNotPresent {
			log.Info("Install prometheus-operator in your cluster to create ServiceMonitor objects", "error", err.Error())
		}
	}
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

	v.RegisterAlias("leaderElection.enabled", "leader-elect")
	v.RegisterAlias("leaderElection.leaseDuration", "leader-elect-lease-duration")
	v.RegisterAlias("leaderElection.renewDeadline", "leader-elect-renew-deadline")
	v.RegisterAlias("leaderElection.retryPeriod", "leader-elect-retry-period")

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
