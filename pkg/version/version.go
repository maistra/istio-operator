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
