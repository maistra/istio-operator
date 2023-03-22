package conversion

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var (
	extensionProvidersTestCases []conversionExtensionProvidersTestCase
	completeIstio               = v1.NewHelmValues(map[string]interface{}{
		"global": map[string]interface{}{
			"multiCluster":  globalMultiClusterDefaults,
			"meshExpansion": globalMeshExpansionDefaults,
		},
	})
)

type conversionExtensionProvidersTestCase struct {
	name       string
	spec       *v2.ControlPlaneSpec
	helmValues string
}

func init() {
	for _, v := range versions.AllV2Versions {
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
				t.Fatalf("error converting to values: %s", err)
			}

			helmValues := &v1.HelmValues{}
			if err := helmValues.UnmarshalYAML([]byte(tc.helmValues)); err != nil {
				t.Fatalf("failed to parse helm values: %s", err)
			}
			if !reflect.DeepEqual(helmValues.DeepCopy(), actualHelmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", helmValues.GetContent(), actualHelmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			mergeMaps(completeIstio.DeepCopy().GetContent(), actualHelmValues.DeepCopy().GetContent())
			if err := populateExtensionProvidersConfig(actualHelmValues.DeepCopy(), specv2); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			assertEquals(t, tc.spec.ExtensionProviders, specv2.ExtensionProviders)
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
				Version:            ver,
				ExtensionProviders: []*v2.ExtensionProviderConfig{},
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
				ExtensionProviders: []*v2.ExtensionProviderConfig{
					{
						Name:       "prometheus",
						Prometheus: &v2.ExtensionProviderPrometheusConfig{},
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
