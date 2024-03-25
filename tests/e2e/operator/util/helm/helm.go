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

package helm

import (
	"fmt"
	"strings"

	g "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/operator/util/shell"
)

// Install runs helm install in the given directory with params
// name: name of the helm release
// chart: chart directory
// args: additional helm args, for example "--set image=Image"
func Install(name string, chart string, args ...string) error {
	argsStr := strings.Join(args, " ")
	command := fmt.Sprintf("helm install %s %s %s", name, chart, argsStr)
	output, err := shell.ExecuteCommand(command)
	if err != nil {
		return fmt.Errorf("error running %s: %s. Output: %s", command, err, output)
	}

	g.Success("Helm install executed successfully")
	return nil
}

// Uninstall runs helm uninstall in the given directory with params
// name: name of the helm release
// args: additional helm args, for example "--namespace sail-operator"
func Uninstall(name string, args ...string) error {
	argsStr := strings.Join(args, " ")
	command := fmt.Sprintf("helm uninstall %s %s", name, argsStr)
	output, err := shell.ExecuteCommand(command)
	if err != nil {
		return fmt.Errorf("error running Helm uninstall: %s. Output: %s", err, output)
	}

	g.Success("Helm uninstall executed successfully")
	return nil
}
