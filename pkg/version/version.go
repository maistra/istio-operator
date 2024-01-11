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

package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

var (
	buildVersion     = "unknown"
	buildGitRevision = "unknown"
	buildStatus      = "unknown"
	buildTag         = "unknown"

	// Info exports the build version information.
	Info BuildInfo
)

// BuildInfo describes version information about the binary build.
type BuildInfo struct {
	Version           string
	GitRevision       string
	BuildStatus       string
	GitTag            string
	GoVersion         string
	GoArch            string
	ControllerRuntime string
}

func (b BuildInfo) String() string {
	return fmt.Sprintf("%#v", b)
}

func init() {
	var controllerRuntimeVersion string
	bi, ok := debug.ReadBuildInfo()
	if ok {
		for _, dep := range bi.Deps {
			if dep.Path == "sigs.k8s.io/controller-runtime" {
				controllerRuntimeVersion = dep.Version
				break
			}
		}
	}
	Info = BuildInfo{
		Version:           buildVersion,
		GitRevision:       buildGitRevision,
		BuildStatus:       buildStatus,
		GitTag:            buildTag,
		GoVersion:         runtime.Version(),
		GoArch:            fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		ControllerRuntime: controllerRuntimeVersion,
	}
}
