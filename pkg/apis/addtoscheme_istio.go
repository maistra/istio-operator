package apis

import (
	"github.com/maistra/istio-operator/pkg/apis/istio/simple/config/v1alpha2"
	"github.com/maistra/istio-operator/pkg/apis/istio/simple/networking/v1alpha3"
	"github.com/maistra/istio-operator/pkg/apis/istio/simple/security/v1beta1"
)

func init() {
	AddToSchemes = append(AddToSchemes,
        v1alpha2.SchemeBuilder.AddToScheme,
        v1alpha3.SchemeBuilder.AddToScheme,
		v1beta1.SchemeBuilder.AddToScheme,
	)
}
