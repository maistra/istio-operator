package v1

import (
	"fmt"
	"testing"

	"github.com/ghodss/yaml"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const smcpYaml = `apiVersion: maistra.io/v1
kind: ServiceMeshControlPlane
metadata:
  creationTimestamp: null
  name: minimal-install
  namespace: istio-system
spec:
  istio:
    foo:
      bar: baz
status:
  lastAppliedConfiguration:
    istio:
      foo:
        bar: baz
`

func TestUnmarshallHelmValuesInSMCP(t *testing.T) {
	smcp := &ServiceMeshControlPlane{}
	err := yaml.Unmarshal([]byte(smcpYaml), smcp)
	if err != nil {
		panic(fmt.Sprintf("Unexpected error: %v", err))
	}

	if foo, ok, _ := smcp.Spec.Istio.GetMap("foo"); !ok {
		t.Fatalf("SMCP.spec.istio.foo is not a map[string]interface{}")
	} else {
		if foo["bar"] != "baz" {
			t.Fatalf("Unexpected value of SMCP.spec.istio.foo.bar; expected: baz, actual: %v", foo["bar"])
		}
	}
}

func TestMarshallHelmValuesInSMCP(t *testing.T) {
	smcp := &ServiceMeshControlPlane{
		TypeMeta: v1.TypeMeta{
			Kind:       "ServiceMeshControlPlane",
			APIVersion: "maistra.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "minimal-install",
			Namespace: "istio-system",
		},
		Spec: ControlPlaneSpec{
			Istio: NewHelmValues(map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": "baz",
				},
			}),
		},
		Status: ControlPlaneStatus{
			LastAppliedConfiguration: ControlPlaneSpec{
				Istio: NewHelmValues(map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": "baz",
					},
				}),
			},
		},
	}
	actualYaml, err := yaml.Marshal(smcp)
	if err != nil {
		panic(fmt.Sprintf("Unexpected error: %v", err))
	}

	if string(actualYaml) != smcpYaml {
		t.Fatalf("Unexpected YAML;\nexpected:\n-%v-\n\nactual:\n-%v-", smcpYaml, string(actualYaml))
	}
}
