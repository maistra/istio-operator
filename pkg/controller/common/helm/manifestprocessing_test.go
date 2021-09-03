package helm

import (
	"context"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/releaseutil"
)

func TestEmptyYAMLBlocks(t *testing.T) {
	manifest := manifest.Manifest{
		Name: "bad.yaml",
		Content: `
---

# comment section

--- 

  ---
`,
		Head: &releaseutil.SimpleHead{},
	}

	processor := NewManifestProcessor(common.ControllerResources{}, &PatchFactory{}, "app", "version", types.NamespacedName{}, nil, nil, nil)

	_, err := processor.ProcessManifest(context.TODO(), manifest, "bad")

    if len(err) > 0 {
        t.Errorf("expected empty yaml blocks to process without error")
    }
}

func TestConvertWebhookConfigurationFromV1beta1ToV1(t *testing.T) {
	original := `apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingWebhookConfiguration
metadata:
  name: istiod-webhook
webhooks:
- clientConfig:
    service:
      name: istiod-minimal
      namespace: istio-system
      path: /inject
      port: 443
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: sidecar-injector.istio.io
  namespaceSelector:
    matchExpressions:
    - key: maistra.io/member-of
      operator: In
      values:
      - istio-system
    - key: maistra.io/ignore-namespace
      operator: DoesNotExist
    - key: istio-injection
      operator: NotIn
      values:
      - disabled
    - key: istio-env
      operator: DoesNotExist
  objectSelector: {}
  reinvocationPolicy: Never
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
`
	expected := `apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: istiod-webhook
webhooks:
- admissionReviewVersions:
  - v1beta1
  clientConfig:
    service:
      name: istiod-minimal
      namespace: istio-system
      path: /inject
      port: 443
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: sidecar-injector.istio.io
  namespaceSelector:
    matchExpressions:
    - key: maistra.io/member-of
      operator: In
      values:
      - istio-system
    - key: maistra.io/ignore-namespace
      operator: DoesNotExist
    - key: istio-injection
      operator: NotIn
      values:
      - disabled
    - key: istio-env
      operator: DoesNotExist
  objectSelector: {}
  reinvocationPolicy: Never
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
`

	obj := toUnstructured(t, original)
	expectedObj := toUnstructured(t, expected)

	converted, err := convertWebhookConfigurationFromV1beta1ToV1(obj)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	assert.DeepEquals(converted, expectedObj, "Converted object does not match expectation", t)
}

func toUnstructured(t *testing.T, original string) *unstructured.Unstructured {
	rawJSON, err := yaml.YAMLToJSON([]byte(original))
	if err != nil {
		t.Fatalf("Could not convert YAML to JSON: %v", err)
	}

	obj := &unstructured.Unstructured{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(rawJSON, nil, obj)
	if err != nil {
		t.Fatalf("Could not decode object: %v", err)
	}
	return obj
}