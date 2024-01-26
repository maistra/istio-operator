/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package v1alpha1

import (
	"encoding/json"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"maistra.io/istio-operator/pkg/helm"
)

// Mode for the ingress controller.
type IngressControllerMode int32

// Specifies which tracer to use.
type Tracer int32

// Specifies the sidecar's default behavior when handling outbound traffic from the application.
type OutboundTrafficPolicyConfigMode int32

// ArchConfig specifies the pod scheduling target architecture(amd64, ppc64le, s390x, arm64)
// for all the Istio control plane components.
type ArchConfig struct {
	// Sets pod scheduling weight for amd64 arch
	Amd64 uint32 `json:"amd64,omitempty"`
	// Sets pod scheduling weight for ppc64le arch.
	Ppc64Le uint32 `json:"ppc64le,omitempty"`
	// Sets pod scheduling weight for s390x arch.
	S390X uint32 `json:"s390x,omitempty"`
	// Sets pod scheduling weight for arm64 arch.
	Arm64 uint32 `json:"arm64,omitempty"`
}

// Configuration for CNI.
type CNIConfig struct {
	// Controls whether CNI is enabled.
	Enabled bool   `json:"enabled,omitempty"`
	Hub     string `json:"hub,omitempty"`
	// +kubebuilder:validation:XIntOrString
	Tag               *intstr.IntOrString `json:"tag,omitempty"`
	Variant           string              `json:"variant,omitempty"`
	Image             string              `json:"image,omitempty"`
	PullPolicy        string              `json:"pullPolicy,omitempty"`
	CniBinDir         string              `json:"cniBinDir,omitempty"`
	CniConfDir        string              `json:"cniConfDir,omitempty"`
	CniConfFileName   string              `json:"cniConfFileName,omitempty"`
	CniNetnsDir       string              `json:"cniNetnsDir,omitempty"`
	ExcludeNamespaces []string            `json:"excludeNamespaces,omitempty"`
	Affinity          *k8sv1.Affinity     `json:"affinity,omitempty"`
	PspClusterRole    string              `json:"psp_cluster_role,omitempty"`
	LogLevel          string              `json:"logLevel,omitempty"`
	Repair            *CNIRepairConfig    `json:"repair,omitempty"`
	Chained           bool                `json:"chained,omitempty"`
	ResourceQuotas    *ResourceQuotas     `json:"resource_quotas,omitempty"`
	Resources         *Resources          `json:"resources,omitempty"`
	Privileged        bool                `json:"privileged,omitempty"`
	// The Container seccompProfile
	//
	// See: https://kubernetes.io/docs/tutorials/security/seccomp/
	SeccompProfile map[string]string `json:"seccompProfile,omitempty"`
	Ambient        *CNIAmbientConfig `json:"ambient,omitempty"`
	Provider       string            `json:"provider,omitempty"`
	// K8s rolling update strategy
	// +kubebuilder:validation:XIntOrString
	RollingMaxUnavailable *intstr.IntOrString `json:"rollingMaxUnavailable,omitempty"`
}

type CNIAmbientConfig struct {
	// Controls whether ambient redirection is enabled
	Enabled      bool   `json:"enabled,omitempty"`
	RedirectMode string `json:"redirectMode,omitempty"`
	ConfigDir    string `json:"configDir,omitempty"`
}

type CNIRepairConfig struct {
	// Controls whether repair behavior is enabled.
	Enabled bool   `json:"enabled,omitempty"`
	Hub     string `json:"hub,omitempty"`
	// +kubebuilder:validation:XIntOrString
	Tag   *intstr.IntOrString `json:"tag,omitempty"`
	Image string              `json:"image,omitempty"`
	// Controls whether various repair behaviors are enabled.
	LabelPods           bool   `json:"labelPods,omitempty"`
	RepairPods          bool   `json:"repairPods,omitempty"`
	DeletePods          bool   `json:"deletePods,omitempty"`
	BrokenPodLabelKey   string `json:"brokenPodLabelKey,omitempty"`
	BrokenPodLabelValue string `json:"brokenPodLabelValue,omitempty"`
	InitContainerName   string `json:"initContainerName,omitempty"`
}

type ResourceQuotas struct {
	// Controls whether to create resource quotas or not for the CNI DaemonSet.
	Enabled bool  `json:"enabled,omitempty"`
	Pods    int64 `json:"pods,omitempty"`
}

// Mirrors Resources for unmarshaling.
type Resources struct {
	Limits   map[string]string `json:"limits,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Requests map[string]string `json:"requests,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

// Mirrors ServiceAccount for unmarshaling.
type ServiceAccount struct {
	Annotations map[string]string `json:"annotations,omitempty"`
}

// DefaultPodDisruptionBudgetConfig specifies the default pod disruption budget configuration.
//
// See https://kubernetes.io/docs/concepts/workloads/pods/disruptions/
type DefaultPodDisruptionBudgetConfig struct {
	// Controls whether a PodDisruptionBudget with a default minAvailable value of 1 is created for each deployment.
	Enabled bool `json:"enabled,omitempty"`
}

// DefaultResourcesConfig specifies the default k8s resources settings for all Istio control plane components.
type DefaultResourcesConfig struct {
	// k8s resources settings.
	//
	// See https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container
	Requests *ResourcesRequestsConfig `json:"requests,omitempty"`
}

// Global Configuration for Istio components.
type GlobalConfig struct {
	// List of certSigners to allow "approve" action in the ClusterRole
	CertSigners         []string `json:"certSigners,omitempty"`
	ConfigRootNamespace string   `json:"configRootNamespace,omitempty"`
	// Controls whether the server-side validation is enabled.
	ConfigValidation                bool     `json:"configValidation,omitempty"`
	DefaultConfigVisibilitySettings []string `json:"defaultConfigVisibilitySettings,omitempty"`
	// Specifies the docker hub for Istio images.
	Hub string `json:"hub,omitempty"`
	// Specifies the image pull policy for the Istio images. one of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. Cannot be updated.
	//
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	ImagePullPolicy  string   `json:"imagePullPolicy,omitempty"` // ImagePullPolicy             v1.PullPolicy                 `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`
	// Specifies the default namespace for the Istio control plane components.
	IstioNamespace string `json:"istioNamespace,omitempty"`
	LogAsJSON      bool   `json:"logAsJson,omitempty"`
	// Specifies the global logging level settings for the Istio control plane components.
	Logging *GlobalLoggingConfig `json:"logging,omitempty"`
	MeshID  string               `json:"meshID,omitempty"`
	// Configure the mesh networks to be used by the Split Horizon EDS.
	//
	// The following example defines two networks with different endpoints association methods.
	// For `network1` all endpoints that their IP belongs to the provided CIDR range will be
	// mapped to network1. The gateway for this network example is specified by its public IP
	// address and port.
	// The second network, `network2`, in this example is defined differently with all endpoints
	// retrieved through the specified Multi-Cluster registry being mapped to network2. The
	// gateway is also defined differently with the name of the gateway service on the remote
	// cluster. The public IP for the gateway will be determined from that remote service (only
	// LoadBalancer gateway service type is currently supported, for a NodePort type gateway service,
	// it still need to be configured manually).
	//
	// meshNetworks:
	//
	//	network1:
	//	  endpoints:
	//	  - fromCidr: "192.168.0.1/24"
	//	  gateways:
	//	  - address: 1.1.1.1
	//	    port: 80
	//	network2:
	//	  endpoints:
	//	  - fromRegistry: reg1
	//	  gateways:
	//	  - registryServiceName: istio-ingressgateway.istio-system.svc.cluster.local
	//	    port: 443
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	MeshNetworks map[string]string `json:"meshNetworks,omitempty"`
	// Specifies the Configuration for Istio mesh across multiple clusters through Istio gateways.
	MultiCluster *MultiClusterConfig `json:"multiCluster,omitempty"`
	Network      string              `json:"network,omitempty"`
	// Custom DNS config for the pod to resolve names of services in other
	// clusters. Use this to add additional search domains, and other settings.
	// see https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#dns-config
	// This does not apply to gateway pods as they typically need a different
	// set of DNS settings than the normal application pods (e.g. in multicluster scenarios).
	PodDNSSearchNamespaces       []string `json:"podDNSSearchNamespaces,omitempty"`
	OmitSidecarInjectorConfigMap bool     `json:"omitSidecarInjectorConfigMap,omitempty"`
	// Controls whether to restrict the applications namespace the controller manages;
	// If set it to false, the controller watches all namespaces.
	OneNamespace           bool `json:"oneNamespace,omitempty"`
	OperatorManageWebhooks bool `json:"operatorManageWebhooks,omitempty"`
	// Specifies how proxies are configured within Istio.
	Proxy *ProxyConfig `json:"proxy,omitempty"`
	// Specifies the Configuration for proxy_init container which sets the pods' networking to intercept the inbound/outbound traffic.
	ProxyInit *ProxyInitConfig `json:"proxy_init,omitempty"`
	// Specifies the Configuration for the SecretDiscoveryService instead of using K8S secrets to mount the certificates.
	Sds *SDSConfig `json:"sds,omitempty"`
	// Specifies the tag for the Istio docker images.
	// +kubebuilder:validation:XIntOrString
	Tag     *intstr.IntOrString `json:"tag,omitempty"`
	Variant string              `json:"variant,omitempty"`
	// Specifies the Configuration for each of the supported tracers.
	Tracer *TracerConfig `json:"tracer,omitempty"`
	// Controls whether to use of Mesh Configuration Protocol to distribute configuration.
	UseMCP bool `json:"useMCP,omitempty"`
	// Specifies the Istio control plane’s pilot Pod IP address or remote cluster DNS resolvable hostname.
	RemotePilotAddress string `json:"remotePilotAddress,omitempty"`
	// Specifies the configution of istiod
	Istiod *IstiodConfig `json:"istiod,omitempty"`
	// Configure the Pilot certificate provider.
	// Currently, four providers are supported: "kubernetes", "istiod", "custom" and "none".
	PilotCertProvider string `json:"pilotCertProvider,omitempty"`
	// Configure the policy for validating JWT.
	// Currently, two options are supported: "third-party-jwt" and "first-party-jwt".
	JwtPolicy string `json:"jwtPolicy,omitempty"`
	// Specifies the configuration for Security Token Service.
	Sts *STSConfig `json:"sts,omitempty"`
	// Configures the revision this control plane is a part of
	Revision string `json:"revision,omitempty"`
	// Controls whether the in-cluster MTLS key and certs are loaded from the secret volume mounts.
	MountMtlsCerts bool `json:"mountMtlsCerts,omitempty"`
	// The address of the CA for CSR.
	CaAddress string `json:"caAddress,omitempty"`
	// Controls whether one external istiod is enabled.
	ExternalIstiod bool `json:"externalIstiod,omitempty"`
	// Controls whether a remote cluster is the config cluster for an external istiod
	ConfigCluster bool `json:"configCluster,omitempty"`
	// The name of the CA for workloads.
	// For example, when caName=GkeWorkloadCertificate, GKE workload certificates
	// will be used as the certificates for workloads.
	// The default value is "" and when caName="", the CA will be configured by other
	// mechanisms (e.g., environmental variable CA_PROVIDER).
	CaName           string `json:"caName,omitempty"`
	Autoscalingv2API bool   `json:"autoscalingv2API,omitempty"`
	// Platform in which Istio is deployed. Possible values are: "openshift" and "gcp"
	// An empty value means it is a vanilla Kubernetes distribution, therefore no special
	// treatment will be considered.
	Platform       string   `json:"platform,omitempty"`
	IPFamilies     []string `json:"ipFamilies,omitempty"`
	IPFamilyPolicy string   `json:"ipFamilyPolicy,omitempty"` // The next available key is 72
}

// Configuration for Security Token Service (STS) server.
//
// See https://tools.ietf.org/html/draft-ietf-oauth-token-exchange-16
type STSConfig struct {
	ServicePort uint32 `json:"servicePort,omitempty"`
}

type IstiodConfig struct {
	// If enabled, istiod will perform config analysis
	EnableAnalysis bool `json:"enableAnalysis,omitempty"`
}

// GlobalLoggingConfig specifies the global logging level settings for the Istio control plane components.
type GlobalLoggingConfig struct {
	// Comma-separated minimum per-scope logging level of messages to output, in the form of <scope>:<level>,<scope>:<level>
	// The control plane has different scopes depending on component, but can configure default log level across all components
	// If empty, default scope and level will be used as configured in code
	Level string `json:"level,omitempty"`
}

// MultiClusterConfig specifies the Configuration for Istio mesh across multiple clusters through the istio gateways.
type MultiClusterConfig struct {
	// Enables the connection between two kubernetes clusters via their respective ingressgateway services.
	// Use if the pods in each cluster cannot directly talk to one another.
	Enabled            bool   `json:"enabled,omitempty"`
	ClusterName        string `json:"clusterName,omitempty"`
	GlobalDomainSuffix string `json:"globalDomainSuffix,omitempty"`
	IncludeEnvoyFilter bool   `json:"includeEnvoyFilter,omitempty"`
}

// OutboundTrafficPolicyConfig controls the default behavior of the sidecar for handling outbound traffic from the application.
type OutboundTrafficPolicyConfig struct {
	Mode OutboundTrafficPolicyConfigMode `json:"mode,omitempty"`
}

// Configuration for Pilot.
type PilotConfig struct {
	// Controls whether Pilot is enabled.
	Enabled bool `json:"enabled,omitempty"`
	// Controls whether a HorizontalPodAutoscaler is installed for Pilot.
	AutoscaleEnabled bool `json:"autoscaleEnabled,omitempty"`
	// Minimum number of replicas in the HorizontalPodAutoscaler for Pilot.
	AutoscaleMin uint32 `json:"autoscaleMin,omitempty"`
	// Maximum number of replicas in the HorizontalPodAutoscaler for Pilot.
	AutoscaleMax uint32 `json:"autoscaleMax,omitempty"`
	// See https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#configurable-scaling-behavior
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	AutoscaleBehavior map[string]string `json:"autoscaleBehavior,omitempty"`
	// Image name used for Pilot.
	//
	// This can be set either to image name if hub is also set, or can be set to the full hub:name string.
	//
	// Examples: custom-pilot, docker.io/someuser:custom-pilot
	Image string `json:"image,omitempty"`
	// Trace sampling fraction.
	//
	// Used to set the fraction of time that traces are sampled. Higher values are more accurate but add CPU overhead.
	//
	// Allowed values: 0.0 to 1.0
	TraceSampling float64 `json:"traceSampling,omitempty"`
	// Namespace that the configuration management feature is installed into, if different from Pilot namespace.
	ConfigNamespace string `json:"configNamespace,omitempty"`
	// Maximum duration that a sidecar can be connected to a pilot.
	//
	// This setting balances out load across pilot instances, but adds some resource overhead.
	//
	// Examples: 300s, 30m, 1h
	KeepaliveMaxServerConnectionAge string `json:"keepaliveMaxServerConnectionAge,omitempty"`
	// Labels that are added to Pilot deployment and pods.
	//
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
	DeploymentLabels map[string]string `json:"deploymentLabels,omitempty"`
	PodLabels        map[string]string `json:"podLabels,omitempty"`
	// Configuration settings passed to Pilot as a ConfigMap.
	//
	// This controls whether the mesh config map, generated from values.yaml is generated.
	// If false, pilot wil use default values or user-supplied values, in that order of preference.
	ConfigMap bool `json:"configMap,omitempty"`
	// Controls whether Pilot is configured through the Mesh Control Protocol (MCP).
	//
	// If set to true, Pilot requires an MCP server (like Galley) to be installed.
	UseMCP bool `json:"useMCP,omitempty"`
	// Environment variables passed to the Pilot container.
	//
	// Examples:
	// env:
	//
	//	ENV_VAR_1: value1
	//	ENV_VAR_2: value2
	Env                map[string]string `json:"env,omitempty"`
	Affinity           *k8sv1.Affinity   `json:"affinity,omitempty"`
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`
	// ConfigSource describes a source of configuration data for networking
	// rules, and other Istio configuration artifacts. Multiple data sources
	// can be configured for a single control plane.
	ConfigSource            *PilotConfigSource `json:"configSource,omitempty"`
	JwksResolverExtraRootCA string             `json:"jwksResolverExtraRootCA,omitempty"`
	Plugins                 []string           `json:"plugins,omitempty"`
	Hub                     string             `json:"hub,omitempty"`
	// +kubebuilder:validation:XIntOrString
	Tag     *intstr.IntOrString `json:"tag,omitempty"`
	Variant string              `json:"variant,omitempty"`
	// The Container seccompProfile
	//
	// See: https://kubernetes.io/docs/tutorials/security/seccomp/
	SeccompProfile            map[string]string                `json:"seccompProfile,omitempty"`
	TopologySpreadConstraints []k8sv1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	ExtraContainerArgs []map[string]string `json:"extraContainerArgs,omitempty"`
	VolumeMounts       []k8sv1.VolumeMount `json:"volumeMounts,omitempty"`
	Volumes            []k8sv1.Volume      `json:"volumes,omitempty"`
	IPFamilies         []map[string]string `json:"ipFamilies,omitempty"`
	IPFamilyPolicy     string              `json:"ipFamilyPolicy,omitempty"`
}

// Controls legacy k8s ingress. Only one pilot profile should enable ingress support.
type PilotIngressConfig struct {
	// Sets the type ingress service for Pilot.
	//
	// If empty, node-port is assumed.
	//
	// Allowed values: node-port, istio-ingressgateway, ingress
	IngressService        string                `json:"ingressService,omitempty"`
	IngressControllerMode IngressControllerMode `json:"ingressControllerMode,omitempty"`
	// If mode is STRICT, this value must be set on "kubernetes.io/ingress.class" annotation to activate.
	IngressClass string `json:"ingressClass,omitempty"`
}

// Controls whether Istio policy is applied to Pilot.
type PilotPolicyConfig struct {
	// Controls whether Istio policy is applied to Pilot.
	Enabled bool `json:"enabled,omitempty"`
}

// Controls telemetry configuration
type TelemetryConfig struct {
	// Controls whether telemetry is exported for Pilot.
	Enabled bool `json:"enabled,omitempty"`
	// Use telemetry v2.
	V2 *TelemetryV2Config `json:"v2,omitempty"`
}

// Controls whether pilot will configure telemetry v2.
type TelemetryV2Config struct {
	// Controls whether pilot will configure telemetry v2.
	Enabled     bool                          `json:"enabled,omitempty"`
	Prometheus  *TelemetryV2PrometheusConfig  `json:"prometheus,omitempty"`
	Stackdriver *TelemetryV2StackDriverConfig `json:"stackdriver,omitempty"`
}

// Controls telemetry v2 prometheus settings.
type TelemetryV2PrometheusConfig struct {
	// Controls whether stats envoyfilter would be enabled or not.
	Enabled bool `json:"enabled,omitempty"`
}

// TelemetryV2StackDriverConfig controls telemetry v2 stackdriver settings.
type TelemetryV2StackDriverConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}

// PilotConfigSource describes information about a configuration store inside a
// mesh. A single control plane instance can interact with one or more data
// sources.
type PilotConfigSource struct {
	// Describes the source of configuration, if nothing is specified default is MCP.
	SubscribedResources []string `json:"subscribedResources,omitempty"`
}

// Configuration for a port.
type PortsConfig struct {
	// Port name.
	Name string `json:"name,omitempty"`
	// Port number.
	Port int32 `json:"port,omitempty"`
	// NodePort number.
	NodePort int32 `json:"nodePort,omitempty"`
	// Target port number.
	TargetPort int32 `json:"targetPort,omitempty"`
	// Protocol name.
	Protocol string `json:"protocol,omitempty"`
}

// Configuration for Proxy.
type ProxyConfig struct {
	AutoInject string `json:"autoInject,omitempty"`
	// Domain for the cluster, default: "cluster.local".
	//
	// K8s allows this to be customized, see https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/
	ClusterDomain string `json:"clusterDomain,omitempty"`
	// Per Component log level for proxy, applies to gateways and sidecars.
	//
	// If a component level is not set, then the global "logLevel" will be used. If left empty, "misc:error" is used.
	ComponentLogLevel string `json:"componentLogLevel,omitempty"`
	// Enables core dumps for newly injected sidecars.
	//
	// If set, newly injected sidecars will have core dumps enabled.
	EnableCoreDump bool `json:"enableCoreDump,omitempty"`
	// Specifies the Istio ingress ports not to capture.
	ExcludeInboundPorts string `json:"excludeInboundPorts,omitempty"`
	// Lists the excluded IP ranges of Istio egress traffic that the sidecar captures.
	ExcludeIPRanges string `json:"excludeIPRanges,omitempty"`
	// Image name or path for the proxy, default: "proxyv2".
	//
	// If registry or tag are not specified, global.hub and global.tag are used.
	//
	// Examples: my-proxy (uses global.hub/tag), docker.io/myrepo/my-proxy:v1.0.0
	Image string `json:"image,omitempty"`
	// Lists the IP ranges of Istio egress traffic that the sidecar captures.
	//
	// Example: "172.30.0.0/16,172.20.0.0/16"
	// This would only capture egress traffic on those two IP Ranges, all other outbound traffic would # be allowed by the sidecar."
	IncludeIPRanges string `json:"includeIPRanges,omitempty"`
	// Log level for proxy, applies to gateways and sidecars. If left empty, "warning" is used. Expected values are: trace\|debug\|info\|warning\|error\|critical\|off
	LogLevel string `json:"logLevel,omitempty"`
	// Enables privileged securityContext for the istio-proxy container.
	//
	// See https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
	Privileged bool `json:"privileged,omitempty"`
	// Sets the initial delay for readiness probes in seconds.
	ReadinessInitialDelaySeconds uint32 `json:"readinessInitialDelaySeconds,omitempty"`
	// Sets the interval between readiness probes in seconds.
	ReadinessPeriodSeconds uint32 `json:"readinessPeriodSeconds,omitempty"`
	// Sets the number of successive failed probes before indicating readiness failure.
	ReadinessFailureThreshold uint32        `json:"readinessFailureThreshold,omitempty"`
	StartupProbe              *StartupProbe `json:"startupProbe,omitempty"`
	// Default port used for the Pilot agent's health checks.
	StatusPort           uint32 `json:"statusPort,omitempty"`
	Tracer               Tracer `json:"tracer,omitempty"`
	ExcludeOutboundPorts string `json:"excludeOutboundPorts,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Lifecycle            map[string]string `json:"lifecycle,omitempty"`
	IncludeInboundPorts  string            `json:"includeInboundPorts,omitempty"`
	IncludeOutboundPorts string            `json:"includeOutboundPorts,omitempty"`
}

type StartupProbe struct {
	Enabled          bool   `json:"enabled,omitempty"`
	FailureThreshold uint32 `json:"failureThreshold,omitempty"`
}

// Configuration for proxy_init container which sets the pods' networking to intercept the inbound/outbound traffic.
type ProxyInitConfig struct {
	// Specifies the image for the proxy_init container.
	Image string `json:"image,omitempty"`
}

// Configuration for K8s resource requests.
type ResourcesRequestsConfig struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// Configuration for the SecretDiscoveryService instead of using K8S secrets to mount the certificates.
type SDSConfig struct{}

// Configuration for secret volume mounts.
//
// See https://kubernetes.io/docs/concepts/configuration/secret/#using-secrets.
type SecretVolume struct {
	MountPath  string `json:"mountPath,omitempty"`
	Name       string `json:"name,omitempty"`
	SecretName string `json:"secretName,omitempty"`
}

// SidecarInjectorConfig is described in istio.io documentation.
type SidecarInjectorConfig struct {
	// Enables sidecar auto-injection in namespaces by default.
	EnableNamespacesByDefault bool `json:"enableNamespacesByDefault,omitempty"`
	// Setting this to `IfNeeded` will result in the sidecar injector being run again if additional mutations occur. Default: Never
	ReinvocationPolicy string `json:"reinvocationPolicy,omitempty"`
	// Instructs Istio to not inject the sidecar on those pods, based on labels that are present in those pods.
	//
	// Annotations in the pods have higher precedence than the label selectors.
	// Order of evaluation: Pod Annotations → NeverInjectSelector → AlwaysInjectSelector → Default Policy.
	// See https://istio.io/docs/setup/kubernetes/additional-setup/sidecar-injection/#more-control-adding-exceptions
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	NeverInjectSelector []map[string]string `json:"neverInjectSelector,omitempty"`
	// See NeverInjectSelector.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	AlwaysInjectSelector []map[string]string `json:"alwaysInjectSelector,omitempty"`
	// If true, webhook or istioctl injector will rewrite PodSpec for liveness health check to redirect request to sidecar. This makes liveness check work even when mTLS is enabled.
	RewriteAppHTTPProbe bool `json:"rewriteAppHTTPProbe,omitempty"`
	// injectedAnnotations are additional annotations that will be added to the pod spec after injection
	// This is primarily to support PSP annotations.
	InjectedAnnotations map[string]string `json:"injectedAnnotations,omitempty"`
	// Enable objectSelector to filter out pods with no need for sidecar before calling istio-sidecar-injector.
	ObjectSelector map[string]string `json:"objectSelector,omitempty"`
	// Configure the injection url for sidecar injector webhook
	InjectionURL string `json:"injectionURL,omitempty"`
	// Templates defines a set of custom injection templates that can be used. For example, defining:
	//
	// templates:
	//
	//	hello: |
	//	  metadata:
	//	    labels:
	//	      hello: world
	//
	// Then starting a pod with the `inject.istio.io/templates: hello` annotation, will result in the pod
	// being injected with the hello=world labels.
	// This is intended for advanced configuration only; most users should use the built in template
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Templates map[string]string `json:"templates,omitempty"`
	// defaultTemplates: ["sidecar", "hello"]
	DefaultTemplates []string `json:"defaultTemplates,omitempty"`
}

// Configuration for each of the supported tracers.
type TracerConfig struct {
	// Configuration for the datadog tracing service.
	Datadog *TracerDatadogConfig `json:"datadog,omitempty"`
	// Configuration for the lightstep tracing service.
	Lightstep *TracerLightStepConfig `json:"lightstep,omitempty"`
	// Configuration for the zipkin tracing service.
	Zipkin *TracerZipkinConfig `json:"zipkin,omitempty"`
	// Configuration for the stackdriver tracing service.
	Stackdriver *TracerStackdriverConfig `json:"stackdriver,omitempty"`
}

// Configuration for the datadog tracing service.
type TracerDatadogConfig struct {
	// Address in host:port format for reporting trace data to the Datadog agent.
	Address string `json:"address,omitempty"`
}

// Configuration for the lightstep tracing service.
type TracerLightStepConfig struct {
	// Sets the lightstep satellite pool address in host:port format for reporting trace data.
	Address string `json:"address,omitempty"`
	// Sets the lightstep access token.
	AccessToken string `json:"accessToken,omitempty"`
}

// Configuration for the zipkin tracing service.
type TracerZipkinConfig struct {
	// Address of zipkin instance in host:port format for reporting trace data.
	//
	// Example: <zipkin-collector-service>.<zipkin-collector-namespace>:941
	Address string `json:"address,omitempty"`
}

// Configuration for the stackdriver tracing service.
type TracerStackdriverConfig struct {
	// enables trace output to stdout.
	Debug bool `json:"debug,omitempty"`
	// The global default max number of attributes per span.
	MaxNumberOfAttributes uint32 `json:"maxNumberOfAttributes,omitempty"`
	// The global default max number of annotation events per span.
	MaxNumberOfAnnotations uint32 `json:"maxNumberOfAnnotations,omitempty"`
	// The global default max number of message events per span.
	MaxNumberOfMessageEvents uint32 `json:"maxNumberOfMessageEvents,omitempty"`
}

// Configuration for base component
type BaseConfig struct {
	// For Helm2 use, adds the CRDs to templates.
	EnableCRDTemplates bool `json:"enableCRDTemplates,omitempty"`
	// URL to use for validating webhook.
	ValidationURL string `json:"validationURL,omitempty"`
	// For istioctl usage to disable istio config crds in base
	EnableIstioConfigCRDs bool `json:"enableIstioConfigCRDs,omitempty"`
	ValidateGateway       bool `json:"validateGateway,omitempty"`
}

type IstiodRemoteConfig struct {
	// URL to use for sidecar injector webhook.
	InjectionURL string `json:"injectionURL,omitempty"`
	// Path to use for the sidecar injector webhook service.
	InjectionPath string `json:"injectionPath,omitempty"`
}

type Values struct {
	Cni    *CNIConfig    `json:"cni,omitempty"`
	Global *GlobalConfig `json:"global,omitempty"`
	Pilot  *PilotConfig  `json:"pilot,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Ztunnel map[string]string `json:"ztunnel,omitempty"`
	// Controls whether telemetry is exported for Pilot.
	Telemetry              *TelemetryConfig       `json:"telemetry,omitempty"`
	SidecarInjectorWebhook *SidecarInjectorConfig `json:"sidecarInjectorWebhook,omitempty"`
	IstioCni               *CNIConfig             `json:"istio_cni,omitempty"`
	Revision               string                 `json:"revision,omitempty"`
	OwnerName              string                 `json:"ownerName,omitempty"`
	MeshConfig             *MeshConfig            `json:"meshConfig,omitempty"`
	Base                   *BaseConfig            `json:"base,omitempty"`
	IstiodRemote           *IstiodRemoteConfig    `json:"istiodRemote,omitempty"`
	RevisionTags           []string               `json:"revisionTags,omitempty"`
	DefaultRevision        string                 `json:"defaultRevision,omitempty"`
}

// ZeroVPNConfig enables cross-cluster access using SNI matching.
type ZeroVPNConfig struct {
	// Controls whether ZeroVPN is enabled.
	Enabled bool   `json:"enabled,omitempty"`
	Suffix  string `json:"suffix,omitempty"`
}

func (v *Values) ToHelmValues() helm.HelmValues {
	var obj helm.HelmValues
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, &obj); err != nil {
		panic(err)
	}
	return obj
}
