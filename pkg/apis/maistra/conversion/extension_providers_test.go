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
	}
}

func TestInvalidValueType(t *testing.T) {
	helmValues := v1.HelmValues{}
	if err := helmValues.UnmarshalYAML([]byte(`
meshConfig:
  extensionProviders:
  - envoyExtAuthzHttp:
      service: "test"
      port: "80"
`)); err != nil {
		t.Fatalf("failed to parse helm values: %s", err)
	}

	specV1 := v1.ControlPlaneSpec{
		Version: "v2.4",
		Istio:   &helmValues,
	}
	var specV2 v2.ControlPlaneSpec
	if err := Convert_v1_ControlPlaneSpec_To_v2_ControlPlaneSpec(&specV1, &specV2, nil); err != nil {
		t.Fatalf("failed to convert spec: %s", err)
	}

	errorMessage, found, err := specV2.TechPreview.GetString("errored.message")
	if err != nil {
		t.Fatalf("failed to get techPreview.errored.message field: %s", err)
	} else if !found {
		t.Fatalf("expected to find techPreview.errored.message field")
	}

	if !strings.Contains(errorMessage, "80 is of the type string") {
		t.Fatalf("expected techPreview.errored.message to contain '80 is of the type string', got: %s", errorMessage)
	}
}
