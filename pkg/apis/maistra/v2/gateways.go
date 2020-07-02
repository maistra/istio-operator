package v2

import (
	corev1 "k8s.io/api/core/v1"
)

// GatewaysConfig configures gateways for the mesh
type GatewaysConfig struct {
	// Ingress configures the ingress gateway for the mesh
	// works in conjunction with cluster.meshExpansion.ingress configuration
	// (for enabling ILB gateway and mesh expansion ports)
	// .Values.gateways.istio-ingressgateway
	Ingress *IngressGatewayConfig `json:"ingress,omitempty"`
	// Egress configures the egress gateway for the mesh.
	// .Values.gateways.istio-egressgateway
	Egress *GatewayConfig `json:"egress,omitempty"`
	// Other user defined gateways
	// .Values.gateways.<key>
	AdditionalGateways map[string]GatewayConfig `json:"additional,omitempty"`
}

// GatewayConfig represents the configuration for a gateway
// XXX: should standard istio secrets be configured automatically, i.e. should
// the user be forced to add these manually?
type GatewayConfig struct {
	// Namespace is the namespace within which the gateway will be installed,
	// defaults to control plane namespace.
	// .Values.gateways.<gateway-name>.namespace
	// XXX: for the standard gateways, it might be possible that related
	// resources could be installed in control plane namespace instead of the
	// gateway namespace.  not sure if this is a problem or not.
	Namespace string `json:"namespace,omitempty"`
	// Service configures the service associated with the gateway, e.g. port
	// mappings, service type, annotations/labels, etc.
	// .Values.gateways.<gateway-name>.ports, .Values.gateways.<gateway-name>.type,
	// .Values.gateways.<gateway-name>.loadBalancerIP,
	// .Values.gateways.<gateway-name>.serviceAnnotations,
	// .Values.gateways.<gateway-name>.serviceLabels
	// XXX: currently there is no distinction between labels and serviceLabels
	Service GatewayServiceConfig `json:"service,omitempty"`
	// The router mode to be used by the gateway.
	// .Values.gateways.<gateway-name>.env.ISTIO_META_ROUTER_MODE, defaults to sni-dnat
	RouterMode RouterModeType `json:"routerMode,omitempty"`
	// RequestedNetworkView is a list of networks whose services should be made
	// available to the gateway.  This is used primarily for mesh expansion/multi-cluster.
	// .Values.gateways.<gateway-name>.env.ISTIO_META_REQUESTED_NETWORK_VIEW env, defaults to empty list
	// XXX: I think this is only applicable to egress gateways
	RequestedNetworkView []string `json:"requestedNetworkView,omitempty"`
	// EnableSDS for the gateway.
	// .Values.gateways.<gateway-name>.sds.enabled
	// XXX: I believe this is only applicable to ingress gateways
	EnableSDS *bool `json:"enableSDS,omitempty"`
	// Volumes is used to configure additional Secret and ConfigMap volumes that
	// should be mounted for the gateway's pod.
	// .Values.gateways.<gateway-name>.secretVolumes, .Values.gateways.<gateway-name>.configMapVolumes
	Volumes []VolumeConfig `json:"volumes,omitempty"`
	// Runtime is used to configure execution parameters for the pod/containers
	// e.g. resources, replicas, etc.
	Runtime *ComponentRuntimeConfig `json:"runtime,omitempty"`
	// XXX: do we need to support additionalContainers???
}

type IngressGatewayConfig struct {
	GatewayConfig `json:",inline"`
	// MeshExpansionPorts define the port set used with multi-cluster/mesh expansion
	MeshExpansionPorts []corev1.ServicePort `json:"meshExpansionPorts,omitempty"`
}

// RouterModeType represents the router modes available.
type RouterModeType string

const (
	// RouterModeTypeSNIDNAT represents sni-dnat router mode
	RouterModeTypeSNIDNAT RouterModeType = "sni-dnat"
	// RouterModeTypeStandard represents standard router mode
	RouterModeTypeStandard RouterModeType = "standard"
)

// GatewayServiceConfig configures the k8s Service associated with the gateway
type GatewayServiceConfig struct {
	// XXX: selector is ignored
	// Service details used to configure the gateway's Service resource
	corev1.ServiceSpec `json:",inline"`
	// metadata to be applied to the gateway's service (annotations and labels)
	Metadata MetadataConfig `json:"metadata,omitempty"`
}

// VolumeConfig is used to specify volumes that should be mounted on the pod.
type VolumeConfig struct {
	// Volume.Name maps to .Values.gateways.<gateway-name>.<type>.<type-name> (type-name is configMapName or secretName)
	// .configVolumes -> .configMapName = volume.name
	// .secretVolumes -> .secretName = volume.name
	// Only ConfigMap and Secret fields are supported
	Volume GatewayVolume `json:"volume,omitempty"`
	// Mount.Name maps to .Values.gateways.<gateway-name>.<type>.name
	// .configVolumes -> .name = mount.name, .mountPath = mount.mountPath
	// .secretVolumes -> .name = mount.name, .mountPath = mount.mountPath
	// Only Name and MountPath fields are supported
	Mount corev1.VolumeMount `json:"volumeMount,omitempty"`
}

// GatewayVolume is a pared down version of corev1.Volume, which only supports
// specifying ConfigMap and Secret volume types.
type GatewayVolume struct {
	// Volume's name.
	// Must be a DNS_LABEL and unique within the pod.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name"`
	// ConfigMap represents a configMap that should populate this volume
	// +optional
	ConfigMap *corev1.ConfigMapVolumeSource `json:"configMap,omitempty"`
	// Secret represents a secret that should populate this volume.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#secret
	// +optional
	Secret *corev1.SecretVolumeSource `json:"secret,omitempty"`
}
