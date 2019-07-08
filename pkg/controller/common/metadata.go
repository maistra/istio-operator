package common

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// MetadataNamespace is the namespace for service mesh metadata (labels, annotations)
	MetadataNamespace = "maistra.io"

	// OwnerKey represents the mesh (namespace) to which the resource relates
	OwnerKey = MetadataNamespace + "/owner"

	// MemberOfKey represents the mesh (namespace) to which the resource relates
	MemberOfKey       = MetadataNamespace + "/member-of"
	LegacyMemberOfKey = "istio.openshift.io/member-of"

	// GenerationKey represents the generation to which the resource was last reconciled
	GenerationKey = MetadataNamespace + "/generation"

	// MeshGenerationKey represents the generation of the service mesh to which the resource was last reconciled
	MeshGenerationKey = MetadataNamespace + "/mesh-generation"

	// InternalKey is used to identify the resource as being internal to the mesh itself (i.e. should not be applied to members)
	InternalKey = MetadataNamespace + "/internal"
)

func FetchOwnedResources(kubeClient client.Client, gvk schema.GroupVersionKind, owner, namespace string) (*unstructured.UnstructuredList, error) {
	labelSelector := map[string]string{OwnerKey: owner}
	objects := &unstructured.UnstructuredList{}
	objects.SetGroupVersionKind(gvk)
	err := kubeClient.List(context.TODO(), client.MatchingLabels(labelSelector).InNamespace(namespace), objects)
	return objects, err
}

func FetchMeshResources(kubeClient client.Client, gvk schema.GroupVersionKind, mesh, namespace string) (*unstructured.UnstructuredList, error) {
	labelSelector := map[string]string{MemberOfKey: mesh}
	objects := &unstructured.UnstructuredList{}
	objects.SetGroupVersionKind(gvk)
	err := kubeClient.List(context.TODO(), client.MatchingLabels(labelSelector).InNamespace(namespace), objects)
	return objects, err
}
