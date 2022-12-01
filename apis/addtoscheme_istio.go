package apis

import (
	"github.com/maistra/istio-operator/apis/external/istio/config/v1alpha2"
	"github.com/maistra/istio-operator/apis/external/istio/networking/v1alpha3"
	"github.com/maistra/istio-operator/apis/external/istio/security/v1beta1"
)

func init() {
	AddToSchemes = append(AddToSchemes,
		v1alpha2.SchemeBuilder.AddToScheme,
		v1alpha3.SchemeBuilder.AddToScheme,
		v1beta1.SchemeBuilder.AddToScheme,
	)
}
