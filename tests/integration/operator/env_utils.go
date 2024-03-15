//go:build e2e

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
	"strconv"

	g "github.com/onsi/ginkgo/v2"
)

var (
	ocp                   = getBoolEnv("OCP", false)
	skipDeploy            = getBoolEnv("SKIP_DEPLOY", false)
	image                 = getEnv("IMAGE", "quay.io/maistra-dev/istio-operator:latest")
	namespace             = getEnv("NAMESPACE", "sail-operator")
	deploymentName        = getEnv("DEPLOYMENT_NAME", "sail-operator")
	controlPlaneNamespace = getEnv("CONTROL_PLANE_NS", "istio-system")
	wd, _                 = os.Getwd()
	istioName             = getEnv("ISTIO_NAME", "default")
	baseDir               = filepath.Join(wd, "../../..")
)

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	g.GinkgoWriter.Printf("Env variable %s is set to %s\n", key, value)

	return value
}

func getBoolEnv(key string, defaultValue bool) bool {
	value := getEnv(key, strconv.FormatBool(defaultValue))
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}

	return boolValue
}
