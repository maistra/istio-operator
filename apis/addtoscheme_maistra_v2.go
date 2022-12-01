package apis

import (
	v2 "github.com/maistra/istio-operator/apis/maistra/v2"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		v2.SchemeBuilder.AddToScheme,
	)
}
