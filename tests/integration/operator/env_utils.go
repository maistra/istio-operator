// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integrationoperator

import (
	"os"
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
)

var (
	ocp                   = getEnvOrDefault("OCP", "false")
	skipDeploy            = getEnvOrDefault("SKIP_DEPLOY", "false")
	image                 = getEnvOrDefault("IMAGE", "quay.io/maistra-dev/istio-operator:latest")
	istioManifest         = getEnvOrDefault("ISTIO_MANIFEST", "chart/samples/istio-sample-kubernetes.yaml")
	namespace             = getEnvOrDefault("NAMESPACE", "istio-operator")
	deploymentName        = getEnvOrDefault("DEPLOYMENT_NAME", "istio-operator")
	controlPlaneNamespace = getEnvOrDefault("CONTROL_PLANE_NS", "istio-system")
	wd, _                 = os.Getwd()
	istioName             = getEnvOrDefault("ISTIO_NAME", "default")
	baseDir               = filepath.Join(wd, "../../..")
)

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	g.GinkgoWriter.Printf("Env variable %s is set to %s\n", key, value)

	return value
}
