package apis

import "github.com/maistra/istio-operator/apis/external/kiali/v1alpha1"

func init() {
	AddToSchemes = append(AddToSchemes,
		v1alpha1.SchemeBuilder.AddToScheme,
	)
}
