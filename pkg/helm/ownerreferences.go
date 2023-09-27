package helm

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/resource"
)

const (
	AnnotationPrimaryResource     = "operator-sdk/primary-resource"
	AnnotationPrimaryResourceType = "operator-sdk/primary-resource-type"
)

// addOwnerReferenceVisitor returns a visitor function that adds the specified
// OwnerReference to each resource it visits
func addOwnerReferenceVisitor(ownerReference metav1.OwnerReference, istioNamespace string) resource.VisitorFunc {
	return func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		objMeta, err := meta.Accessor(info.Object)
		if err != nil {
			return err
		}

		if objMeta.GetNamespace() == istioNamespace {
			objMeta.SetOwnerReferences([]metav1.OwnerReference{ownerReference})
		} else {
			annotations := objMeta.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			ownerAPIGroup, _, _ := strings.Cut(ownerReference.APIVersion, "/")
			annotations[AnnotationPrimaryResource] = istioNamespace + "/" + ownerReference.Name
			annotations[AnnotationPrimaryResourceType] = ownerReference.Kind + "." + ownerAPIGroup
			objMeta.SetAnnotations(annotations)
		}
		return nil
	}
}

func GetOwnerFromAnnotations(annotations map[string]string) (*types.NamespacedName, string, string) {
	if annotations == nil {
		return nil, "", ""
	}
	primaryResource := annotations[AnnotationPrimaryResource]
	primaryResourceType := annotations[AnnotationPrimaryResourceType]

	if primaryResource == "" || primaryResourceType == "" {
		return nil, "", ""
	}
	nameParts := strings.Split(primaryResource, "/")
	typeParts := strings.SplitN(primaryResourceType, ".", 2)

	if len(nameParts) != 2 || len(typeParts) != 2 {
		return nil, "", ""
	}
	return &types.NamespacedName{
		Namespace: nameParts[0],
		Name:      nameParts[1],
	}, typeParts[0], typeParts[1]
}
