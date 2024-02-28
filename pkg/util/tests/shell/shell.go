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
package shell

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ExecuteCommandWithInput executes a command given the input string and returns the output and err if any
func ExecuteCommandWithInput(command string, input string) (string, error) {
	return ExecuteShell(command, input)
}

// ExecuteCommandWithInput executes a command given the input string and returns the output and err if any
func ExecuteCommand(command string) (string, error) {
	return ExecuteShell(command, "")
}

// ExecuteCommand executes a command given the input string and returns the output and err if any
func ExecuteShell(command string, input string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		// We need to return the output as well to help debugging
		return string(out), fmt.Errorf("error executing command: %v, output: %s", err, string(out))
	}

	return string(out), nil
}
