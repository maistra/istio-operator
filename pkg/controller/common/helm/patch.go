package helm

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/util"

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
func (p *PatchFactory) CreatePatch(current, new *unstructured.Unstructured) (Patch, error) {
	return &basicPatch{client: p.client, oldObj: current, newObj: new}, nil
}

type basicPatch struct {
	client client.Client
	oldObj *unstructured.Unstructured
	newObj *unstructured.Unstructured
}

func (p *basicPatch) Apply(ctx context.Context) (*unstructured.Unstructured, error) {
	var patch client.Patch
	if originalBytes, err := util.GetOriginalConfiguration(p.oldObj); err == nil && len(originalBytes) > 0 {
		if originalObj, _, err := unstructured.UnstructuredJSONScheme.Decode(originalBytes, nil, nil); err == nil {
			patch = client.MergeFrom(originalObj)
		}
	}
	if patch == nil {
		// try merging with the existing
		// this isn't ideal, but is more robust
		patch = client.MergeFrom(p.oldObj)
	}
	if err := p.client.Patch(ctx, p.newObj, patch, client.FieldOwner("istio-operator")); err != nil {
		return nil, err
	}
	return p.newObj, nil
}
