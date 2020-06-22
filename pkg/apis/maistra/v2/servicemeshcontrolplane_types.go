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
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=servicemeshcontrolplanes,scope=Namespaced
// +kubebuilder:resource:shortName=smcp
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

	// Version specifies what Maistra version of the control plane to install.
	// When creating a new ServiceMeshControlPlane with an empty version, the
	// admission webhook sets the version to the current version.
	// Existing ServiceMeshControlPlanes with an empty version are treated as
	// having the version set to "v1.0"
	Version string `json:"version,omitempty"`
	// Cluster is the general configuration of the cluster (cluster name,
	// network name, multi-cluster, mesh expansion, etc.)
	Cluster *ControlPlaneClusterConfig `json:"cluster,omitempty"`
	// Logging represents the logging configuration for the control plane components
	// XXX: Should this be separate from Proxy.Logging?
	Logging *LoggingConfig `json:"logging,omitempty"`
	// Policy configures policy checking for the control plane.
	// .Values.policy.enabled, true if not null
	Policy *PolicyConfig `json:"policy,omitempty"`
	// Proxy configures the default behavior for sidecars.  Many values were
	// previously exposed through .Values.global.proxy
	Proxy *ProxyConfig `json:"proxy,omitempty"`
	// Security configures aspects of security for the control plane.
	Security *SecurityConfig `json:"security,omitempty"`
	// Telemetry configures telemetry for the mesh.
	// .Values.mixer.telemetry.enabled, true if not null.  1.6, .Values.telemetry.enabled
	Telemetry *TelemetryConfig `json:"telemetry,omitempty"`
	// Gateways configures gateways for the mesh
	// .Values.gateways.*
	Gateways *GatewaysConfig `json:"gateways,omitempty"`
	// Runtime configuration for pilot (and galley, etc., pre 1.2)
	Runtime *ControlPlaneRuntimeConfig `json:"runtime,omitempty"`
	// Addons is used to configure additional features beyond core control plane
	// components, e.g. visualization, metric storage, etc.
	Addons *AddonsConfig `json:"addons,omitempty"`
}
