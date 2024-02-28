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

	g "github.com/onsi/ginkgo/v2"
	"maistra.io/istio-operator/pkg/util/tests/shell"
)

// hemlTemplate runs helm template in the given directory with params and returns the output yaml string
// name: name of the helm release
// chart: chart directory
// ns: namespace
// args: additional helm args, for example "--set image=Image"
func Template(name string, chart string, ns string, args ...string) (string, error) {
	g.GinkgoWriter.Println("Running Helm template")
	argsStr := strings.Join(args, " ")
	command := fmt.Sprintf("helm template %s %s --namespace %s %s", name, chart, ns, argsStr)
	outputString, err := shell.ExecuteCommand(command)
	if err != nil {
		g.GinkgoWriter.Printf("Error running Helm template: %s", outputString)
		return "", err
	}

	g.GinkgoWriter.Println("Helm template executed successfully")
	return outputString, nil
}
