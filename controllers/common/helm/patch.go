package helm

import (
	"context"
	"fmt"

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
func (p *PatchFactory) CreatePatch(oldObj, newObj *unstructured.Unstructured) (Patch, error) {
	return &basicPatch{client: p.client, oldObj: oldObj, newObj: newObj}, nil
}

type basicPatch struct {
	client client.Client
	oldObj *unstructured.Unstructured
	newObj *unstructured.Unstructured
}

type routeNoHostError error

func IsRouteNoHostError(err error) bool {
	switch err.(type) {
	case routeNoHostError:
		return true
	}
	return false
}

func (p *basicPatch) Apply(ctx context.Context) (*unstructured.Unstructured, error) {
	if p.oldObj.GroupVersionKind().Group == "route.openshift.io" && p.oldObj.GroupVersionKind().Kind == "Route" &&
		!hasHostSet(p.newObj) {
		return nil, routeNoHostError(fmt.Errorf("spec.host not set on Route, need to recreate"))
	}

	var patch client.Patch
	if originalBytes, err := util.GetOriginalConfiguration(p.oldObj); err == nil && len(originalBytes) > 0 {
		if _, objGVK, err := unstructured.UnstructuredJSONScheme.Decode(originalBytes, nil, nil); err == nil {
			newObj := &unstructured.Unstructured{}
			newObj.SetGroupVersionKind(*objGVK)
			patch = client.MergeFrom(newObj)
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

func hasHostSet(route *unstructured.Unstructured) bool {
	val, found, _ := unstructured.NestedFieldNoCopy(route.Object, "spec", "host")
	if !found {
		return false
	}

	s, ok := val.(string)
	if !ok || s == "" {
		return false
	}
	return true
}
