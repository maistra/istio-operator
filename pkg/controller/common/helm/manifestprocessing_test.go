package helm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	testCases := []struct {
		name                            string
		admissionReviewVersions         []string
		expectedAdmissionReviewVersions []string
		sideEffects                     *v1beta1.SideEffectClass
		expectedSideEffects             *v1.SideEffectClass
	}{
		{
			name:                            "add-admissionReviewVersions",
			admissionReviewVersions:         nil,
			expectedAdmissionReviewVersions: []string{"v1beta1"},
			sideEffects:                     v1beta1SideEffectClassPtr(v1beta1.SideEffectClassNone),
			expectedSideEffects:             v1SideEffectClassPtr(v1.SideEffectClassNone),
		},
		{
			name:                            "preserve-existing-admissionReviewVersions",
			admissionReviewVersions:         []string{"v1"},
			expectedAdmissionReviewVersions: []string{"v1"},
			sideEffects:                     v1beta1SideEffectClassPtr(v1beta1.SideEffectClassNone),
			expectedSideEffects:             v1SideEffectClassPtr(v1.SideEffectClassNone),
		},
		{
			name:                            "add-sideEffects",
			admissionReviewVersions:         []string{"v1beta1"},
			expectedAdmissionReviewVersions: []string{"v1beta1"},
			sideEffects:                     nil,
			expectedSideEffects:             v1SideEffectClassPtr(v1.SideEffectClassNone),
		},
		{
			name:                            "preserve-existing-sideEffects",
			admissionReviewVersions:         []string{"v1beta1"},
			expectedAdmissionReviewVersions: []string{"v1beta1"},
			sideEffects:                     v1beta1SideEffectClassPtr(v1beta1.SideEffectClassNoneOnDryRun),
			expectedSideEffects:             v1SideEffectClassPtr(v1.SideEffectClassNoneOnDryRun),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			input := v1beta1.MutatingWebhookConfiguration{
				TypeMeta: metav1.TypeMeta{
					Kind:       "MutatingWebhookConfiguration",
					APIVersion: "admissionregistration.k8s.io/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "istiod-webhook",
				},
				Webhooks: []v1beta1.MutatingWebhook{
					{
						Name:                    "sidecar-injector.istio.io",
						AdmissionReviewVersions: tc.admissionReviewVersions,
						SideEffects:             tc.sideEffects,
					},
				},
			}

			expected := v1.MutatingWebhookConfiguration{
				TypeMeta: metav1.TypeMeta{
					Kind:       "MutatingWebhookConfiguration",
					APIVersion: "admissionregistration.k8s.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "istiod-webhook",
				},
				Webhooks: []v1.MutatingWebhook{
					{
						Name:                    "sidecar-injector.istio.io",
						AdmissionReviewVersions: tc.expectedAdmissionReviewVersions,
						SideEffects:             tc.expectedSideEffects,
					},
				},
			}

			obj := toUnstructured(t, &input)
			expectedObj := toUnstructured(t, &expected)

			converted, err := convertWebhookConfigurationFromV1beta1ToV1(obj)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			assert.DeepEquals(converted, expectedObj, "Converted object does not match expectation", t)
		})
	}
}

func toUnstructured(t *testing.T, input runtime.Object) *unstructured.Unstructured {
	inputJSON, err := json.Marshal(input)
	assert.Success(err, "json.Marshal", t)

	obj := &unstructured.Unstructured{}
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(inputJSON, nil, obj)
	if err != nil {
		t.Fatalf("Could not decode object: %v", err)
	}
	return obj
}

func v1beta1SideEffectClassPtr(s v1beta1.SideEffectClass) *v1beta1.SideEffectClass {
	return &s
}

func v1SideEffectClassPtr(s v1.SideEffectClass) *v1.SideEffectClass {
	return &s
}
