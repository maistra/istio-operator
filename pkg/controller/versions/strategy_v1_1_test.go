package versions

import (
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type validateExtensionProviderTestCase struct {
	name           string
	values         string
	expectedErrors []string
}

func TestValidateExtensionProviders(t *testing.T) {
	v1_1 := &versionStrategyV1_1{Ver: V1_1}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			helmValues := &v1.HelmValues{}
			if err := helmValues.UnmarshalYAML([]byte(tc.values)); err != nil {
				t.Fatalf("failed to parse test data: %s", tc.values)
			}
			smcp := &v1.ServiceMeshControlPlane{
				Spec: v1.ControlPlaneSpec{
					Istio: helmValues,
				},
			}
			errs := v1_1.validateExtensionProviders(smcp, []error{})
			actualErrors := sets.NewString(errorsToStrings(errs)...)
			expectedErrors := sets.NewString(tc.expectedErrors...)
			if actualErrors.Difference(expectedErrors).Len() > 0 || expectedErrors.Difference(actualErrors).Len() > 0 {
				t.Fatalf("unexpected errors;\nexpected:\n%v\n\nactual:\n%v", tc.expectedErrors, errs)
			}
		})
	}
}

var testCases = []validateExtensionProviderTestCase{
	{
		name: "valid values",
		values: `
meshConfig:
  extensionProviders:
  - name: prometheus
    prometheus: {}
  - name: ext-authz-http,
    envoyExtAuthzHttp:
      service: ext-authz.foo.svc.cluster.local,
      port: 8000,
      timeout: 1s
`,
	},
	{
		name: "invalid structure of extensionProviders",
		values: `
meshConfig:
  extensionProviders:
    key: value
`,
		expectedErrors: []string{"failed to parse 'meshConfig.extensionProviders': .meshConfig.extensionProviders accessor error: " +
			"map[key:value] is of the type map[string]interface {}, expected []interface{}"},
	},
	{
		name: "missing name",
		values: `
meshConfig:
  extensionProviders:
  - prometheus: {}
`,
		expectedErrors: []string{"extension providers must specify name"},
	},
	{
		name: "empty name",
		values: `
meshConfig:
  extensionProviders:
  - name: ""
    prometheus: {}
`,
		expectedErrors: []string{"extension provider name cannot be empty"},
	},
	{
		name: "no provider",
		values: `
meshConfig:
  extensionProviders:
  - name: no-provider
`,
		expectedErrors: []string{
			"extension provider no-provider does not define any provider - it must specify one of: prometheus or envoyExtAuthzHttp",
		},
	},
	{
		name: "too many provider types",
		values: `
meshConfig:
  extensionProviders:
  - name: to-many-provider-types
    prometheus: {}
    envoyExtAuthzHttp:
      service: ext-authz.foo.svc.cluster.local,
      port: 8000,
`,
		expectedErrors: []string{
			"extension provider 'to-many-provider-types' must specify only one type of provider: prometheus or envoyExtAuthzHttp",
		},
	},
	{
		name: "invalid timeout",
		values: `
meshConfig:
  extensionProviders:
  - name: invalid-timeout
    envoyExtAuthzHttp:
      service: ext-authz.foo.svc.cluster.local,
      port: 8000,
      timeout: 1sec
`,
		expectedErrors: []string{
			"invalid extension provider 'invalid-timeout': envoyExtAuthzHttp.timeout must be specified in the duration format - got '1sec'",
		},
	},
}
