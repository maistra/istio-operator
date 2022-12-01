package apis

import v1 "github.com/maistra/istio-operator/apis/external/jaeger/v1"

func init() {
	AddToSchemes = append(AddToSchemes,
		v1.SchemeBuilder.AddToScheme,
	)
}
