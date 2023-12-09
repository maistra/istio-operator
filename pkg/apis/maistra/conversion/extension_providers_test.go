package conversion

import (
	"reflect"
	"strings"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var extensionProvidersTestCases []conversionExtensionProvidersTestCase

type conversionExtensionProvidersTestCase struct {
	name       string
	spec       *v2.ControlPlaneSpec
	helmValues string
}

func init() {
	for _, v := range versions.TestedVersions {
		if v.AtLeast(versions.V2_4) {
			extensionProvidersTestCases = append(extensionProvidersTestCases, extensionProvidersTestCasesV2(v)...)
		}
	}
}

func TestExtensionProvidersConversionFromV2(t *testing.T) {
	for _, tc := range extensionProvidersTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			actualHelmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateExtensionProvidersValues(specCopy, actualHelmValues.GetContent()); err != nil {
				t.Errorf("error converting to values: %s", err)
			}

			expectedHelmValues := v1.HelmValues{}
			if err := expectedHelmValues.UnmarshalYAML([]byte(tc.helmValues)); err != nil {
				t.Fatalf("failed to parse helm values: %s", err)
			}
			if !reflect.DeepEqual(expectedHelmValues.DeepCopy(), actualHelmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", expectedHelmValues.GetContent(), actualHelmValues.GetContent())
			}
			specv2 := v2.ControlPlaneSpec{}
			if err := populateExtensionProvidersConfig(expectedHelmValues.DeepCopy(), &specv2); err != nil {
				t.Errorf("error converting from values: %s", err)
			}
			assertEquals(t, tc.spec.MeshConfig, specv2.MeshConfig)
		})
	}
}

func extensionProvidersTestCasesV2(version versions.Version) []conversionExtensionProvidersTestCase {
	ver := version.String()
	return []conversionExtensionProvidersTestCase{
		{
			name: "nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
			},
			helmValues: "{}",
		},
		{
			name: "empty." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders: []
`,
		},
		{
			name: "prometheus." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{
						{
							Name:       "prometheus",
							Prometheus: &v2.ExtensionProviderPrometheusConfig{},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders:
  - name: prometheus
    prometheus: {}
`,
		},
		{
			name: "zipkin_required_fields." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{
						{
							Name: "zipkin",
							Zipkin: &v2.ExtensionProviderZipkinTracingConfig{
								Service: "zipkin.default.svc.cluster.local",
								Port:    8000,
							},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders:
  - name: zipkin
    zipkin:
      service: zipkin.default.svc.cluster.local
      port: 8000
`,
		},
		{
			name: "zipkin_all_fields." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{
						{
							Name: "zipkin",
							Zipkin: &v2.ExtensionProviderZipkinTracingConfig{
								Service:            "zipkin.default.svc.cluster.local",
								Port:               8000,
								MaxTagLength:       int64Ptr(64),
								Enable64bitTraceID: boolPtr(true),
							},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders:
  - name: zipkin
    zipkin:
      service: zipkin.default.svc.cluster.local
      port: 8000
      maxTagLength: 64
      enable64bitTraceId: true
`,
		},
		{
			name: "opentelemetry_required_fields." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{
						{
							Name: "opentelemetry",
							Opentelemetry: &v2.ExtensionProviderOtelTracingConfig{
								Service: "otlp.default.svc.cluster.local",
								Port:    8000,
							},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders:
  - name: opentelemetry
    opentelemetry:
      service: otlp.default.svc.cluster.local
      port: 8000
`,
		},
		{
			name: "opentelemetry_all_fields." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{
						{
							Name: "opentelemetry",
							Opentelemetry: &v2.ExtensionProviderOtelTracingConfig{
								Service:      "otlp.default.svc.cluster.local",
								Port:         8000,
								MaxTagLength: int64Ptr(64),
							},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders:
  - name: opentelemetry
    opentelemetry:
      service: otlp.default.svc.cluster.local
      port: 8000
      maxTagLength: 64
`,
		},
		{
			name: "envoyOtelAls_required_fields." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{
						{
							Name: "envoyOtelAls",
							EnvoyOtelAls: &v2.ExtensionProviderEnvoyOtelLogConfig{
								Service: "envoy-als.foo.svc.cluster.local",
								Port:    8000,
							},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders:
  - name: envoyOtelAls
    envoyOtelAls:
      service: envoy-als.foo.svc.cluster.local
      port: 8000
`,
		},
		{
			name: "envoyOtelAls_all_fields." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{
						{
							Name: "envoyOtelAls",
							EnvoyOtelAls: &v2.ExtensionProviderEnvoyOtelLogConfig{
								Service: "envoy-als.foo.svc.cluster.local",
								Port:    8000,
								LogName: strPtr("otel_envoy_accesslog"),
							},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders:
  - name: envoyOtelAls
    envoyOtelAls:
      service: envoy-als.foo.svc.cluster.local
      port: 8000
      logName: otel_envoy_accesslog
`,
		},
		{
			name: "envoyExtAuthzHttp." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{
						{
							Name: "ext-authz-http",
							EnvoyExtAuthzHTTP: &v2.ExtensionProviderEnvoyExternalAuthorizationHTTPConfig{
								Service:                      "ext-authz.foo.svc.cluster.local",
								Port:                         8000,
								Timeout:                      strPtr("1s"),
								PathPrefix:                   strPtr("/authz"),
								FailOpen:                     boolPtr(true),
								StatusOnError:                strPtr("500"),
								IncludeRequestHeadersInCheck: []string{"x-ext-authz"},
								IncludeAdditionalHeadersInCheck: map[string]string{
									"x-ext-authz-additional-header": "value",
								},
								IncludeRequestBodyInCheck: &v2.ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig{
									MaxRequestBytes:     int64Ptr(100),
									AllowPartialMessage: boolPtr(true),
									PackAsBytes:         boolPtr(true),
								},
								HeadersToUpstreamOnAllow:   []string{"upstream-on-allow"},
								HeadersToDownstreamOnDeny:  []string{"downstream-on-deny"},
								HeadersToDownstreamOnAllow: []string{"downstream-on-allow"},
							},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders:
  - name: ext-authz-http
    envoyExtAuthzHttp:
      service: ext-authz.foo.svc.cluster.local
      port: 8000
      timeout: 1s
      pathPrefix: "/authz"
      failOpen: true
      statusOnError: "500"
      includeRequestHeadersInCheck:
      - x-ext-authz
      includeAdditionalHeadersInCheck:
        x-ext-authz-additional-header: value
      includeRequestBodyInCheck:
        maxRequestBytes: 100
        allowPartialMessage: true
        packAsBytes: true
      headersToUpstreamOnAllow: 
      - upstream-on-allow
      headersToDownstreamOnDeny: 
      - downstream-on-deny
      headersToDownstreamOnAllow:
      - downstream-on-allow
`,
		},
		{
			name: "prometheus-and-envoyExtAuthzHttp." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{
						{
							Name: "ext-authz-http",
							EnvoyExtAuthzHTTP: &v2.ExtensionProviderEnvoyExternalAuthorizationHTTPConfig{
								Service: "ext-authz.foo.svc.cluster.local",
								Port:    8000,
							},
						},
						{
							Name:       "prometheus",
							Prometheus: &v2.ExtensionProviderPrometheusConfig{},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders:
  - name: ext-authz-http
    envoyExtAuthzHttp:
      service: ext-authz.foo.svc.cluster.local
      port: 8000
  - name: prometheus
    prometheus: {}
`,
		},
		{
			name: "envoyExtAuthzGrpc." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				MeshConfig: &v2.MeshConfig{
					ExtensionProviders: []*v2.ExtensionProviderConfig{
						{
							Name: "ext-authz-grpc",
							EnvoyExtAuthzGRPC: &v2.ExtensionProviderEnvoyExternalAuthorizationGRPCConfig{
								Service:       "ext-authz.foo.svc.cluster.local",
								Port:          8000,
								Timeout:       strPtr("1s"),
								FailOpen:      boolPtr(true),
								StatusOnError: strPtr("500"),
								IncludeRequestBodyInCheck: &v2.ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig{
									MaxRequestBytes:     int64Ptr(100),
									AllowPartialMessage: boolPtr(true),
									PackAsBytes:         boolPtr(true),
								},
							},
						},
					},
				},
			},
			helmValues: `
meshConfig:
  extensionProviders:
  - name: ext-authz-grpc
    envoyExtAuthzGrpc:
      service: ext-authz.foo.svc.cluster.local
      port: 8000
      timeout: 1s
      failOpen: true
      statusOnError: "500"
      includeRequestBodyInCheck:
        maxRequestBytes: 100
        allowPartialMessage: true
        packAsBytes: true
`,
		},
	}
}

// TestStringPortInEnvoyExtAuthzHTTPValues checks that convertEnvoyExtAuthzHTTPValuesToConfig returns an error instead
// of panicking when the users specifies the port number using a string instead of an int.
func TestStringPortInEnvoyExtAuthzHTTPValues(t *testing.T) {
	helmValues := v1.NewHelmValues(
		map[string]interface{}{
			"service": "test",
			"port":    "80", // string instead of an int
		})

	if _, err := convertEnvoyExtAuthzHTTPValuesToConfig(helmValues); err == nil {
		t.Fatalf("expected convertEnvoyExtAuthzHTTPValuesToConfig to return error, but it returned nil")
	} else if !strings.Contains(err.Error(), "80 is of the type string") {
		t.Fatalf("expected error message to contain '80 is of the type string', got: %s", err)
	}
}

// TestConversionExtensionProviderZipkin checks v1 helmValues to v2 spec
func TestConverionExtensionProviderZipkin(t *testing.T) {
	helmValues := v1.NewHelmValues(
		map[string]interface{}{
			"name": "zipkin",
			"zipkin": map[string]interface{}{
				"service": "jaeger-collector.istio-system.svc.cluster.local",
				"port":    int64(9411),
			},
		})
	if _, err := convertProviderValuesToConfig(helmValues); err != nil {
		t.Fatalf("expected convertProviderValuesToConfig to return no error, got: %s", err)
	}

	helmValuesFull := v1.NewHelmValues(
		map[string]interface{}{
			"name": "zipkin",
			"zipkin": map[string]interface{}{
				"service":            "jaeger-collector.istio-system.svc.cluster.local",
				"port":               int64(9411),
				"enable64bitTraceId": true,
				"maxTagLength":       int64(128),
			},
		})
	if _, err := convertProviderValuesToConfig(helmValuesFull); err != nil {
		t.Fatalf("expected convertProviderValuesToConfig to return no error, got: %s", err)
	}
}
