package helm

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PatchFactory wraps the objects needed to create Patch objects.
type PatchFactory struct {
	client client.Client
}

// Patch represents a "patch" for an object
type Patch interface {
	Apply(ctx context.Context) (*unstructured.Unstructured, error)
}

// NewPatchFactory creates a new PatchFactory
func NewPatchFactory(k8sClient client.Client) *PatchFactory {
	return &PatchFactory{client: k8sClient}
}

// CreatePatch creates a patch based on the current and new versions of an object
func (p *PatchFactory) CreatePatch(current, new runtime.Object) (Patch, error) {
	return &basicPatch{client: p.client, oldObj: current, newObj: new}, nil
}

type basicPatch struct {
	client client.Client
	oldObj runtime.Object
	newObj runtime.Object
}

func (p *basicPatch) Apply(ctx context.Context) (*unstructured.Unstructured, error) {
	if err := p.client.Patch(ctx, p.newObj, client.Merge, client.FieldOwner("istio-operator")); err != nil {
		return nil, err
	}
	if newUnstructured, ok := p.newObj.(*unstructured.Unstructured); ok {
		return newUnstructured, nil
	}
	return nil, fmt.Errorf("could not decode unstructured object:\n%v", p.newObj)
}
