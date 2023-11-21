package helm

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOwnerReferencePostRenderer(t *testing.T) {
	postRenderer := OwnerReferencePostRenderer{
		ownerReference: metav1.OwnerReference{
			APIVersion: "operator.istio.io/v1alpha1",
			Kind:       "Istio",
			Name:       "my-istio",
			UID:        "123",
		},
		ownerNamespace: "istio-system",
	}

	input := `---
apiVersion: v1
kind: Deployment
metadata:
  name: deployment-in-same-namespace
  namespace: istio-system
spec:
  replicas: 1
---
# some comment
# there's no object here
---
apiVersion: v1
kind: Service
metadata:
  name: service-in-different-namespace
  namespace: other-namespace
spec:
  ports:
  - port: 80
`

	expected := `apiVersion: v1
kind: Deployment
metadata:
  name: deployment-in-same-namespace
  namespace: istio-system
  ownerReferences:
    - apiVersion: operator.istio.io/v1alpha1
      kind: Istio
      name: my-istio
      uid: "123"
spec:
  replicas: 1
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    operator-sdk/primary-resource: istio-system/my-istio
    operator-sdk/primary-resource-type: Istio.operator.istio.io
  name: service-in-different-namespace
  namespace: other-namespace
spec:
  ports:
    - port: 80
`

	actual, err := postRenderer.Run(bytes.NewBufferString(input))
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(expected, actual.String()); diff != "" {
		t.Errorf("ownerReference wasn't added properly; diff (-expected, +actual):\n%v", diff)
	}
}
