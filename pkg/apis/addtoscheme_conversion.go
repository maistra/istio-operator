package apis

import "github.com/maistra/istio-operator/pkg/apis/maistra/conversion"

func init() {
    AddToSchemes = append(AddToSchemes, conversion.SchemeBuilder.AddToScheme)
}