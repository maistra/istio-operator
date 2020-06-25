package conversion

import (
	"k8s.io/apimachinery/pkg/runtime"
)

var (
    SchemeBuilder = runtime.NewSchemeBuilder()
	localSchemeBuilder = &SchemeBuilder
)
