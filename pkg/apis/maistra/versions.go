package maistra

import "fmt"

const (
	// InvalidVersion is not a valid version
	InvalidVersion version = iota
	// UndefinedVersion is...undefined
	UndefinedVersion
	// V1_0 -> v1.0
	V1_0
	// V1_1 -> v1.1
	V1_1
	// Add new versions here, above lastKnownVersion. Remember to add a string mapping in init() below
	lastKnownVersion version = iota - 1
)

func init() {
	versionToString = map[version]string{
		InvalidVersion:   "InvalidVersion",
		UndefinedVersion: "",
		V1_0:             "v1.0",
		V1_1:             "v1.1",
	}

	for v, str := range versionToString {
		if v != InvalidVersion {
			stringToVersion[str] = v
			if v != UndefinedVersion {
				supportedVersions = append(supportedVersions, v)
			}
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

// ParseVersion returns a version for the specified string
func ParseVersion(str string) (Version, error) {
	if v, ok := stringToVersion[str]; ok {
		return v.Version(), nil
	}
	return InvalidVersion, fmt.Errorf("invalid version: %s", str)
}

var versionToString = make(map[version]string)
var stringToVersion = make(map[string]version)
var supportedVersions []Version
