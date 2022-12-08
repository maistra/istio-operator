package common

import (
	admissionv1 "k8s.io/api/admission/v1"
	pkgtypes "k8s.io/apimachinery/pkg/types"
)

type NamespaceFilter string

func (f NamespaceFilter) Watching(namespace string) bool {
	return len(f) == 0 || namespace == string(f)
}

func ToNamespacedName(req *admissionv1.AdmissionRequest) pkgtypes.NamespacedName {
	return pkgtypes.NamespacedName{Namespace: req.Namespace, Name: req.Name}
}
