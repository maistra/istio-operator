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

// Configuration affecting the service mesh as a whole.

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Resource describes the source of configuration
type Resource int32

type AccessLogEncoding int32

// Default Policy for upgrading http1.1 connections to http2.
type H2UpgradePolicy int32

type OutboundTrafficPolicyMode int32

// TraceContext selects the context propagation headers used for
// distributed tracing.
type ExtensionProviderOpenCensusAgentTracingProviderTraceContext int32

type ProxyPathNormalizationNormalizationType int32

// TLS protocol versions.
type TLSConfigTLSProtocol int32

// MeshConfig defines mesh-wide settings for the Istio service mesh.
type MeshConfig struct {
	// Port on which Envoy should listen for all outbound traffic to other services.
	// Default port is 15001.
	ProxyListenPort int32 `json:"proxyListenPort,omitempty"`
	// Port on which Envoy should listen for all inbound traffic to the pod/vm will be captured to.
	// Default port is 15006.
	ProxyInboundListenPort int32 `json:"proxyInboundListenPort,omitempty"`
	// Port on which Envoy should listen for HTTP PROXY requests if set.
	ProxyHTTPPort int32 `json:"proxyHttpPort,omitempty"`
	// Connection timeout used by Envoy. (MUST BE >=1ms)
	// Default timeout is 10s.
	ConnectTimeout *v1.Duration `json:"connectTimeout,omitempty"`
	// If set then set `SO_KEEPALIVE` on the socket to enable TCP Keepalives.
	TCPKeepalive *ConnectionPoolSettingsTCPSettingsTCPKeepalive `json:"tcpKeepalive,omitempty"`
	// Class of ingress resources to be processed by Istio ingress
	// controller. This corresponds to the value of
	// `kubernetes.io/ingress.class` annotation.
	IngressClass string `json:"ingressClass,omitempty"`
	// Name of the Kubernetes service used for the istio ingress controller.
	// If no ingress controller is specified, the default value `istio-ingressgateway` is used.
	IngressService string `json:"ingressService,omitempty"`
	// Defines whether to use Istio ingress controller for annotated or all ingress resources.
	// Default mode is `STRICT`.
	IngressControllerMode IngressControllerMode `json:"ingressControllerMode,omitempty"`
	// Defines which gateway deployment to use as the Ingress controller. This field corresponds to
	// the Gateway.selector field, and will be set as `istio: INGRESS_SELECTOR`.
	// By default, `ingressgateway` is used, which will select the default IngressGateway as it has the
	// `istio: ingressgateway` labels.
	// It is recommended that this is the same value as ingress_service.
	IngressSelector string `json:"ingressSelector,omitempty"`
	// Flag to control generation of trace spans and request IDs.
	// Requires a trace span collector defined in the proxy configuration.
	EnableTracing bool `json:"enableTracing,omitempty"`
	// File address for the proxy access log (e.g. /dev/stdout).
	// Empty value disables access logging.
	AccessLogFile string `json:"accessLogFile,omitempty"`
	// Format for the proxy access log
	// Empty value results in proxy's default access log format
	AccessLogFormat string `json:"accessLogFormat,omitempty"`
	// Encoding for the proxy access log (`TEXT` or `JSON`).
	// Default value is `TEXT`.
	AccessLogEncoding AccessLogEncoding `json:"accessLogEncoding,omitempty"`
	// This flag enables Envoy's gRPC Access Log Service.
	// See [Access Log Service](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/grpc/v3/als.proto)
	// for details about Envoy's gRPC Access Log Service API.
	// Default value is `false`.
	EnableEnvoyAccessLogService bool `json:"enableEnvoyAccessLogService,omitempty"`
	// This flag disables Envoy Listener logs.
	// See [Listener Access Log](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/listener/v3/listener.proto#envoy-v3-api-field-config-listener-v3-listener-access-log)
	// Istio Enables Envoy's listener access logs on "NoRoute" response flag.
	// Default value is `false`.
	DisableEnvoyListenerLog bool `json:"disableEnvoyListenerLog,omitempty"`
	// Default proxy config used by gateway and sidecars.
	// In case of Kubernetes, the proxy config is applied once during the injection process,
	// and remain constant for the duration of the pod. The rest of the mesh config can be changed
	// at runtime and config gets distributed dynamically.
	// On Kubernetes, this can be overridden on individual pods with the `proxy.istio.io/config` annotation.
	DefaultConfig *ProxyConfig `json:"defaultConfig,omitempty"`
	// Set the default behavior of the sidecar for handling outbound
	// traffic from the application.  If your application uses one or
	// more external services that are not known apriori, setting the
	// policy to `ALLOW_ANY` will cause the sidecars to route any unknown
	// traffic originating from the application to its requested
	// destination. Users are strongly encouraged to use ServiceEntries
	// to explicitly declare any external dependencies, instead of using
	// `ALLOW_ANY`, so that traffic to these services can be
	// monitored. Can be overridden at a Sidecar level by setting the
	// `OutboundTrafficPolicy` in the [Sidecar
	// API](https://istio.io/docs/reference/config/networking/sidecar/#OutboundTrafficPolicy).
	// Default mode is `ALLOW_ANY` which means outbound traffic to unknown destinations will be allowed.
	OutboundTrafficPolicy *OutboundTrafficPolicy `json:"outboundTrafficPolicy,omitempty"`
	// ConfigSource describes a source of configuration data for networking
	// rules, and other Istio configuration artifacts. Multiple data sources
	// can be configured for a single control plane.
	ConfigSources []*ConfigSource `json:"configSources,omitempty"`
	// This flag is used to enable mutual `TLS` automatically for service to service communication
	// within the mesh, default true.
	// If set to true, and a given service does not have a corresponding `DestinationRule` configured,
	// or its `DestinationRule` does not have ClientTLSSettings specified, Istio configures client side
	// TLS configuration appropriately. More specifically,
	// If the upstream authentication policy is in `STRICT` mode, use Istio provisioned certificate
	// for mutual `TLS` to connect to upstream.
	// If upstream service is in plain text mode, use plain text.
	// If the upstream authentication policy is in PERMISSIVE mode, Istio configures clients to use
	// mutual `TLS` when server sides are capable of accepting mutual `TLS` traffic.
	// If service `DestinationRule` exists and has `ClientTLSSettings` specified, that is always used instead.
	EnableAutoMtls bool `json:"enableAutoMtls,omitempty"`
	// The trust domain corresponds to the trust root of a system.
	// Refer to [SPIFFE-ID](https://github.com/spiffe/spiffe/blob/master/standards/SPIFFE-ID.md#21-trust-domain)
	TrustDomain string `json:"trustDomain,omitempty"`
	// The trust domain aliases represent the aliases of `trust_domain`.
	// For example, if we have
	// ```yaml
	// trustDomain: td1
	// trustDomainAliases: ["td2", "td3"]
	// ```
	// Any service with the identity `td1/ns/foo/sa/a-service-account`, `td2/ns/foo/sa/a-service-account`,
	// or `td3/ns/foo/sa/a-service-account` will be treated the same in the Istio mesh.
	TrustDomainAliases []string `json:"trustDomainAliases,omitempty"`
	// The extra root certificates for workload-to-workload communication.
	// The plugin certificates (the 'cacerts' secret) or self-signed certificates (the 'istio-ca-secret' secret)
	// are automatically added by Istiod.
	// The CA certificate that signs the workload certificates is automatically added by Istio Agent.
	// +kubebuilder:validation:MaxItems=100
	CaCertificates []*CertificateData `json:"caCertificates,omitempty"`
	// The default value for the ServiceEntry.export_to field and services
	// imported through container registry integrations, e.g. this applies to
	// Kubernetes Service resources. The value is a list of namespace names and
	// reserved namespace aliases. The allowed namespace aliases are:
	// ```
	// * - All Namespaces
	// . - Current Namespace
	// ~ - No Namespace
	// ```
	// If not set the system will use "*" as the default value which implies that
	// services are exported to all namespaces.
	//
	// `All namespaces` is a reasonable default for implementations that don't
	// need to restrict access or visibility of services across namespace
	// boundaries. If that requirement is present it is generally good practice to
	// make the default `Current namespace` so that services are only visible
	// within their own namespaces by default. Operators can then expand the
	// visibility of services to other namespaces as needed. Use of `No Namespace`
	// is expected to be rare but can have utility for deployments where
	// dependency management needs to be precise even within the scope of a single
	// namespace.
	//
	// For further discussion see the reference documentation for `ServiceEntry`,
	// `Sidecar`, and `Gateway`.
	DefaultServiceExportTo []string `json:"defaultServiceExportTo,omitempty"`
	// The default value for the VirtualService.export_to field. Has the same
	// syntax as `default_service_export_to`.
	//
	// If not set the system will use "*" as the default value which implies that
	// virtual services are exported to all namespaces
	DefaultVirtualServiceExportTo []string `json:"defaultVirtualServiceExportTo,omitempty"`
	// The default value for the `DestinationRule.export_to` field. Has the same
	// syntax as `default_service_export_to`.
	//
	// If not set the system will use "*" as the default value which implies that
	// destination rules are exported to all namespaces
	DefaultDestinationRuleExportTo []string `json:"defaultDestinationRuleExportTo,omitempty"`
	// The namespace to treat as the administrative root namespace for
	// Istio configuration. When processing a leaf namespace Istio will search for
	// declarations in that namespace first and if none are found it will
	// search in the root namespace. Any matching declaration found in the root
	// namespace is processed as if it were declared in the leaf namespace.
	//
	// The precise semantics of this processing are documented on each resource
	// type.
	RootNamespace string `json:"rootNamespace,omitempty"`
	// Locality based load balancing distribution or failover settings.
	// If unspecified, locality based load balancing will be enabled by default.
	// However, this requires outlierDetection to actually take effect for a particular
	// service, see https://istio.io/latest/docs/tasks/traffic-management/locality-load-balancing/failover/
	LocalityLbSetting *LocalityLoadBalancerSetting `json:"localityLbSetting,omitempty"`
	// Configures DNS refresh rate for Envoy clusters of type `STRICT_DNS`
	// Default refresh rate is `60s`.
	DNSRefreshRate *v1.Duration `json:"dnsRefreshRate,omitempty"`
	// Specify if http1.1 connections should be upgraded to http2 by default.
	// if sidecar is installed on all pods in the mesh, then this should be set to `UPGRADE`.
	// If one or more services or namespaces do not have sidecar(s), then this should be set to `DO_NOT_UPGRADE`.
	// It can be enabled by destination using the `destinationRule.trafficPolicy.connectionPool.http.h2UpgradePolicy` override.
	H2UpgradePolicy H2UpgradePolicy `json:"h2UpgradePolicy,omitempty"`
	// Name to be used while emitting statistics for inbound clusters. The same pattern is used while computing stat prefix for
	// network filters like TCP and Redis.
	// By default, Istio emits statistics with the pattern `inbound|<port>|<port-name>|<service-FQDN>`.
	// For example `inbound|7443|grpc-reviews|reviews.prod.svc.cluster.local`. This can be used to override that pattern.
	//
	// A Pattern can be composed of various pre-defined variables. The following variables are supported.
	//
	// - `%SERVICE%` - Will be substituted with name of the service.
	// - `%SERVICE_FQDN%` - Will be substituted with FQDN of the service.
	// - `%SERVICE_PORT%` - Will be substituted with port of the service.
	// - `%TARGET_PORT%`  - Will be substituted with the target port of the service.
	// - `%SERVICE_PORT_NAME%` - Will be substituted with port name of the service.
	//
	// Following are some examples of supported patterns for reviews:
	//
	// - `%SERVICE_FQDN%_%SERVICE_PORT%` will use reviews.prod.svc.cluster.local_7443 as the stats name.
	// - `%SERVICE%` will use reviews.prod as the stats name.
	InboundClusterStatName string `json:"inboundClusterStatName,omitempty"`
	// Name to be used while emitting statistics for outbound clusters. The same pattern is used while computing stat prefix for
	// network filters like TCP and Redis.
	// By default, Istio emits statistics with the pattern `outbound|<port>|<subsetname>|<service-FQDN>`.
	// For example `outbound|8080|v2|reviews.prod.svc.cluster.local`. This can be used to override that pattern.
	//
	// A Pattern can be composed of various pre-defined variables. The following variables are supported.
	//
	// - `%SERVICE%` - Will be substituted with name of the service.
	// - `%SERVICE_FQDN%` - Will be substituted with FQDN of the service.
	// - `%SERVICE_PORT%` - Will be substituted with port of the service.
	// - `%SERVICE_PORT_NAME%` - Will be substituted with port name of the service.
	// - `%SUBSET_NAME%` - Will be substituted with subset.
	//
	// Following are some examples of supported patterns for reviews:
	//
	// - `%SERVICE_FQDN%_%SERVICE_PORT%` will use `reviews.prod.svc.cluster.local_7443` as the stats name.
	// - `%SERVICE%` will use reviews.prod as the stats name.
	OutboundClusterStatName string `json:"outboundClusterStatName,omitempty"`
	// If enabled, Istio agent will merge metrics exposed by the application with metrics from Envoy
	// and Istio agent. The sidecar injection will replace `prometheus.io` annotations present on the pod
	// and redirect them towards Istio agent, which will then merge metrics of from the application with Istio metrics.
	// This relies on the annotations `prometheus.io/scrape`, `prometheus.io/port`, and
	// `prometheus.io/path` annotations.
	// If you are running a separately managed Envoy with an Istio sidecar, this may cause issues, as the metrics will collide.
	// In this case, it is recommended to disable aggregation on that deployment with the
	// `prometheus.istio.io/merge-metrics: "false"` annotation.
	// If not specified, this will be enabled by default.
	EnablePrometheusMerge bool `json:"enablePrometheusMerge,omitempty"`
	// Defines a list of extension providers that extend Istio's functionality. For example, the AuthorizationPolicy
	// can be used with an extension provider to delegate the authorization decision to a custom authorization system.
	// +kubebuilder:validation:MaxItems=100
	ExtensionProviders []*ExtensionProvider `json:"extensionProviders,omitempty"`
	// Specifies extension providers to use by default in Istio configuration resources.
	DefaultProviders *DefaultProviders `json:"defaultProviders,omitempty"`
	// A list of Kubernetes selectors that specify the set of namespaces that Istio considers when
	// computing configuration updates for sidecars. This can be used to reduce Istio's computational load
	// by limiting the number of entities (including services, pods, and endpoints) that are watched and processed.
	// If omitted, Istio will use the default behavior of processing all namespaces in the cluster.
	// Elements in the list are disjunctive (OR semantics), i.e. a namespace will be included if it matches any selector.
	// The following example selects any namespace that matches either below:
	// 1. The namespace has both of these labels: `env: prod` and `region: us-east1`
	// 2. The namespace has label `app` equal to `cassandra` or `spark`.
	// ```yaml
	// discoverySelectors:
	//   - matchLabels:
	//     env: prod
	//     region: us-east1
	//   - matchExpressions:
	//   - key: app
	//     operator: In
	//     values:
	//   - cassandra
	//   - spark
	//
	// ```
	// Refer to the [Kubernetes selector docs](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors)
	// for additional detail on selector semantics.
	DiscoverySelectors []*v1.LabelSelector `json:"discoverySelectors,omitempty"`
	// ProxyPathNormalization configures how URL paths in incoming and outgoing HTTP requests are
	// normalized by the sidecars and gateways.
	// The normalized paths will be used in all aspects through the requests' lifetime on the
	// sidecars and gateways, which includes routing decisions in outbound direction (client proxy),
	// authorization policy match and enforcement in inbound direction (server proxy), and the URL
	// path proxied to the upstream service.
	// If not set, the NormalizationType.DEFAULT configuration will be used.
	PathNormalization *ProxyPathNormalization `json:"pathNormalization,omitempty"`
	// Configure the default HTTP retry policy.
	// The default number of retry attempts is set at 2 for these errors:
	//
	//	"connect-failure,refused-stream,unavailable,cancelled,retriable-status-codes".
	//
	// Setting the number of attempts to 0 disables retry policy globally.
	// This setting can be overridden on a per-host basis using the Virtual Service
	// API.
	// All settings in the retry policy except `perTryTimeout` can currently be
	// configured globally via this field.
	DefaultHTTPRetryPolicy *HTTPRetry `json:"defaultHttpRetryPolicy,omitempty"`
	// The below configuration parameters can be used to specify TLSConfig for mesh traffic.
	// For example, a user could enable min TLS version for ISTIO_MUTUAL traffic and specify a curve for non ISTIO_MUTUAL traffic like below:
	// ```yaml
	// meshConfig:
	//
	//	meshMTLS:
	//	  minProtocolVersion: TLSV1_3
	//	tlsDefaults:
	//	  Note: applicable only for non ISTIO_MUTUAL scenarios
	//	  ecdhCurves:
	//	    - P-256
	//	    - P-512
	//
	// ```
	// Configuration of mTLS for traffic between workloads with ISTIO_MUTUAL TLS traffic.
	//
	// Note: Mesh mTLS does not respect ECDH curves.
	MeshMTLS *TLSConfig `json:"meshMTLS,omitempty"`
	// Configuration of TLS for all traffic except for ISTIO_MUTUAL mode.
	// Currently, this supports configuration of ecdh_curves and cipher_suites only.
	// For ISTIO_MUTUAL TLS settings, use meshMTLS configuration.
	TLSDefaults *TLSConfig `json:"tlsDefaults,omitempty"`
}

// ConfigSource describes information about a configuration store inside a
// mesh. A single control plane instance can interact with one or more data
// sources.
type ConfigSource struct {
	// Address of the server implementing the Istio Mesh Configuration
	// protocol (MCP). Can be IP address or a fully qualified DNS name.
	// Use xds:// to specify a grpc-based xds backend, k8s:// to specify a k8s controller or
	// fs:/// to specify a file-based backend with absolute path to the directory.
	Address string `json:"address,omitempty"`
	// Use the tlsSettings to specify the tls mode to use. If the MCP server
	// uses Istio mutual TLS and shares the root CA with Pilot, specify the TLS
	// mode as `ISTIO_MUTUAL`.
	TLSSettings *ClientTLSSettings `json:"tlsSettings,omitempty"`
	// Describes the source of configuration, if nothing is specified default is MCP
	SubscribedResources []Resource `json:"subscribedResources,omitempty"`
}

type OutboundTrafficPolicy struct {
	Modetype OutboundTrafficPolicyMode `json:"mode,omitempty"`
}

// +kubebuilder:validation:XValidation:message="At most one of [pem spiffeBundleUrl] should be set",rule="[has(self.pem), has(self.spiffeBundleUrl)].exists_one(x,x)"
type CertificateData struct {
	// The PEM data of the certificate.
	Pem string `json:"pem,omitempty"` // oneof
	// The SPIFFE bundle endpoint URL that complies to:
	// https://github.com/spiffe/spiffe/blob/master/standards/SPIFFE_Trust_Domain_and_Bundle.md#the-spiffe-trust-domain-and-bundle
	// The endpoint should support authentication based on Web PKI:
	// https://github.com/spiffe/spiffe/blob/master/standards/SPIFFE_Trust_Domain_and_Bundle.md#521-web-pki
	// The certificate is retrieved from the endpoint.
	SpiffeBundleURL string `json:"spiffeBundleUrl,omitempty"` // oneof
	// Optional. Specify the kubernetes signers (External CA) that use this trustAnchor
	// when Istiod is acting as RA(registration authority)
	// If set, they are used for these signers. Otherwise, this trustAnchor is used for all signers.
	CertSigners []string `json:"certSigners,omitempty"`
	// Optional. Specify the list of trust domains to which this trustAnchor data belongs.
	// If set, they are used for these trust domains. Otherwise, this trustAnchor is used for default trust domain
	// and its aliases.
	// Note that we can have multiple trustAnchor data for a same trust_domain.
	// In that case, trustAnchors with a same trust domain will be merged and used together to verify peer certificates.
	// If neither cert_signers nor trust_domains is set, this trustAnchor is used for all trust domains and all signers.
	// If only trust_domains is set, this trustAnchor is used for these trust_domains and all signers.
	// If only cert_signers is set, this trustAnchor is used for these cert_signers and all trust domains.
	// If both cert_signers and trust_domains is set, this trustAnchor is only used for these signers and trust domains.
	TrustDomains []string `json:"trustDomains,omitempty"`
}

type CA struct {
	// REQUIRED. Address of the CA server implementing the Istio CA gRPC API.
	// Can be IP address or a fully qualified DNS name with port
	// Eg: custom-ca.default.svc.cluster.local:8932, 192.168.23.2:9000
	// +kubebuilder:validation:Required
	Address string `json:"address"`
	// Use the tls_settings to specify the tls mode to use.
	// Regarding tls_settings:
	// - DISABLE MODE is legitimate for the case Istiod is making the request via an Envoy sidecar.
	// DISABLE MODE can also be used for testing
	// - TLS MUTUAL MODE be on by default. If the CA certificates
	// (cert bundle to verify the CA server's certificate) is omitted, Istiod will
	// use the system root certs to verify the CA server's certificate.
	TLSSettings *ClientTLSSettings `json:"tlsSettings,omitempty"`
	// timeout for forward CSR requests from Istiod to External CA
	// Default: 10s
	RequestTimeout *v1.Duration `json:"requestTimeout,omitempty"`
	// Use istiod_side to specify CA Server integrate to Istiod side or Agent side
	// Default: true
	IstiodSide bool `json:"istiodSide,omitempty"`
}

// +kubebuilder:validation:XValidation:message="At most one of [envoyExtAuthzHttp envoyExtAuthzGrpc zipkin datadog stackdriver skywalking opentelemetry prometheus envoyFileAccessLog envoyHttpAls envoyTcpAls envoyOtelAls] should be set",rule="[has(self.envoyExtAuthzHttp), has(self.envoyExtAuthzGrpc), has(self.zipkin), has(self.datadog), has(self.stackdriver), has(self.skywalking), has(self.opentelemetry), has(self.prometheus), has(self.envoyFileAccessLog), has(self.envoyHttpAls), has(self.envoyTcpAls), has(self.envoyOtelAls)].exists_one(x,x)"
// +kubebuilder:validation:MaxProperties=2
type ExtensionProvider struct {
	// REQUIRED. A unique name identifying the extension provider.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Configures an external authorizer that implements the Envoy ext_authz filter authorization check service using the HTTP API.
	EnvoyExtAuthzHTTP *ExtensionProviderEnvoyExternalAuthorizationHTTPProvider `json:"envoyExtAuthzHttp,omitempty"` // oneof
	// Configures an external authorizer that implements the Envoy ext_authz filter authorization check service using the gRPC API.
	EnvoyExtAuthzGrpc *ExtensionProviderEnvoyExternalAuthorizationGrpcProvider `json:"envoyExtAuthzGrpc,omitempty"` // oneof
	// Configures a tracing provider that uses the Zipkin API.
	Zipkin *ExtensionProviderZipkinTracingProvider `json:"zipkin,omitempty"` // oneof
	// Configures a Datadog tracing provider.
	Datadog *ExtensionProviderDatadogTracingProvider `json:"datadog,omitempty"` // oneof
	// Configures a Stackdriver provider.
	Stackdriver *ExtensionProviderStackdriverProvider `json:"stackdriver,omitempty"` // oneof
	// Configures a Apache SkyWalking provider.
	Skywalking *ExtensionProviderSkyWalkingTracingProvider `json:"skywalking,omitempty"` // oneof
	// Configures an OpenTelemetry tracing provider.
	Opentelemetry *ExtensionProviderOpenTelemetryTracingProvider `json:"opentelemetry,omitempty"` // oneof
	// Configures a Prometheus metrics provider.
	Prometheus *ExtensionProviderPrometheusMetricsProvider `json:"prometheus,omitempty"` // oneof
	// Configures an Envoy File Access Log provider.
	EnvoyFileAccessLog *ExtensionProviderEnvoyFileAccessLogProvider `json:"envoyFileAccessLog,omitempty"` // oneof
	// Configures an Envoy Access Logging Service provider for HTTP traffic.
	EnvoyHTTPAls *ExtensionProviderEnvoyHTTPGrpcV3LogProvider `json:"envoyHttpAls,omitempty"` // oneof
	// Configures an Envoy Access Logging Service provider for TCP traffic.
	EnvoyTCPAls *ExtensionProviderEnvoyTCPGrpcV3LogProvider `json:"envoyTcpAls,omitempty"` // oneof
	// Configures an Envoy Open Telemetry Access Logging Service provider.
	EnvoyOtelAls *ExtensionProviderEnvoyOpenTelemetryLogProvider `json:"envoyOtelAls,omitempty"` // oneof
}

// Holds the name references to the providers that will be used by default
// in other Istio configuration resources if the provider is not specified.
//
// These names must match a provider defined in `extension_providers` that is
// one of the supported tracing providers.
type DefaultProviders struct {
	// Name of the default provider(s) for tracing.
	Tracing []string `json:"tracing,omitempty"`
	// Name of the default provider(s) for metrics.
	Metrics []string `json:"metrics,omitempty"`
	// Name of the default provider(s) for access logging.
	AccessLogging []string `json:"accessLogging,omitempty"`
}

type ProxyPathNormalization struct {
	Normalization ProxyPathNormalizationNormalizationType `json:"normalization,omitempty"`
}

type TLSConfig struct {
	// Optional: the minimum TLS protocol version. The default minimum
	// TLS version will be TLS 1.2. As servers may not be Envoy and be
	// set to TLS 1.2 (e.g., workloads using mTLS without sidecars), the
	// minimum TLS version for clients may also be TLS 1.2.
	// In the current Istio implementation, the maximum TLS protocol version
	// is TLS 1.3.
	MinProtocolVersion TLSConfigTLSProtocol `json:"minProtocolVersion,omitempty"`
	// Optional: If specified, the TLS connection will only support the specified ECDH curves for the DH key exchange.
	// If not specified, the default curves enforced by Envoy will be used. For details about the default curves, refer to
	// [Ecdh Curves](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/transport_sockets/tls/v3/common.proto).
	EcdhCurves []string `json:"ecdhCurves,omitempty"`
	// Optional: If specified, the TLS connection will only support the specified cipher list when negotiating TLS 1.0-1.2.
	// If not specified, the following cipher suites will be used:
	// ```
	// ECDHE-ECDSA-AES256-GCM-SHA384
	// ECDHE-RSA-AES256-GCM-SHA384
	// ECDHE-ECDSA-AES128-GCM-SHA256
	// ECDHE-RSA-AES128-GCM-SHA256
	// AES256-GCM-SHA384
	// AES128-GCM-SHA256
	// ```
	CipherSuites []string `json:"cipherSuites,omitempty"`
}

// Settings for the selected services.
type ServiceSettingsSettings struct {
	// If true, specifies that the client and service endpoints must reside in the same cluster.
	// By default, in multi-cluster deployments, the Istio control plane assumes all service
	// endpoints to be reachable from any client in any of the clusters which are part of the
	// mesh. This configuration option limits the set of service endpoints visible to a client
	// to be cluster scoped.
	//
	// There are some common scenarios when this can be useful:
	//
	//   - A service (or group of services) is inherently local to the cluster and has local storage
	//     for that cluster. For example, the kube-system namespace (e.g. the Kube API Server).
	//   - A mesh administrator wants to slowly migrate services to Istio. They might start by first
	//     having services cluster-local and then slowly transition them to mesh-wide. They could do
	//     this service-by-service (e.g. mysvc.myns.svc.cluster.local) or as a group
	//     (e.g. *.myns.svc.cluster.local).
	//
	// By default Istio will consider kubernetes.default.svc (i.e. the API Server) as well as all
	// services in the kube-system namespace to be cluster-local, unless explicitly overridden here.
	ClusterLocal bool `json:"clusterLocal,omitempty"`
}

type ExtensionProviderEnvoyExternalAuthorizationRequestBody struct {
	// Sets the maximum size of a message body that the ext-authz filter will hold in memory.
	// If max_request_bytes is reached, and allow_partial_message is false, Envoy will return a 413 (Payload Too Large).
	// Otherwise the request will be sent to the provider with a partial message.
	// Note that this setting will have precedence over the fail_open field, the 413 will be returned even when the
	// fail_open is set to true.
	MaxRequestBytes uint32 `json:"maxRequestBytes,omitempty"`
	// When this field is true, ext-authz filter will buffer the message until max_request_bytes is reached.
	// The authorization request will be dispatched and no 413 HTTP error will be returned by the filter.
	// A "x-envoy-auth-partial-body: false|true" metadata header will be added to the authorization request message
	// indicating if the body data is partial.
	AllowPartialMessage bool `json:"allowPartialMessage,omitempty"`
	// If true, the body sent to the external authorization service in the gRPC authorization request is set with raw bytes
	// in the [raw_body field](https://github.com/envoyproxy/envoy/blame/cffb095d59d7935abda12b9509bcd136808367bb/api/envoy/service/auth/v3/attribute_context.proto#L153).
	// Otherwise, it will be filled with UTF-8 string in the [body field](https://github.com/envoyproxy/envoy/blame/cffb095d59d7935abda12b9509bcd136808367bb/api/envoy/service/auth/v3/attribute_context.proto#L147).
	// This field only works with the envoy_ext_authz_grpc provider and has no effect for the envoy_ext_authz_http provider.
	PackAsBytes bool `json:"packAsBytes,omitempty"`
}

type ExtensionProviderEnvoyExternalAuthorizationHTTPProvider struct {
	// REQUIRED. Specifies the service that implements the Envoy ext_authz HTTP authorization service.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "my-ext-authz.foo.svc.cluster.local" or "bar/my-ext-authz.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// The maximum duration that the proxy will wait for a response from the provider (default timeout: 600s).
	// When this timeout condition is met, the proxy marks the communication to the authorization service as failure.
	// In this situation, the response sent back to the client will depend on the configured `fail_open` field.
	Timeout *v1.Duration `json:"timeout,omitempty"`
	// Sets a prefix to the value of authorization request header *Path*.
	// For example, setting this to "/check" for an original user request at path "/admin" will cause the
	// authorization check request to be sent to the authorization service at the path "/check/admin" instead of "/admin".
	PathPrefix string `json:"pathPrefix,omitempty"`
	// If true, the user request will be allowed even if the communication with the authorization service has failed,
	// or if the authorization service has returned a HTTP 5xx error.
	// Default is false and the request will be rejected with "Forbidden" response.
	FailOpen bool `json:"failOpen,omitempty"`
	// Sets the HTTP status that is returned to the client when there is a network error to the authorization service.
	// The default status is "403" (HTTP Forbidden).
	StatusOnError string `json:"statusOnError,omitempty"`
	// List of client request headers that should be included in the authorization request sent to the authorization service.
	// Note that in addition to the headers specified here following headers are included by default:
	// 1. *Host*, *Method*, *Path* and *Content-Length* are automatically sent.
	// 2. *Content-Length* will be set to 0 and the request will not have a message body. However, the authorization
	// request can include the buffered client request body (controlled by include_request_body_in_check setting),
	// consequently the value of Content-Length of the authorization request reflects the size of its payload size.
	//
	// Exact, prefix and suffix matches are supported (similar to the
	// [authorization policy rule syntax](https://istio.io/latest/docs/reference/config/security/authorization-policy/#Rule)
	// except the presence match):
	// - Exact match: "abc" will match on value "abc".
	// - Prefix match: "abc*" will match on value "abc" and "abcd".
	// - Suffix match: "*abc" will match on value "abc" and "xabc".
	IncludeRequestHeadersInCheck []string `json:"includeRequestHeadersInCheck,omitempty"`
	// Set of additional fixed headers that should be included in the authorization request sent to the authorization service.
	// Key is the header name and value is the header value.
	// Note that client request of the same key or headers specified in include_request_headers_in_check will be overridden.
	IncludeAdditionalHeadersInCheck map[string]string `json:"includeAdditionalHeadersInCheck,omitempty"`
	// If set, the client request body will be included in the authorization request sent to the authorization service.
	IncludeRequestBodyInCheck *ExtensionProviderEnvoyExternalAuthorizationRequestBody `json:"includeRequestBodyInCheck,omitempty"`
	// List of headers from the authorization service that should be added or overridden in the original request and
	// forwarded to the upstream when the authorization check result is allowed (HTTP code 200).
	// If not specified, the original request will not be modified and forwarded to backend as-is.
	// Note, any existing headers will be overridden.
	//
	// Exact, prefix and suffix matches are supported (similar to the
	// [authorization policy rule syntax](https://istio.io/latest/docs/reference/config/security/authorization-policy/#Rule)
	// except the presence match):
	// - Exact match: "abc" will match on value "abc".
	// - Prefix match: "abc*" will match on value "abc" and "abcd".
	// - Suffix match: "*abc" will match on value "abc" and "xabc".
	HeadersToUpstreamOnAllow []string `json:"headersToUpstreamOnAllow,omitempty"`
	// List of headers from the authorization service that should be forwarded to downstream when the authorization
	// check result is not allowed (HTTP code other than 200).
	// If not specified, all the authorization response headers, except *Authority (Host)* will be in the response to
	// the downstream.
	// When a header is included in this list, *Path*, *Status*, *Content-Length*, *WWWAuthenticate* and *Location* are
	// automatically added.
	// Note, the body from the authorization service is always included in the response to downstream.
	//
	// Exact, prefix and suffix matches are supported (similar to the
	// [authorization policy rule syntax](https://istio.io/latest/docs/reference/config/security/authorization-policy/#Rule)
	// except the presence match):
	// - Exact match: "abc" will match on value "abc".
	// - Prefix match: "abc*" will match on value "abc" and "abcd".
	// - Suffix match: "*abc" will match on value "abc" and "xabc".
	HeadersToDownstreamOnDeny []string `json:"headersToDownstreamOnDeny,omitempty"`
	// List of headers from the authorization service that should be forwarded to downstream when the authorization
	// check result is allowed (HTTP code 200).
	// If not specified, the original response will not be modified and forwarded to downstream as-is.
	// Note, any existing headers will be overridden.
	//
	// Exact, prefix and suffix matches are supported (similar to the
	// [authorization policy rule syntax](https://istio.io/latest/docs/reference/config/security/authorization-policy/#Rule)
	// except the presence match):
	// - Exact match: "abc" will match on value "abc".
	// - Prefix match: "abc*" will match on value "abc" and "abcd".
	// - Suffix match: "*abc" will match on value "abc" and "xabc".
	HeadersToDownstreamOnAllow []string `json:"headersToDownstreamOnAllow,omitempty"`
}

type ExtensionProviderEnvoyExternalAuthorizationGrpcProvider struct {
	// REQUIRED. Specifies the service that implements the Envoy ext_authz gRPC authorization service.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "my-ext-authz.foo.svc.cluster.local" or "bar/my-ext-authz.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// The maximum duration that the proxy will wait for a response from the provider, this is the timeout for a specific request (default timeout: 600s).
	// When this timeout condition is met, the proxy marks the communication to the authorization service as failure.
	// In this situation, the response sent back to the client will depend on the configured `fail_open` field.
	Timeout *v1.Duration `json:"timeout,omitempty"`
	// If true, the HTTP request or TCP connection will be allowed even if the communication with the authorization service has failed,
	// or if the authorization service has returned a HTTP 5xx error.
	// Default is false. For HTTP request, it will be rejected with 403 (HTTP Forbidden). For TCP connection, it will be closed immediately.
	FailOpen bool `json:"failOpen,omitempty"`
	// Sets the HTTP status that is returned to the client when there is a network error to the authorization service.
	// The default status is "403" (HTTP Forbidden).
	StatusOnError string `json:"statusOnError,omitempty"`
	// If set, the client request body will be included in the authorization request sent to the authorization service.
	IncludeRequestBodyInCheck *ExtensionProviderEnvoyExternalAuthorizationRequestBody `json:"includeRequestBodyInCheck,omitempty"`
}

// Defines configuration for a Zipkin tracer.

type ExtensionProviderZipkinTracingProvider struct {
	// REQUIRED. Specifies the service that the Zipkin API.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "zipkin.default.svc.cluster.local" or "bar/zipkin.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// Optional. Controls the overall path length allowed in a reported span.
	// NOTE: currently only controls max length of the path tag.
	MaxTagLength uint32 `json:"maxTagLength,omitempty"`
	// Optional. A 128 bit trace id will be used in Istio.
	// If true, will result in a 64 bit trace id being used.
	Enable64BitTraceID bool `json:"enable64bitTraceId,omitempty"`
}

// Defines configuration for a Lightstep tracer.
// Note: Lightstep has moved to OpenTelemetry-based integrations. Istio 1.15+
// will generate OpenTelemetry-compatible configuration when using this option.

type ExtensionProviderLightstepTracingProvider struct {
	// REQUIRED. Specifies the service for the Lightstep collector.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "lightstep.default.svc.cluster.local" or "bar/lightstep.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// The Lightstep access token.
	AccessToken string `json:"accessToken,omitempty"`
	// Optional. Controls the overall path length allowed in a reported span.
	// NOTE: currently only controls max length of the path tag.
	MaxTagLength uint32 `json:"maxTagLength,omitempty"`
}

// Defines configuration for a Datadog tracer.

type ExtensionProviderDatadogTracingProvider struct {
	// REQUIRED. Specifies the service for the Datadog agent.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "datadog.default.svc.cluster.local" or "bar/datadog.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// Optional. Controls the overall path length allowed in a reported span.
	// NOTE: currently only controls max length of the path tag.
	MaxTagLength uint32 `json:"maxTagLength,omitempty"`
}

// Defines configuration for a SkyWalking tracer.

type ExtensionProviderSkyWalkingTracingProvider struct {
	// REQUIRED. Specifies the service for the SkyWalking receiver.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "skywalking.default.svc.cluster.local" or "bar/skywalking.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// Optional. The SkyWalking OAP access token.
	AccessToken string `json:"accessToken,omitempty"`
}

// Defines configuration for Stackdriver.
//
// WARNING: Stackdriver tracing uses OpenCensus configuration under the hood and, as a result, cannot be used
// alongside any OpenCensus provider configuration. This is due to a limitation in the implementation of OpenCensus
// driver in Envoy.

type ExtensionProviderStackdriverProvider struct {
	// Optional. Controls the overall path length allowed in a reported span.
	// NOTE: currently only controls max length of the path tag.
	MaxTagLength uint32 `json:"maxTagLength,omitempty"`
	// Optional. Controls Stackdriver logging behavior.
	Logging *ExtensionProviderStackdriverProviderLogging `json:"logging,omitempty"`
}

// Defines configuration for an OpenCensus tracer writing to an OpenCensus backend.
//
// WARNING: OpenCensusAgentTracingProviders should be used with extreme care. Configuration of
// OpenCensus providers CANNOT be changed during the course of proxy's lifetime due to a limitation
// in the implementation of OpenCensus driver in Envoy. This means only a single provider configuration
// may be used for OpenCensus at any given time for a proxy or group of proxies AND that any change to the provider
// configuration MUST be accompanied by a restart of all proxies that will use that configuration.
//
// NOTE: Stackdriver tracing uses OpenCensus configuration under the hood and, as a result, cannot be used
// alongside OpenCensus provider configuration.

type ExtensionProviderOpenCensusAgentTracingProvider struct {
	// REQUIRED. Specifies the service for the OpenCensusAgent.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "ocagent.default.svc.cluster.local" or "bar/ocagent.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// Specifies the set of context propagation headers used for distributed
	// tracing. Default is `["W3C_TRACE_CONTEXT"]`. If multiple values are specified,
	// the proxy will attempt to read each header for each request and will
	// write all headers.
	Context []ExtensionProviderOpenCensusAgentTracingProviderTraceContext `json:"context,omitempty"`
	// Optional. Controls the overall path length allowed in a reported span.
	// NOTE: currently only controls max length of the path tag.
	MaxTagLength uint32 `json:"maxTagLength,omitempty"`
}

type ExtensionProviderPrometheusMetricsProvider struct{}

// Defines configuration for Envoy-based access logging that writes to
// local files (and/or standard streams).
type ExtensionProviderEnvoyFileAccessLogProvider struct {
	// Path to a local file to write the access log entries.
	// This may be used to write to streams, via `/dev/stderr` and `/dev/stdout`
	// If unspecified, defaults to `/dev/stdout`.
	Path string `json:"path,omitempty"`
	// Optional. Allows overriding of the default access log format.
	LogFormat *ExtensionProviderEnvoyFileAccessLogProviderLogFormat `json:"logFormat,omitempty"`
}

// Defines configuration for an Envoy [Access Logging Service](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/grpc/v3/als.proto#grpc-access-log-service-als)
// integration for HTTP traffic.

type ExtensionProviderEnvoyHTTPGrpcV3LogProvider struct {
	// REQUIRED. Specifies the service that implements the Envoy ALS gRPC authorization service.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "envoy-als.foo.svc.cluster.local" or "bar/envoy-als.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// Optional. The friendly name of the access log.
	// Defaults:
	// -  "http_envoy_accesslog"
	// -  "listener_envoy_accesslog"
	LogName string `json:"logName,omitempty"`
	// Optional. Additional filter state objects to log.
	FilterStateObjectsToLog []string `json:"filterStateObjectsToLog,omitempty"`
	// Optional. Additional request headers to log.
	AdditionalRequestHeadersToLog []string `json:"additionalRequestHeadersToLog,omitempty"`
	// Optional. Additional response headers to log.
	AdditionalResponseHeadersToLog []string `json:"additionalResponseHeadersToLog,omitempty"`
	// Optional. Additional response trailers to log.
	AdditionalResponseTrailersToLog []string `json:"additionalResponseTrailersToLog,omitempty"`
}

// Defines configuration for an Envoy [Access Logging Service](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/grpc/v3/als.proto#grpc-access-log-service-als)
// integration for TCP traffic.

type ExtensionProviderEnvoyTCPGrpcV3LogProvider struct {
	// REQUIRED. Specifies the service that implements the Envoy ALS gRPC authorization service.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "envoy-als.foo.svc.cluster.local" or "bar/envoy-als.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// Optional. The friendly name of the access log.
	// Defaults:
	// - "tcp_envoy_accesslog"
	// - "listener_envoy_accesslog"
	LogName string `json:"logName,omitempty"`
	// Optional. Additional filter state objects to log.
	FilterStateObjectsToLog []string `json:"filterStateObjectsToLog,omitempty"`
}

// Defines configuration for an Envoy [OpenTelemetry (gRPC) Access Log](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/open_telemetry/v3/logs_service.proto)

type ExtensionProviderEnvoyOpenTelemetryLogProvider struct {
	// REQUIRED. Specifies the service that implements the Envoy ALS gRPC authorization service.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "envoy-als.foo.svc.cluster.local" or "bar/envoy-als.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// Optional. The friendly name of the access log.
	// Defaults:
	// - "otel_envoy_accesslog"
	LogName string `json:"logName,omitempty"`
	// Optional. Format for the proxy access log
	// Empty value results in proxy's default access log format, following Envoy access logging formatting.
	LogFormat *ExtensionProviderEnvoyOpenTelemetryLogProviderLogFormat `json:"logFormat,omitempty"`
}

// Defines configuration for an OpenTelemetry tracing backend. Istio 1.16.1 or higher is needed.

type ExtensionProviderOpenTelemetryTracingProvider struct {
	// REQUIRED. Specifies the OpenTelemetry endpoint that will receive OTLP traces.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "otlp.default.svc.cluster.local" or "bar/otlp.example.com".
	// +kubebuilder:validation:Required
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	// +kubebuilder:validation:Required
	Port uint32 `json:"port"`
	// Optional. Controls the overall path length allowed in a reported span.
	// NOTE: currently only controls max length of the path tag.
	MaxTagLength uint32 `json:"maxTagLength,omitempty"`
}

type ExtensionProviderStackdriverProviderLogging struct {
	// Collection of tag names and tag expressions to include in the log
	// entry. Conflicts are resolved by the tag name by overriding previously
	// supplied values.
	//
	// Example:
	//
	//	labels:
	//	  path: request.url_path
	//	  foo: request.headers['x-foo']
	Labels map[string]string `json:"labels"`
}

// +kubebuilder:validation:XValidation:message="At most one of [text labels] should be set",rule="[has(self.text), has(self.labels)].exists_one(x,x)"
type ExtensionProviderEnvoyFileAccessLogProviderLogFormat struct {
	// Textual format for the envoy access logs. Envoy [command operators](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage#command-operators) may be
	// used in the format. The [format string documentation](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage#config-access-log-format-strings)
	// provides more information.
	//
	// NOTE: Istio will insert a newline ('\n') on all formats (if missing).
	//
	// Example: `text: "%LOCAL_REPLY_BODY%:%RESPONSE_CODE%:path=%REQ(:path)%"`
	Text string `json:"text,omitempty"` // oneof
	// JSON structured format for the envoy access logs. Envoy [command operators](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage#command-operators)
	// can be used as values for fields within the Struct. Values are rendered
	// as strings, numbers, or boolean values, as appropriate
	// (see: [format dictionaries](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage#config-access-log-format-dictionaries)). Nested JSON is
	// supported for some command operators (e.g. `FILTER_STATE` or `DYNAMIC_METADATA`).
	// Use `labels: {}` for default envoy JSON log format.
	//
	// Example:
	// ```
	// labels:
	//
	//	status: "%RESPONSE_CODE%"
	//	message: "%LOCAL_REPLY_BODY%"
	//
	// ```
	Labels map[string]string `json:"labels,omitempty"` // oneof
}

type ExtensionProviderEnvoyOpenTelemetryLogProviderLogFormat struct {
	// Textual format for the envoy access logs. Envoy [command operators](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage#command-operators) may be
	// used in the format. The [format string documentation](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage#config-access-log-format-strings)
	// provides more information.
	// Alias to `body` filed in [Open Telemetry](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/open_telemetry/v3/logs_service.proto)
	// Example: `text: "%LOCAL_REPLY_BODY%:%RESPONSE_CODE%:path=%REQ(:path)%"`
	Text string `json:"text,omitempty"`
	// Optional. Additional attributes that describe the specific event occurrence.
	// Structured format for the envoy access logs. Envoy [command operators](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage#command-operators)
	// can be used as values for fields within the Struct. Values are rendered
	// as strings, numbers, or boolean values, as appropriate
	// (see: [format dictionaries](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage#config-access-log-format-dictionaries)). Nested JSON is
	// supported for some command operators (e.g. FILTER_STATE or DYNAMIC_METADATA).
	// Alias to `attributes` filed in [Open Telemetry](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/open_telemetry/v3/logs_service.proto)
	//
	// Example:
	// ```
	// labels:
	//
	//	status: "%RESPONSE_CODE%"
	//	message: "%LOCAL_REPLY_BODY%"
	//
	// ```
	Labels map[string]string `json:"labels,omitempty"`
}
