package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
)

func init() {
	SchemeBuilder.Register(&ServiceMeshControlPlane{}, &ServiceMeshControlPlaneList{})
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceMeshControlPlane is the Schema for the controlplanes API
// +k8s:openapi-gen=true
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=smcp,categories=maistra-io
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.annotations.readyComponentCount",description="How many of the total number of components are ready"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].reason",description="Whether or not the control plane installation is up to date and ready to handle requests."
// +kubebuilder:printcolumn:name="Template",type="string",JSONPath=".status.lastAppliedConfiguration.template",description="The configuration template to use as the base."
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.lastAppliedConfiguration.version",description="The actual current version of the control plane installation."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"
// +kubebuilder:printcolumn:name="Image HUB",type="string",JSONPath=".status.lastAppliedConfiguration.istio.global.hub",description="The image hub used as the base for all component images.",priority=1
type ServiceMeshControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneSpec   `json:"spec,omitempty"`
	Status ControlPlaneStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceMeshControlPlaneList contains a list of ServiceMeshControlPlane
type ServiceMeshControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceMeshControlPlane `json:"items"`
}

// ControlPlaneStatus defines the observed state of ServiceMeshControlPlane
type ControlPlaneStatus struct {
	maistrav1.ControlPlaneStatus `json:",inline"`
}

// ControlPlaneSpec represents the configuration for installing a control plane
type ControlPlaneSpec struct {
	// XXX: the resource name is intended to be used as the revision name, which
	// is used by istio.io/rev labels/annotations to specify which control plane
	// workloads should be connecting with.

	// Profiles selects the profile to use for default values. Defaults to
	// "default" when not set.
	// +optional
	Profiles []string `json:"profiles,omitempty"`

	// Version specifies what Maistra version of the control plane to install.
	// When creating a new ServiceMeshControlPlane with an empty version, the
	// admission webhook sets the version to the current version.
	// Existing ServiceMeshControlPlanes with an empty version are treated as
	// having the version set to "v1.0"
	// +optional
	Version string `json:"version,omitempty"`
	// Cluster is the general configuration of the cluster (cluster name,
	// network name, multi-cluster, mesh expansion, etc.)
	// +optional
	Cluster *ControlPlaneClusterConfig `json:"cluster,omitempty"`
	// Logging represents the logging configuration for the control plane components
	// XXX: Should this be separate from Proxy.Logging?
	// +optional
	Logging *LoggingConfig `json:"logging,omitempty"`
	// Policy configures policy checking for the control plane.
	// .Values.policy.enabled, true if not null
	// +optional
	Policy *PolicyConfig `json:"policy,omitempty"`
	// Proxy configures the default behavior for sidecars.  Many values were
	// previously exposed through .Values.global.proxy
	// +optional
	Proxy *ProxyConfig `json:"proxy,omitempty"`
	// Security configures aspects of security for the control plane.
	// +optional
	Security *SecurityConfig `json:"security,omitempty"`
	// Telemetry configures telemetry for the mesh.
	// .Values.mixer.telemetry.enabled, true if not null.  1.6, .Values.telemetry.enabled
	// +optional
	Telemetry *TelemetryConfig `json:"telemetry,omitempty"`
	// Gateways configures gateways for the mesh
	// .Values.gateways.*
	// +optional
	Gateways *GatewaysConfig `json:"gateways,omitempty"`
	// Runtime configuration for pilot (and galley, etc., pre 2.0)
	// +optional
	Runtime *ControlPlaneRuntimeConfig `json:"runtime,omitempty"`
	// Addons is used to configure additional features beyond core control plane
	// components, e.g. visualization, metric storage, etc.
	// +optional
	Addons *AddonsConfig `json:"addons,omitempty"`
}

// Enablement is a common definition for features that can be enabled
type Enablement struct {
	// Enabled specifies whether or not this feature is enabled
	Enabled *bool `json:"enabled,omitempty"`
}
