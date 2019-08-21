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
	MemberOfKey = MetadataNamespace + "/member-of"

	// GenerationKey represents the generation to which the resource was last reconciled
	GenerationKey = MetadataNamespace + "/generation"

	// MeshGenerationKey represents the generation of the service mesh to which the resource was last reconciled
	MeshGenerationKey = MetadataNamespace + "/mesh-generation"

	// InternalKey is used to identify the resource as being internal to the mesh itself (i.e. should not be applied to members)
	InternalKey = MetadataNamespace + "/internal"

	// FinalizerName is the finalizer name the controllers add to any resources that need to be finalized during deletion
	FinalizerName = MetadataNamespace + "/istio-operator"

	// KubernetesAppNamespace is the common namespace for application information
	KubernetesAppNamespace    = "app.kubernetes.io"
	KubernetesAppNameKey      = KubernetesAppNamespace + "/name"
	KubernetesAppInstanceKey  = KubernetesAppNamespace + "/instance"
	KubernetesAppVersionKey   = KubernetesAppNamespace + "/version"
	KubernetesAppComponentKey = KubernetesAppNamespace + "/component"
	KubernetesAppPartOfKey    = KubernetesAppNamespace + "/part-of"
	KubernetesAppManagedByKey = KubernetesAppNamespace + "/managed-by"
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
