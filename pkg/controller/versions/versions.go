package versions

import (
	"context"
	"fmt"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// InvalidVersion is not a valid version
	InvalidVersion version = iota
	// V1_0 -> v1.0
	V1_0
	// V1_1 -> v1.1
	V1_1
	// V1_2 -> v1.2
	V1_2
	// Add new versions here, above lastKnownVersion. Remember to add a string mapping in init() below
	lastKnownVersion version = iota - 1
)

func init() {
	versionToString = map[version]string{
		InvalidVersion: "InvalidVersion",
		V1_0:           "v1.0",
		V1_1:           "v1.1",
		V1_2:           "v1.2",
	}

	versionToStrategy = map[version]VersionStrategy{
		InvalidVersion: &invalidVersionStrategy{InvalidVersion},
		V1_0:           &versionStrategyV1_0{V1_0},
		V1_1:           &versionStrategyV1_1{V1_1},
		V1_2:           &versionStrategyV1_2{V1_2},
	}

	for v, str := range versionToString {
		if v != InvalidVersion {
			stringToVersion[str] = v
			if v == V1_0 {
				// special handling for legacy case
				stringToVersion[""] = v
			}
			supportedVersions = append(supportedVersions, v)
		}
	}
}

const (
	// DefaultVersion to use for new resources which have no version specified.
	DefaultVersion = lastKnownVersion
	// LegacyVersion to use with existing resources which have no version specified.
	LegacyVersion = V1_0
)

// Version represents a version of a control plane, major.minor, usually
// identified as something like v1.0.  Version objects are guaranteed to be
// sequentually ordered from oldest to newest.
type Version interface {
	fmt.Stringer
	// Version returns the internal version representation
	Version() version
	// Compare compares this version with another version.  If other is an older
	// version, a positive value will be returned.  If other is a newer version,
	// a negative value is returned.  If other is the same version, zero is
	// returned.
	Compare(other Version) int
	// Strategy provides a customizations specific to this version.
	Strategy() VersionStrategy
}

// VersionStrategy provides encapsulates customization required for a particular
// version.
type VersionStrategy interface {
	Version
	SetImageValues(ctx context.Context, cl client.Client, smcp *v1.ControlPlaneSpec) error
	Validate(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error
	ValidateDowngrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error
	ValidateUpgrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error
}

// GetSupportedVersions returns a list of versions supported by this operator
func GetSupportedVersions() []Version {
	return supportedVersions
}

type version int

var _ Version = version(0)

func (v version) String() string {
	if str, ok := versionToString[v]; ok {
		return str
	}
	panic(fmt.Sprintf("invalid version: %d", v))
}

func (v version) Compare(other Version) int {
	return int(v.Version() - other.Version())
}

func (v version) Version() version {
	return v
}

func (v version) Strategy() VersionStrategy {
	if strategy, ok := versionToStrategy[v]; ok {
		return strategy
	}
	panic(fmt.Sprintf("invalid version: %d", v))
}

// ParseVersion returns a version for the specified string
func ParseVersion(str string) (Version, error) {
	if v, ok := stringToVersion[str]; ok {
		return v.Version(), nil
	}
	return InvalidVersion, fmt.Errorf("invalid version: %s", str)
}

type nilVersionStrategy struct {
	version
}

var _ VersionStrategy = (*nilVersionStrategy)(nil)

func (v *nilVersionStrategy) SetImageValues(ctx context.Context, cl client.Client, smcp *v1.ControlPlaneSpec) error {
	return nil
}
func (v *nilVersionStrategy) Validate(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	return nil
}
func (v *nilVersionStrategy) ValidateDowngrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	return nil
}
func (v *nilVersionStrategy) ValidateUpgrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	return nil
}

type invalidVersionStrategy struct {
	version
}

var _ VersionStrategy = (*invalidVersionStrategy)(nil)

func (v *invalidVersionStrategy) SetImageValues(ctx context.Context, cl client.Client, smcp *v1.ControlPlaneSpec) error {
	return fmt.Errorf("invalid version: %s", v.version)
}
func (v *invalidVersionStrategy) Validate(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	return fmt.Errorf("invalid version: %s", v.version)
}
func (v *invalidVersionStrategy) ValidateDowngrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	return fmt.Errorf("invalid version: %s", v.version)
}
func (v *invalidVersionStrategy) ValidateUpgrade(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	return fmt.Errorf("invalid version: %s", v.version)
}

var versionToString = make(map[version]string)
var versionToStrategy = make(map[version]VersionStrategy)
var stringToVersion = make(map[string]version)
var supportedVersions []Version
