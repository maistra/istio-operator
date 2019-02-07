package main

import (
	"context"
	"runtime"

	"github.com/maistra/istio-operator/pkg/stub"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	sdkVersion "github.com/operator-framework/operator-sdk/version"

	"flag"
	"github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("operator-sdk Version: %v", sdkVersion.Version)
}

func main() {

	openShiftRelease := flag.String("release", "v3.10", "The OpenShift release")
	masterPublicURL := flag.String("masterPublicURL", "", "The public URL of the master when using Launcher")
	istioPrefix := flag.String("istioPrefix", "", "The default istio prefix for images")
	istioVersion := flag.String("istioVersion", "", "The default istio version for images")
	deploymentType := flag.String("deploymentType", "", "The default deployment type")
	alwaysPull := flag.Bool("alwaysPull", false, "Whether to always pull the installer container")
	enable3scale := flag.Bool("enable3scale", false, "Whether to enable the 3scale adapter")
	flag.Parse()

	printVersion()

	sdk.ExposeMetricsPort()

	resource := "istio.openshift.com/v1alpha1"
	kind := "Installation"
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Fatalf("Failed to get watch namespace: %v", err)
	}
	resyncPeriod := 0
	logrus.Infof("Watching resource %s, kind %s, namespace %s, resyncPeriod %d", resource, kind, namespace, resyncPeriod)
	sdk.Watch(resource, kind, namespace, resyncPeriod)
	sdk.Handle(stub.NewHandler(*openShiftRelease, *masterPublicURL, *istioPrefix, *istioVersion, *deploymentType, *alwaysPull, *enable3scale))
	sdk.Run(context.TODO())
}
