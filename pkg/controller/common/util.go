package common

import (
	"strconv"

	"github.com/go-logr/logr"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceManager struct {
	Client       client.Client
	PatchFactory *PatchFactory
	Log          logr.Logger
}

func IndexOf(l []string, s string) int {
	for i, elem := range l {
		if elem == s {
			return i
		}
	}
	return -1
}

func HasLabel(resource metav1.Object, label string) bool {
	labels := resource.GetLabels()
	if labels == nil {
		return false
	}
	_, ok := labels[label]
	return ok
}

func DeleteLabel(resource metav1.Object, label string) {
	labels := resource.GetLabels()
	if labels == nil {
		return
	}
	delete(labels, label)
	resource.SetLabels(labels)
}

func SetLabel(resource metav1.Object, label, value string) {
	labels := resource.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[label] = value
	resource.SetLabels(labels)
}

func HasAnnotation(resource metav1.Object, annotation string) bool {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[annotation]
	return ok
}

func DeleteAnnotation(resource metav1.Object, annotation string) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		return
	}
	delete(annotations, annotation)
	resource.SetAnnotations(annotations)
}

func GetAnnotation(resource metav1.Object, annotation string) (value string, ok bool) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	value, ok = annotations[annotation]
	return
}

func SetAnnotation(resource metav1.Object, annotation, value string) {
	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[annotation] = value
	resource.SetAnnotations(annotations)
}

func IsMeshMultitenant(mesh *maistrav1.ServiceMeshControlPlane) bool {
	if mesh == nil {
		return false
	}
	if global, ok := mesh.Spec.Istio["global"]; ok {
		switch globalMap := global.(type) {
		case map[string]interface{}:
			if multitenant, ok := globalMap["multitenant"]; ok {
				switch flag := multitenant.(type) {
				case bool:
					return flag
				case string:
					if boolval, err := strconv.ParseBool(flag); err != nil {
						return boolval
					}
				}
			}
		}
	}
	return false
}
