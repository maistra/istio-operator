package installation_test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Define common variables from the setup script to be used later if they are needed
var (
	command          = "kubectl"
	ocp              = os.Getenv("OCP")
	skipBuild        = os.Getenv("SKIP_BUILD")
	hub              = os.Getenv("HUB")
	image_base       = os.Getenv("IMAGE_BASE")
	image            = os.Getenv("IMAGE")
	tag              = os.Getenv("TAG")
	istio_manifest   = ""
	timeout          = "180s"
	namespace        = os.Getenv("NAMESPACE")
	deployment_name  = os.Getenv("DEPLOYMENT_NAME")
	control_plane_ns = os.Getenv("CONTROL_PLANE_NS")
	wd, _            = os.Getwd()
	deploy_operator  = os.Getenv("DEPLOY_OPERATOR")
	target           = "deploy"
)

func TestInstall(t *testing.T) {
	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Install Operator Suite")
}

func setup() {
	println("************ Running Setup ************")
	if ocp == "true" {
		command = "oc"
		fmt.Printf("Absolute Path: %s\n", wd)
		istio_manifest = fmt.Sprintf(wd, "/config/samples/istio-sample-openshift.yaml")

		// Add there more setup code specific to OpenShift
	} else {
		istio_manifest = fmt.Sprintf(wd, "/config/samples/istio-sample-kubernetes.yaml")

		// Add there more setup code specific to Kubernetes
	}

}
