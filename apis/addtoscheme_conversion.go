package apis

import "github.com/maistra/istio-operator/apis/maistra/conversion"

func init() {
	AddToSchemes = append(AddToSchemes, conversion.SchemeBuilder.AddToScheme)
}
