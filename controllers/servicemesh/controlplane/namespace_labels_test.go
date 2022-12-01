package controlplane

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/maistra/istio-operator/controllers/common/test"
	"github.com/maistra/istio-operator/controllers/common/test/assert"
)

func TestSetNamespaceLabels(t *testing.T) {
	cases := []struct {
		name           string
		existingLabels map[string]string
		newLabels      map[string]string
		expectedLabels map[string]string
		expectNoAction bool
	}{
		{
			name:           "add-label",
			existingLabels: map[string]string{"foo": "foo-value"},
			newLabels:      map[string]string{"bar": "bar-value"},
			expectedLabels: map[string]string{"foo": "foo-value", "bar": "bar-value"},
		},
		{
			name:           "no-change",
			existingLabels: map[string]string{"foo": "foo-value"},
			newLabels:      map[string]string{"foo": "foo-value"},
			expectNoAction: true,
		},
		{
			name:           "remove-label",
			existingLabels: map[string]string{"foo": "foo-value", "bar": "bar-value"},
			newLabels:      map[string]string{"foo": ""},
			expectedLabels: map[string]string{"bar": "bar-value"},
		},
		{
			name:           "remove-last-label",
			existingLabels: map[string]string{"foo": "foo-value"},
			newLabels:      map[string]string{"foo": ""},
			expectedLabels: nil,
		},
		{
			name:           "remove-nonexisting-label",
			existingLabels: map[string]string{"foo": "foo-value"},
			newLabels:      map[string]string{"bar": ""},
			expectNoAction: true,
		},
		{
			name:           "overwrite-label",
			existingLabels: map[string]string{"foo": "old-value"},
			newLabels:      map[string]string{"foo": "new-value"},
			expectedLabels: map[string]string{"foo": "new-value"},
		},
		{
			name:           "nil-existing-labels",
			existingLabels: nil,
			newLabels:      map[string]string{"foo": "foo-value"},
			expectedLabels: map[string]string{"foo": "foo-value"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "my-namespace",
					Labels: tc.existingLabels,
				},
			}

			cl, tracker := test.CreateClient(ns)

			err := setNamespaceLabels(ctx, cl, "my-namespace", tc.newLabels)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tc.expectNoAction {
				test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
			} else {
				test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)

				ns = &corev1.Namespace{}
				test.GetObject(ctx, cl, types.NamespacedName{Namespace: "", Name: "my-namespace"}, ns)
				assert.DeepEquals(ns.Labels, tc.expectedLabels, "Unexpected namespace labels", t)
			}
		})
	}
}
