package versions

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/helm/pkg/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	ver "github.com/maistra/istio-operator/pkg/version"
)

const (
	// InvalidVersion is not a valid version
	InvalidVersion Ver = iota
	// V1_1 -> v1.1
	V1_1
	// V2_0 -> v2.0
	V2_0
	// V2_1 -> v2.1
	V2_1
	// V2_2 -> v2.2
	V2_2
	// V2_3 -> v2.3
	V2_3
	// Add new versions here, above lastKnownVersion. Remember to add a string mapping in init() below
	lastKnownVersion Ver = iota - 1
)

var (
	AllV2Versions  = []Version{V2_0, V2_1, V2_2, V2_3}
	legacyVersions = map[string]bool{
		"v1.0": true,
	}
)

func init() {
	versionToString = map[Ver]string{
		InvalidVersion: "InvalidVersion",
		V1_1:           "v1.1",
		V2_0:           "v2.0",
		V2_1:           "v2.1",
		V2_2:           "v2.2",
		V2_3:           "v2.3",
	}

	versionToStrategy = map[Ver]VersionStrategy{
		InvalidVersion: &invalidVersionStrategy{InvalidVersion},
		V1_1:           &versionStrategyV1_1{Ver: V1_1},
		V2_0:           &versionStrategyV2_0{Ver: V2_0},
		V2_1:           &versionStrategyV2_1{Ver: V2_1},
		V2_2:           &versionStrategyV2_2{Ver: V2_2},
		V2_3:           &versionStrategyV2_3{Ver: V2_3},
	}

	versionToCNINetwork = map[Ver]string{
		InvalidVersion: "",
		V1_1:           "v1-1-istio-cni",
		V2_0:           "v2-0-istio-cni",
		V2_1:           "v2-1-istio-cni",
		V2_2:           "v2-2-istio-cni",
		V2_3:           "v2-3-istio-cni",
	}

	for v, str := range versionToString {
		if v != InvalidVersion {
			stringToVersion[str] = v
		}
	}
	minimumSupportedVersion := ver.Info.MinimumSupportedVersion
	minVersion := stringToVersion[minimumSupportedVersion]
	if minVersion == InvalidVersion {
		panic(fmt.Sprintf("invalid minimum supported version: %v", minimumSupportedVersion))
	}

	for v := range versionToString {
		if v >= minVersion {
			supportedVersions = append(supportedVersions, v)
			supportedVersionNames = append(supportedVersionNames, v.String())
		}
	}
}

const (
	// DefaultVersion to use for new resources which have no version specified.
	DefaultVersion = lastKnownVersion
)

// Version represents a version of a control plane, major.minor, usually
// identified as something like v1.1.  Version objects are guaranteed to be
// sequentually ordered from oldest to newest.
type Version interface {
	fmt.Stringer
	// Version returns the internal version representation
	Version() Ver
	// Compare compares this version with another version.  If other is an older
	// version, a positive value will be returned.  If other is a newer version,
	// a negative value is returned.  If other is the same version, zero is
	// returned.
	Compare(other Version) int
	// Strategy provides a customizations specific to this version.
	Strategy() VersionStrategy
	GetChartsDir() string
	GetUserTemplatesDir() string
	GetDefaultTemplatesDir() string
	GetCNINetworkName() string
	IsSupported() bool
}

// ValidationStrategy is an interface used by the validating webhook for validating SMCP resources.
type ValidationStrategy interface {
	ValidateV1(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error
	ValidateV2(ctx context.Context, cl client.Client, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec) error
	ValidateDowngrade(ctx context.Context, cl client.Client, smcp metav1.Object) error
	ValidateUpgrade(ctx context.Context, cl client.Client, smcp metav1.Object) error
}

// RenderingStrategy is an interface used by the reconciler to manage rendering of charts.
type RenderingStrategy interface {
	GetChartInstallOrder() [][]string
	SetImageValues(ctx context.Context, cr *common.ControllerResources, smcp *v1.ControlPlaneSpec) error
	Render(ctx context.Context, cr *common.ControllerResources, cniConfig cni.Config, smcp *v2.ServiceMeshControlPlane) (map[string][]manifest.Manifest, error)
	// for testing purposes
	ApplyProfiles(ctx context.Context, cr *common.ControllerResources, smcpSpec *v1.ControlPlaneSpec, targetNamespace string) (v1.ControlPlaneSpec, error)
}

// ConversionStrategy is an interface used when converting between v1 and v2 of the SMCP resource.
type ConversionStrategy interface {
	GetExpansionPorts() []corev1.ServicePort
	GetTelemetryType(in *v1.HelmValues, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled bool) v2.TelemetryType
	GetPolicyType(in *v1.HelmValues, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled bool) v2.PolicyType
	GetTrustDomainFieldPath() string
}

// VersionStrategy provides encapsulates customization required for a particular
// version.
type VersionStrategy interface {
	Version
	RenderingStrategy
	ValidationStrategy
	ConversionStrategy
}

// GetSupportedVersions returns a list of versions supported by this operator
func GetSupportedVersions() []Version {
	return supportedVersions
}

func GetSupportedVersionNames() []string {
	return supportedVersionNames
}

type Ver int

var _ Version = Ver(0)

func (v Ver) String() string {
	if str, ok := versionToString[v]; ok {
		return str
	}
	panic(fmt.Sprintf("invalid version: %d", v))
}

func (v Ver) Compare(other Version) int {
	return int(v.Version() - other.Version())
}

func (v Ver) Version() Ver {
	return v
}

func (v Ver) Strategy() VersionStrategy {
	if strategy, ok := versionToStrategy[v]; ok {
		return strategy
	}
	panic(fmt.Sprintf("invalid version: %d", v))
}

func (v Ver) GetCNINetworkName() string {
	if network, ok := versionToCNINetwork[v]; ok {
		return network
	}
	panic(fmt.Sprintf("invalid version: %d", v))
}

func (v Ver) IsSupported() (supported bool) {
	for _, version := range supportedVersions {
		if version == v {
			supported = true
			return
		}
	}
	return
}

// ParseVersion returns a version for the specified string
func ParseVersion(str string) (Ver, error) {
	if v, ok := stringToVersion[str]; ok {
		return v, nil
	}
	if legacyVersions[str] {
		return InvalidVersion, fmt.Errorf("support for %s has been dropped", str)
	}
	return InvalidVersion, fmt.Errorf("invalid version: %s", str)
}

type nilVersionStrategy struct {
	Ver
}

var _ VersionStrategy = (*nilVersionStrategy)(nil)

func (v *nilVersionStrategy) SetImageValues(ctx context.Context, cr *common.ControllerResources, smcp *v1.ControlPlaneSpec) error {
	return nil
}

func (v *nilVersionStrategy) ValidateV1(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	return nil
}

func (v *nilVersionStrategy) ValidateV2(ctx context.Context, cl client.Client, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec) error {
	return nil
}

func (v *nilVersionStrategy) ValidateDowngrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	return nil
}

func (v *nilVersionStrategy) ValidateUpgrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	return nil
}

func (v *nilVersionStrategy) GetChartInstallOrder() [][]string {
	return nil
}

func (v *nilVersionStrategy) Render(
	ctx context.Context, cr *common.ControllerResources, cniConfig cni.Config, smcp *v2.ServiceMeshControlPlane,
) (map[string][]manifest.Manifest, error) {
	return nil, fmt.Errorf("nil version does not support rendering")
}

func (v *nilVersionStrategy) GetExpansionPorts() []corev1.ServicePort {
	return nil
}

func (v *nilVersionStrategy) GetTelemetryType(in *v1.HelmValues, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled bool) v2.TelemetryType {
	return ""
}

func (v *nilVersionStrategy) GetPolicyType(in *v1.HelmValues, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled bool) v2.PolicyType {
	return ""
}

func (v *nilVersionStrategy) GetTrustDomainFieldPath() string {
	return ""
}

type invalidVersionStrategy struct {
	Ver
}

var _ VersionStrategy = (*invalidVersionStrategy)(nil)

func (v *invalidVersionStrategy) SetImageValues(ctx context.Context, cr *common.ControllerResources, smcp *v1.ControlPlaneSpec) error {
	return fmt.Errorf("invalid version: %s", v.Ver)
}

func (v *invalidVersionStrategy) ValidateV1(ctx context.Context, cl client.Client, smcp *v1.ServiceMeshControlPlane) error {
	return fmt.Errorf("invalid version: %s", v.Ver)
}

func (v *invalidVersionStrategy) ValidateV2(ctx context.Context, cl client.Client, meta *metav1.ObjectMeta, spec *v2.ControlPlaneSpec) error {
	return fmt.Errorf("invalid version: %s", v.Ver)
}

func (v *invalidVersionStrategy) ValidateDowngrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	return fmt.Errorf("invalid version: %s", v.Ver)
}

func (v *invalidVersionStrategy) ValidateUpgrade(ctx context.Context, cl client.Client, smcp metav1.Object) error {
	return fmt.Errorf("invalid version: %s", v.Ver)
}

func (v *invalidVersionStrategy) GetChartInstallOrder() [][]string {
	return nil
}

func (v *invalidVersionStrategy) Render(ctx context.Context, cr *common.ControllerResources,
	cniConfig cni.Config, smcp *v2.ServiceMeshControlPlane,
) (map[string][]manifest.Manifest, error) {
	return nil, fmt.Errorf("invalid version: %s", v.Ver)
}

func (v *invalidVersionStrategy) GetExpansionPorts() []corev1.ServicePort {
	return nil
}

func (v *invalidVersionStrategy) GetTelemetryType(in *v1.HelmValues, mixerTelemetryEnabled, mixerTelemetryEnabledSet, remoteEnabled bool) v2.TelemetryType {
	return ""
}

func (v *invalidVersionStrategy) GetPolicyType(in *v1.HelmValues, mixerPolicyEnabled, mixerPolicyEnabledSet, remoteEnabled bool) v2.PolicyType {
	return ""
}

func (v *invalidVersionStrategy) GetTrustDomainFieldPath() string {
	return ""
}

var (
	versionToString       = make(map[Ver]string)
	versionToCNINetwork   = make(map[Ver]string)
	versionToStrategy     = make(map[Ver]VersionStrategy)
	stringToVersion       = make(map[string]Ver)
	supportedVersions     []Version
	supportedVersionNames []string
)
