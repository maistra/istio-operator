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

package v1alpha1

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

//
// COPIED from destination_rule.pb.go
//

// TCP keepalive.
type ConnectionPoolSettingsTCPSettingsTCPKeepalive struct {
	// Maximum number of keepalive probes to send without response before
	// deciding the connection is dead. Default is to use the OS level configuration
	// (unless overridden, Linux defaults to 9.)
	Probes uint32 `json:"probes,omitempty"`
	// The time duration a connection needs to be idle before keep-alive
	// probes start being sent. Default is to use the OS level configuration
	// (unless overridden, Linux defaults to 7200s (ie 2 hours.)
	Time *v1.Duration `json:"time,omitempty"`
	// The time duration between keep-alive probes.
	// Default is to use the OS level configuration
	// (unless overridden, Linux defaults to 75s.)
	Interval *v1.Duration `json:"interval,omitempty"`
}

// Locality-weighted load balancing allows administrators to control the
// distribution of traffic to endpoints based on the localities of where the
// traffic originates and where it will terminate. These localities are
// specified using arbitrary labels that designate a hierarchy of localities in
// {region}/{zone}/{sub-zone} form. For additional detail refer to
// [Locality Weight](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/load_balancing/locality_weight)
// The following example shows how to setup locality weights mesh-wide.
//
// Given a mesh with workloads and their service deployed to "us-west/zone1/*"
// and "us-west/zone2/*". This example specifies that when traffic accessing a
// service originates from workloads in "us-west/zone1/*", 80% of the traffic
// will be sent to endpoints in "us-west/zone1/*", i.e the same zone, and the
// remaining 20% will go to endpoints in "us-west/zone2/*". This setup is
// intended to favor routing traffic to endpoints in the same locality.
// A similar setting is specified for traffic originating in "us-west/zone2/*".
//
// ```yaml
//
//	distribute:
//	  - from: us-west/zone1/*
//	    to:
//	      "us-west/zone1/*": 80
//	      "us-west/zone2/*": 20
//	  - from: us-west/zone2/*
//	    to:
//	      "us-west/zone1/*": 20
//	      "us-west/zone2/*": 80
//
// ```
//
// If the goal of the operator is not to distribute load across zones and
// regions but rather to restrict the regionality of failover to meet other
// operational requirements an operator can set a 'failover' policy instead of
// a 'distribute' policy.
//
// The following example sets up a locality failover policy for regions.
// Assume a service resides in zones within us-east, us-west & eu-west
// this example specifies that when endpoints within us-east become unhealthy
// traffic should failover to endpoints in any zone or sub-zone within eu-west
// and similarly us-west should failover to us-east.
//
// ```yaml
//
//	failover:
//	  - from: us-east
//	    to: eu-west
//	  - from: us-west
//	    to: us-east
//
// ```
// Locality load balancing settings.
type LocalityLoadBalancerSetting struct {
	// Optional: only one of distribute, failover or failoverPriority can be set.
	// Explicitly specify loadbalancing weight across different zones and geographical locations.
	// Refer to [Locality weighted load balancing](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/load_balancing/locality_weight)
	// If empty, the locality weight is set according to the endpoints number within it.
	Distribute []*LocalityLoadBalancerSettingDistribute `json:"distribute,omitempty"`
	// Optional: only one of distribute, failover or failoverPriority can be set.
	// Explicitly specify the region traffic will land on when endpoints in local region becomes unhealthy.
	// Should be used together with OutlierDetection to detect unhealthy endpoints.
	// Note: if no OutlierDetection specified, this will not take effect.
	Failover []*LocalityLoadBalancerSettingFailover `json:"failover,omitempty"`
	// failoverPriority is an ordered list of labels used to sort endpoints to do priority based load balancing.
	// This is to support traffic failover across different groups of endpoints.
	// Two kinds of labels can be specified:
	//
	//   - Specify only label keys `[key1, key2, key3]`, istio would compare the label values of client with endpoints.
	//     Suppose there are total N label keys `[key1, key2, key3, ...keyN]` specified:
	//
	//     1. Endpoints matching all N labels with the client proxy have priority P(0) i.e. the highest priority.
	//     2. Endpoints matching the first N-1 labels with the client proxy have priority P(1) i.e. second highest priority.
	//     3. By extension of this logic, endpoints matching only the first label with the client proxy has priority P(N-1) i.e. second lowest priority.
	//     4. All the other endpoints have priority P(N) i.e. lowest priority.
	//
	//   - Specify labels with key and value `[key1=value1, key2=value2, key3=value3]`, istio would compare the labels with endpoints.
	//     Suppose there are total N labels `[key1=value1, key2=value2, key3=value3, ...keyN=valueN]` specified:
	//
	//     1. Endpoints matching all N labels have priority P(0) i.e. the highest priority.
	//     2. Endpoints matching the first N-1 labels have priority P(1) i.e. second highest priority.
	//     3. By extension of this logic, endpoints matching only the first label has priority P(N-1) i.e. second lowest priority.
	//     4. All the other endpoints have priority P(N) i.e. lowest priority.
	//
	// Note: For a label to be considered for match, the previous labels must match, i.e. nth label would be considered matched only if first n-1 labels match.
	//
	// It can be any label specified on both client and server workloads.
	// The following labels which have special semantic meaning are also supported:
	//
	//   - `topology.istio.io/network` is used to match the network metadata of an endpoint, which can be specified by pod/namespace label `topology.istio.io/network`, sidecar env `ISTIO_META_NETWORK` or MeshNetworks.
	//   - `topology.istio.io/cluster` is used to match the clusterID of an endpoint, which can be specified by pod label `topology.istio.io/cluster` or pod env `ISTIO_META_CLUSTER_ID`.
	//   - `topology.kubernetes.io/region` is used to match the region metadata of an endpoint, which maps to Kubernetes node label `topology.kubernetes.io/region` or the deprecated label `failure-domain.beta.kubernetes.io/region`.
	//   - `topology.kubernetes.io/zone` is used to match the zone metadata of an endpoint, which maps to Kubernetes node label `topology.kubernetes.io/zone` or the deprecated label `failure-domain.beta.kubernetes.io/zone`.
	//   - `topology.istio.io/subzone` is used to match the subzone metadata of an endpoint, which maps to Istio node label `topology.istio.io/subzone`.
	//
	// The below topology config indicates the following priority levels:
	//
	// ```yaml
	// failoverPriority:
	// - "topology.istio.io/network"
	// - "topology.kubernetes.io/region"
	// - "topology.kubernetes.io/zone"
	// - "topology.istio.io/subzone"
	// ```
	//
	// 1. endpoints match same [network, region, zone, subzone] label with the client proxy have the highest priority.
	// 2. endpoints have same [network, region, zone] label but different [subzone] label with the client proxy have the second highest priority.
	// 3. endpoints have same [network, region] label but different [zone] label with the client proxy have the third highest priority.
	// 4. endpoints have same [network] but different [region] labels with the client proxy have the fourth highest priority.
	// 5. all the other endpoints have the same lowest priority.
	//
	// Suppose a service associated endpoints reside in multi clusters, the below example represents:
	// 1. endpoints in `clusterA` and has `version=v1` label have P(0) priority.
	// 2. endpoints not in `clusterA` but has `version=v1` label have P(1) priority.
	// 2. all the other endpoints have P(2) priority.
	//
	// ```yaml
	// failoverPriority:
	// - "version=v1"
	// - "topology.istio.io/cluster=clusterA"
	// ```
	//
	// Optional: only one of distribute, failover or failoverPriority can be set.
	// And it should be used together with `OutlierDetection` to detect unhealthy endpoints, otherwise has no effect.
	FailoverPriority []string `json:"failoverPriority,omitempty"`
	// enable locality load balancing, this is DestinationRule-level and will override mesh wide settings in entirety.
	// e.g. true means that turn on locality load balancing for this DestinationRule no matter what mesh wide settings is.
	Enabled bool `json:"enabled,omitempty"`
}

// Describes how traffic originating in the 'from' zone or sub-zone is
// distributed over a set of 'to' zones. Syntax for specifying a zone is
// {region}/{zone}/{sub-zone} and terminal wildcards are allowed on any
// segment of the specification. Examples:
//
// `*` - matches all localities
//
// `us-west/*` - all zones and sub-zones within the us-west region
//
// `us-west/zone-1/*` - all sub-zones within us-west/zone-1
type LocalityLoadBalancerSettingDistribute struct {
	// Originating locality, '/' separated, e.g. 'region/zone/sub_zone'.
	From string `json:"from,omitempty"`
	// Map of upstream localities to traffic distribution weights. The sum of
	// all weights should be 100. Any locality not present will
	// receive no traffic.
	To map[string]uint32 `json:"to,omitempty"`
}

// Specify the traffic failover policy across regions. Since zone and sub-zone
// failover is supported by default this only needs to be specified for
// regions when the operator needs to constrain traffic failover so that
// the default behavior of failing over to any endpoint globally does not
// apply. This is useful when failing over traffic across regions would not
// improve service health or may need to be restricted for other reasons
// like regulatory controls.
type LocalityLoadBalancerSettingFailover struct {
	// Originating region.
	From string `json:"from,omitempty"`
	// Destination region the traffic will fail over to when endpoints in
	// the 'from' region becomes unhealthy.
	To string `json:"to,omitempty"`
}

// TLS connection mode
type ClientTLSSettingsTLSmode int32

// SSL/TLS related settings for upstream connections. See Envoy's [TLS
// context](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/transport_sockets/tls/v3/common.proto.html#common-tls-configuration)
// for more details. These settings are common to both HTTP and TCP upstreams.
//
// For example, the following rule configures a client to use mutual TLS
// for connections to upstream database cluster.
//
// {{<tabset category-name="example">}}
// {{<tab name="v1alpha3" category-value="v1alpha3">}}
// ```yaml
// apiVersion: networking.istio.io/v1alpha3
// kind: DestinationRule
// metadata:
//
//	name: db-mtls
//
// spec:
//
//	host: mydbserver.prod.svc.cluster.local
//	trafficPolicy:
//	  tls:
//	    mode: MUTUAL
//	    clientCertificate: /etc/certs/myclientcert.pem
//	    privateKey: /etc/certs/client_private_key.pem
//	    caCertificates: /etc/certs/rootcacerts.pem
//
// ```
// {{</tab>}}
//
// {{<tab name="v1beta1" category-value="v1beta1">}}
// ```yaml
// apiVersion: networking.istio.io/v1beta1
// kind: DestinationRule
// metadata:
//
//	name: db-mtls
//
// spec:
//
//	host: mydbserver.prod.svc.cluster.local
//	trafficPolicy:
//	  tls:
//	    mode: MUTUAL
//	    clientCertificate: /etc/certs/myclientcert.pem
//	    privateKey: /etc/certs/client_private_key.pem
//	    caCertificates: /etc/certs/rootcacerts.pem
//
// ```
// {{</tab>}}
// {{</tabset>}}
//
// The following rule configures a client to use TLS when talking to a
// foreign service whose domain matches *.foo.com.
//
// {{<tabset category-name="example">}}
// {{<tab name="v1alpha3" category-value="v1alpha3">}}
// ```yaml
// apiVersion: networking.istio.io/v1alpha3
// kind: DestinationRule
// metadata:
//
//	name: tls-foo
//
// spec:
//
//	host: "*.foo.com"
//	trafficPolicy:
//	  tls:
//	    mode: SIMPLE
//
// ```
// {{</tab>}}
//
// {{<tab name="v1beta1" category-value="v1beta1">}}
// ```yaml
// apiVersion: networking.istio.io/v1beta1
// kind: DestinationRule
// metadata:
//
//	name: tls-foo
//
// spec:
//
//	host: "*.foo.com"
//	trafficPolicy:
//	  tls:
//	    mode: SIMPLE
//
// ```
// {{</tab>}}
// {{</tabset>}}
//
// The following rule configures a client to use Istio mutual TLS when talking
// to rating services.
//
// {{<tabset category-name="example">}}
// {{<tab name="v1alpha3" category-value="v1alpha3">}}
// ```yaml
// apiVersion: networking.istio.io/v1alpha3
// kind: DestinationRule
// metadata:
//
//	name: ratings-istio-mtls
//
// spec:
//
//	host: ratings.prod.svc.cluster.local
//	trafficPolicy:
//	  tls:
//	    mode: ISTIO_MUTUAL
//
// ```
// {{</tab>}}
//
// {{<tab name="v1beta1" category-value="v1beta1">}}
// ```yaml
// apiVersion: networking.istio.io/v1beta1
// kind: DestinationRule
// metadata:
//
//	name: ratings-istio-mtls
//
// spec:
//
//	host: ratings.prod.svc.cluster.local
//	trafficPolicy:
//	  tls:
//	    mode: ISTIO_MUTUAL
//
// ```
// {{</tab>}}
// {{</tabset>}}
type ClientTLSSettings struct {
	// Indicates whether connections to this port should be secured
	// using TLS. The value of this field determines how TLS is enforced.
	Mode ClientTLSSettingsTLSmode `json:"mode,omitempty"`
	// REQUIRED if mode is `MUTUAL`. The path to the file holding the
	// client-side TLS certificate to use.
	// Should be empty if mode is `ISTIO_MUTUAL`.
	ClientCertificate string `json:"clientCertificate,omitempty"`
	// REQUIRED if mode is `MUTUAL`. The path to the file holding the
	// client's private key.
	// Should be empty if mode is `ISTIO_MUTUAL`.
	PrivateKey string `json:"privateKey,omitempty"`
	// OPTIONAL: The path to the file containing certificate authority
	// certificates to use in verifying a presented server certificate. If
	// omitted, the proxy will not verify the server's certificate.
	// Should be empty if mode is `ISTIO_MUTUAL`.
	CaCertificates string `json:"caCertificates,omitempty"`
	// The name of the secret that holds the TLS certs for the
	// client including the CA certificates. This secret must exist in
	// the namespace of the proxy using the certificates.
	// An Opaque secret should contain the following keys and values:
	// `key: <privateKey>`, `cert: <clientCert>`, `cacert: <CACertificate>`,
	// `crl: <certificateRevocationList>`
	// Here CACertificate is used to verify the server certificate.
	// For mutual TLS, `cacert: <CACertificate>` can be provided in the
	// same secret or a separate secret named `<secret>-cacert`.
	// A TLS secret for client certificates with an additional
	// `ca.crt` key for CA certificates and `ca.crl` key for
	// certificate revocation list(CRL) is also supported.
	// Only one of client certificates and CA certificate
	// or credentialName can be specified.
	//
	// **NOTE:** This field is applicable at sidecars only if
	// `DestinationRule` has a `workloadSelector` specified.
	// Otherwise the field will be applicable only at gateways, and
	// sidecars will continue to use the certificate paths.
	CredentialName string `json:"credentialName,omitempty"`
	// A list of alternate names to verify the subject identity in the
	// certificate. If specified, the proxy will verify that the server
	// certificate's subject alt name matches one of the specified values.
	// If specified, this list overrides the value of subject_alt_names
	// from the ServiceEntry. If unspecified, automatic validation of upstream
	// presented certificate for new upstream connections will be done based on the
	// downstream HTTP host/authority header, provided `VERIFY_CERTIFICATE_AT_CLIENT`
	// and `ENABLE_AUTO_SNI` environmental variables are set to `true`.
	SubjectAltNames []string `json:"subjectAltNames,omitempty"`
	// SNI string to present to the server during TLS handshake.
	// If unspecified, SNI will be automatically set based on downstream HTTP
	// host/authority header for SIMPLE and MUTUAL TLS modes, provided `ENABLE_AUTO_SNI`
	// environmental variable is set to `true`.
	Sni string `json:"sni,omitempty"`
	// `insecureSkipVerify` specifies whether the proxy should skip verifying the
	// CA signature and SAN for the server certificate corresponding to the host.
	// This flag should only be set if global CA signature verification is
	// enabled, `VERIFY_CERTIFICATE_AT_CLIENT` environmental variable is set to `true`,
	// but no verification is desired for a specific host. If enabled with or
	// without `VERIFY_CERTIFICATE_AT_CLIENT` enabled, verification of the CA signature and
	// SAN will be skipped.
	//
	// `insecureSkipVerify` is `false` by default.
	// `VERIFY_CERTIFICATE_AT_CLIENT` is `false` by default in Istio version 1.9 but will
	// be `true` by default in a later version where, going forward, it will be
	// enabled by default.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

//
// COPIED from virtual_service.pb.go
//

// Describes the retry policy to use when a HTTP request fails. For
// example, the following rule sets the maximum number of retries to 3 when
// calling ratings:v1 service, with a 2s timeout per retry attempt.
// A retry will be attempted if there is a connect-failure, refused_stream
// or when the upstream server responds with Service Unavailable(503).
//
// {{<tabset category-name="example">}}
// {{<tab name="v1alpha3" category-value="v1alpha3">}}
// ```yaml
// apiVersion: networking.istio.io/v1alpha3
// kind: VirtualService
// metadata:
//
//	name: ratings-route
//
// spec:
//
//	hosts:
//	- ratings.prod.svc.cluster.local
//	http:
//	- route:
//	  - destination:
//	      host: ratings.prod.svc.cluster.local
//	      subset: v1
//	  retries:
//	    attempts: 3
//	    perTryTimeout: 2s
//	    retryOn: connect-failure,refused-stream,503
//
// ```
// {{</tab>}}
//
// {{<tab name="v1beta1" category-value="v1beta1">}}
// ```yaml
// apiVersion: networking.istio.io/v1beta1
// kind: VirtualService
// metadata:
//
//	name: ratings-route
//
// spec:
//
//	hosts:
//	- ratings.prod.svc.cluster.local
//	http:
//	- route:
//	  - destination:
//	      host: ratings.prod.svc.cluster.local
//	      subset: v1
//	  retries:
//	    attempts: 3
//	    perTryTimeout: 2s
//	    retryOn: gateway-error,connect-failure,refused-stream
//
// ```
// {{</tab>}}
// {{</tabset>}}
type HTTPRetry struct {
	// Number of retries to be allowed for a given request. The interval
	// between retries will be determined automatically (25ms+). When request
	// `timeout` of the [HTTP route](https://istio.io/docs/reference/config/networking/virtual-service/#HTTPRoute)
	// or `per_try_timeout` is configured, the actual number of retries attempted also depends on
	// the specified request `timeout` and `per_try_timeout` values. MUST BE >= 0. If `0`, retries will be disabled.
	// The maximum possible number of requests made will be 1 + `attempts`.
	Attempts int32 `json:"attempts,omitempty"`
	// Timeout per attempt for a given request, including the initial call and any retries. Format: 1h/1m/1s/1ms. MUST BE >=1ms.
	// Default is same value as request
	// `timeout` of the [HTTP route](https://istio.io/docs/reference/config/networking/virtual-service/#HTTPRoute),
	// which means no timeout.
	PerTryTimeout *v1.Duration `json:"perTryTimeout,omitempty"`
	// Specifies the conditions under which retry takes place.
	// One or more policies can be specified using a ‘,’ delimited list.
	// If `retry_on` specifies a valid HTTP status, it will be added to retriable_status_codes retry policy.
	// See the [retry policies](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/router_filter#x-envoy-retry-on)
	// and [gRPC retry policies](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/router_filter#x-envoy-retry-grpc-on) for more details.
	RetryOn string `json:"retryOn,omitempty"`
	// Flag to specify whether the retries should retry to other localities.
	// See the [retry plugin configuration](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/http/http_connection_management#retry-plugin-configuration) for more details.
	RetryRemoteLocalities bool `json:"retryRemoteLocalities,omitempty"`
}
