package v2

type ExtensionProviderConfig struct {
	// A unique name identifying the extension provider.
	Name string `json:"name"`
	// Prometheus configures a Prometheus metrics provider.
	Prometheus *ExtensionProviderPrometheusConfig `json:"prometheus,omitempty"`
	// EnvoyExtAuthzHTTP configures an external authorizer that implements
	// the Envoy ext_authz filter authorization check service using the HTTP API.
	EnvoyExtAuthzHTTP *ExtensionProviderEnvoyExternalAuthorizationHttpConfig `json:"envoyExtAuthzHttp,omitempty"`
}

type ExtensionProviderPrometheusConfig struct{}

type ExtensionProviderEnvoyExternalAuthorizationHttpConfig struct {
	// REQUIRED. Specifies the service that implements the Envoy ext_authz HTTP authorization service.
	// The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient
	// to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a
	// service defined by the Kubernetes service or ServiceEntry.
	//
	// Example: "my-ext-authz.foo.svc.cluster.local" or "bar/my-ext-authz.example.com".
	Service string `json:"service"`
	// REQUIRED. Specifies the port of the service.
	Port int64 `json:"port"`
	// The maximum duration that the proxy will wait for a response from the provider (default timeout: 600s).
	// When this timeout condition is met, the proxy marks the communication to the authorization service as failure.
	// In this situation, the response sent back to the client will depend on the configured `fail_open` field.
	Timeout *string `json:"timeout,omitempty"`
	// Sets a prefix to the value of authorization request header *Path*.
	// For example, setting this to "/check" for an original user request at path "/admin" will cause the
	// authorization check request to be sent to the authorization service at the path "/check/admin" instead of "/admin".
	PathPrefix *string `json:"pathPrefix,omitempty"`
	// If true, the user request will be allowed even if the communication with the authorization service has failed,
	// or if the authorization service has returned a HTTP 5xx error.
	// Default is false and the request will be rejected with "Forbidden" response.
	FailOpen *bool `json:"failOpen,omitempty"`
	// Sets the HTTP status that is returned to the client when there is a network error to the authorization service.
	// The default status is "403" (HTTP Forbidden).
	StatusOnError *string `json:"statusOnError,omitempty"`
	// List of client request headers that should be included in the authorization request sent to the authorization service.
	// Note that in addition to the headers specified here following headers are included by default:
	// 1. *Host*, *Method*, *Path* and *Content-Length* are automatically sent.
	// 2. *Content-Length* will be set to 0 and the request will not have a message body. However, the authorization
	// request can include the buffered client request body (controlled by include_request_body_in_check setting),
	// consequently the value of Content-Length of the authorization request reflects the size of its payload size.
	//
	// Exact, prefix and suffix matches are supported (similar to the authorization policy rule syntax except the presence match
	// https://istio.io/latest/docs/reference/config/security/authorization-policy/#Rule):
	// - Exact match: "abc" will match on value "abc".
	// - Prefix match: "abc*" will match on value "abc" and "abcd".
	// - Suffix match: "*abc" will match on value "abc" and "xabc".
	IncludeRequestHeadersInCheck []string `json:"includeRequestHeadersInCheck,omitempty"`
	// Set of additional fixed headers that should be included in the authorization request sent to the authorization service.
	// Key is the header name and value is the header value.
	// Note that client request of the same key or headers specified in include_request_headers_in_check will be overridden.
	IncludeAdditionalHeadersInCheck map[string]string `json:"includeAdditionalHeadersInCheck,omitempty"`
	// If set, the client request body will be included in the authorization request sent to the authorization service.
	IncludeRequestBodyInCheck *ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig `json:"includeRequestBodyInCheck,omitempty"`
}

type ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig struct {
	// Sets the maximum size of a message body that the ext-authz filter will hold in memory.
	// If max_request_bytes is reached, and allow_partial_message is false, Envoy will return a 413 (Payload Too Large).
	// Otherwise the request will be sent to the provider with a partial message.
	// Note that this setting will have precedence over the fail_open field, the 413 will be returned even when the
	// fail_open is set to true.
	MaxRequestBytes *int64 `json:"maxRequestBytes,omitempty"`
	// When this field is true, ext-authz filter will buffer the message until max_request_bytes is reached.
	// The authorization request will be dispatched and no 413 HTTP error will be returned by the filter.
	// A "x-envoy-auth-partial-body: false|true" metadata header will be added to the authorization request message
	// indicating if the body data is partial.
	AllowPartialMessage *bool `json:"allowPartialMessage,omitempty"`
	// nolint:lll
	// If true, the body sent to the external authorization service in the gRPC authorization request is set with raw bytes
	// in the raw_body field (https://github.com/envoyproxy/envoy/blame/cffb095d59d7935abda12b9509bcd136808367bb/api/envoy/service/auth/v3/attribute_context.proto#L153).
	// Otherwise, it will be filled with UTF-8 string in the body field (https://github.com/envoyproxy/envoy/blame/cffb095d59d7935abda12b9509bcd136808367bb/api/envoy/service/auth/v3/attribute_context.proto#L147).
	// This field only works with the envoy_ext_authz_grpc provider and has no effect for the envoy_ext_authz_http provider.
	PackAsBytes *bool `json:"packAsBytes,omitempty"`
}
