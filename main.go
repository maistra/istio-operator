/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	multusv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	maistraiov1 "maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/controllers"
	"maistra.io/istio-operator/pkg/common"
	"maistra.io/istio-operator/pkg/helm"
	"maistra.io/istio-operator/pkg/kube"
	"maistra.io/istio-operator/pkg/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
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

	utilruntime.Must(maistraiov1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var probeAddr string
	var configFile string
	var resourceDirectory string
	var defaultProfiles string
	var logAPIRequests bool
	var printVersion bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&configFile, "config-file", "/etc/istio-operator/config.properties", "Location of the config file, propagated by k8s downward APIs")
	flag.StringVar(&resourceDirectory, "resource-directory", "/var/lib/istio-operator/resources", "Where to find resources (e.g. charts)")
	flag.StringVar(&defaultProfiles, "default-profiles", "default", "One or more comma-separated profile names that are always applied to each Istio resource")
	flag.BoolVar(&logAPIRequests, "log-api-requests", false, "Whether to log each request sent to the Kubernetes API server")
	flag.BoolVar(&printVersion, "version", printVersion, "Prints version information and exits")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if printVersion {
		fmt.Println(version.Info)
		os.Exit(0)
	}

	if defaultProfiles == "" {
		setupLog.Error(nil, "--default-profiles shouldn't be empty")
		os.Exit(1)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
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
		LeaderElection:          true,
		LeaderElectionID:        "8d20bb54.istio.io",
		LeaderElectionNamespace: kube.GetOperatorNamespace(),
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

	helm.ResourceDirectory = resourceDirectory
	controller := controllers.NewIstioReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), resourceDirectory, strings.Split(defaultProfiles, ","))
	err = controller.SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Istio")
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
	log := common.LogFromContext(req.Context())
	log.Info("Performing API request", "method", req.Method, "URL", req.URL)
	return rl.rt.RoundTrip(req)
}

var _ http.RoundTripper = requestLogger{}
