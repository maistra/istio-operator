package v2

// ControlPlaneClusterConfig configures aspects related to clustering.
type ControlPlaneClusterConfig struct {
	// .Values.global.multiCluster.clusterName, defaults to Kubernetes
	Name string `json:"name,omitempty"`
	// .Values.global.network
	// XXX: not sure what the difference is between this and cluster name
	Network string `json:"network,omitempty"`
	// .Values.global.multiCluster.enabled, if not null
	MultiCluster *MultiClusterConfig `json:"multiCluster,omitempty"`
	// .Values.global.meshExpansion.enabled, if not null
	// XXX: it's not clear whether or not there is any overlap with MultiCluster,
	// i.e. does MultiCluster require mesh expansion ports to be configured on
	// the ingress gateway?
	MeshExpansion *MeshExpansionConfig `json:"meshExpansion,omitempty"`
}

// MultiClusterConfig configures aspects related to multi-cluster.
// implies the following:
// adds external to RequestedNetworkView (ISTIO_META_REQUESTED_NETWORK_VIEW) for egress gateway
// adds "global" and "{{ valueOrDefault .DeploymentMeta.Namespace \"default\" }}.global" to pod dns search suffixes
type MultiClusterConfig struct {
	// .Values.global.k8sIngress.enabled
	// implies the following:
	// .Values.global.k8sIngress.gatewayName will match the ingress gateway
	// .Values.global.k8sIngress.enableHttps will be true if gateway service exposes port 443
	// XXX: not sure whether or not this is specific to multicluster, mesh expansion, or both
	Ingress bool `json:"ingress,omitempty"`
	// .Values.global.meshNetworks
	// XXX: if non-empty, local cluster network should be configured as:
	//  <spec.cluster.network>:
	//      endpoints:
	//      - fromRegistry: <spec.cluster.name>
	//      gateways:
	//      - service: <ingress-gateway-service-name>
	//        port: 443 # mtls port
	MeshNetworks map[string]MeshNetworkConfig `json:"meshNetworks,omitempty"`
}

// MeshExpansionConfig configures aspects related to mesh expansion
type MeshExpansionConfig struct {
	// .Values.global.meshExpansion.useILB, true if not null, otherwise uses ingress gateway
	ILBGateway *ILBGatewayConfig `json:"ilbGateway,omitempty"`
}

// ILBGatewayConfig configures an internal load balancer gateway for use with mesh expansion.
type ILBGatewayConfig struct {
	// ports for ILB gateway are hard coded
	// service type is hard-coded to LoadBalancer
	// .Values.gateways.istio-ilbgateway service specific settings (ports, type, etc.)
	Service GatewayServiceConfig `json:"service,omitempty"`
	// .Values.gateways.istio-ilbgateway secretVolumes and configMapVolumes
	Volumes []VolumeConfig `json:"volumes,omitempty"`
	// .Values.gateways.istio-ilbgateway runtime details (resources, autoscaling, etc.)
	Runtime ComponentRuntimeConfig `json:"runtime,omitempty"`
}

// MeshNetworkConfig configures mesh networks for a multi-cluster mesh.
type MeshNetworkConfig struct {
	Endpoints []MeshEndpointConfig `json:"endpoints,omitempty"`
	Gateways  []MeshGatewayConfig  `json:"gateways,omitempty"`
}

// MeshEndpointConfig specifies the endpoint of a mesh network.  Only one of
// FromRegistry or FromCIDR may be specified
type MeshEndpointConfig struct {
	FromRegistry string `json:"fromRegistry,omitempty"`
	FromCIDR     string `json:"fromCIDR,omitempty"`
}

// MeshGatewayConfig specifies the gateway which should be used for accessing
// the network
type MeshGatewayConfig struct {
	Service string `json:"service,omitempty"`
	Address string `json:"address,omitempty"`
	Port    int32  `json:"port,omitempty"`
}
