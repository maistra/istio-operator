package common

const (
	// MetadataNamespace is the namespace for service mesh metadata (labels, annotations)
	MetadataNamespace = "operator.istio.io"

	// CreatedByKey is used in annotations to mark ServiceMeshMemberRolls created by the ServiceMeshMember controller
	CreatedByKey = MetadataNamespace + "/created-by"

	// OwnerKey represents the mesh (namespace) to which the resource relates
	OwnerKey = MetadataNamespace + "/owner"

	// OwnerNameKey represents the name of the SMCP to which the resource relates
	OwnerNameKey = MetadataNamespace + "/owner-name"

	// MemberOfKey represents the mesh (namespace) to which the resource relates
	MemberOfKey = MetadataNamespace + "/member-of"

	// IgnoreNamespaceKey indicates that sidecar injection should be disabled for the namespace
	IgnoreNamespaceKey = MetadataNamespace + "/ignore-namespace"

	// GenerationKey represents the generation to which the resource was last reconciled
	GenerationKey = MetadataNamespace + "/generation"

	// MeshGenerationKey represents the generation of the service mesh to which the resource was last reconciled.
	// This uniquely identifies an installation, incorporating the operator version and the smcp resource generation.
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

	// KubernetesAppPartOfValue is the KubernetesAppPartOfKey label value the operator sets on all objects it creates
	KubernetesAppPartOfValue = "istio"

	// KubernetesAppManagedByValue is the KubernetesAppManagedByKey label value the operator sets on all objects it creates
	KubernetesAppManagedByValue = "maistra-istio-operator"

	// MemberRollName is the only name we allow for ServiceMeshMemberRoll objects
	MemberRollName = "default"

	// MemberName is the only name we allow for ServiceMeshMember objects
	MemberName = "default"
)
