package integration_operator

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
)

const timeout = "180s"

var (
	command          = getEnvOrDefault("COMMAND", "kubectl")
	ocp              = getEnvOrDefault("OCP", "false")
	skipDeploy       = getEnvOrDefault("SKIP_DEPLOY", "false")
	image            = getEnvOrDefault("IMAGE", "quay.io/maistra-dev/istio-operator:latest")
	istio_manifest   = getEnvOrDefault("ISTIO_MANIFEST", "chart/samples/istio-sample-kubernetes.yaml")
	namespace        = getEnvOrDefault("NAMESPACE", "istio-operator")
	deployment_name  = getEnvOrDefault("DEPLOYMENT_NAME", "istio-operator")
	control_plane_ns = getEnvOrDefault("CONTROL_PLANE_NS", "istio-system")
	wd, _            = os.Getwd()
	istio_name       = getEnvOrDefault("ISTIO_NAME", "default")
)

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	GinkgoWriter.Println("Env variable %s is set to %s\n", key, value)

	return value
}
