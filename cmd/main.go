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

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/controllers/istio"
	"github.com/istio-ecosystem/sail-operator/controllers/istiocni"
	"github.com/istio-ecosystem/sail-operator/controllers/istiorevision"
	"github.com/istio-ecosystem/sail-operator/pkg/common"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/version"
	multusv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(multusv1.AddToScheme(scheme))
	utilruntime.Must(networkingv1alpha3.AddToScheme(scheme))

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string
	var configFile string
	var resourceDirectory string
	var defaultProfilesStr string
	var logAPIRequests bool
	var printVersion bool
	var leaderElectionEnabled bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&configFile, "config-file", "/etc/sail-operator/config.properties", "Location of the config file, propagated by k8s downward APIs")
	flag.StringVar(&resourceDirectory, "resource-directory", "/var/lib/sail-operator/resources", "Where to find resources (e.g. charts)")
	flag.StringVar(&defaultProfilesStr, "default-profiles", "default", "Comma-separated profile names that are always applied to each Istio resource")
	flag.BoolVar(&logAPIRequests, "log-api-requests", false, "Whether to log each request sent to the Kubernetes API server")
	flag.BoolVar(&printVersion, "version", printVersion, "Prints version information and exits")
	flag.BoolVar(&leaderElectionEnabled, "leader-elect", true,
		"Enable leader election for this operator. Enabling this will ensure there is only one active controller manager.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if printVersion {
		fmt.Println(version.Info)
		os.Exit(0)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	operatorNamespace := os.Getenv("POD_NAMESPACE")
	if operatorNamespace == "" {
		contents, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		operatorNamespace = string(contents)
		if err != nil || operatorNamespace == "" {
			setupLog.Error(err, "can't determine namespace this operator is running in; if running outside of a pod, please set the POD_NAMESPACE environment variable")
			os.Exit(1)
		}
	}

	if defaultProfilesStr == "" {
		setupLog.Error(nil, "--default-profiles shouldn't be empty")
		os.Exit(1)
	}

	setupLog.Info(version.Info.String())
	setupLog.Info("reading config")
	err := common.ReadConfig(configFile)
	if err != nil {
		setupLog.Error(err, "unable to read config file at "+configFile)
		os.Exit(1)
	}
	setupLog.Info("config loaded", "config", common.Config)

	cfg := ctrl.GetConfigOrDie()
	if logAPIRequests {
		cfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
			return requestLogger{rt: rt}
		})
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          leaderElectionEnabled,
		LeaderElectionID:        "sail-operator-lock",
		LeaderElectionNamespace: operatorNamespace,
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	defaultProfiles := strings.Split(defaultProfilesStr, ",")

	err = istio.NewIstioReconciler(mgr.GetClient(), mgr.GetScheme(), resourceDirectory, defaultProfiles).
		SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Istio")
		os.Exit(1)
	}

	helm.ResourceDirectory = resourceDirectory
	err = istiorevision.NewIstioRevisionReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig()).
		SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IstioRevision")
		os.Exit(1)
	}

	err = istiocni.NewIstioCNIReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), resourceDirectory, defaultProfiles).
		SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IstioCNI")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

type requestLogger struct {
	rt http.RoundTripper
}

func (rl requestLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	log := logf.FromContext(req.Context())
	log.Info("Performing API request", "method", req.Method, "URL", req.URL)
	return rl.rt.RoundTrip(req)
}

var _ http.RoundTripper = requestLogger{}
