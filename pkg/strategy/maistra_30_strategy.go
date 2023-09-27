package strategy

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1 "maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/common"
)

type Maistra30Strategy struct{}

func (s *Maistra30Strategy) ApplyDefaults(istio *v1.Istio) error {
	values := istio.Spec.GetValues()
	if values == nil {
		values = make(map[string]interface{})
	}

	// TODO: move this to a profile or similar file
	err := setIfNotPresent(values, "global.istioNamespace", istio.Namespace)
	if err != nil {
		return err
	}
	err = setIfNotPresent(values, "istio_cni.enabled", true)
	if err != nil {
		return err
	}
	err = setIfNotPresent(values, "cni.privileged", true)
	if err != nil {
		return err
	}
	err = setIfNotPresent(values, "cni.image", common.Config.Images3_0.CNI)
	if err != nil {
		return err
	}
	err = setIfNotPresent(values, "pilot.image", common.Config.Images3_0.Istiod)
	if err != nil {
		return err
	}
	err = setIfNotPresent(values, "global.proxy.image", common.Config.Images3_0.Proxy)
	if err != nil {
		return err
	}
	err = setIfNotPresent(values, "global.proxy_init.image", common.Config.Images3_0.Proxy)
	if err != nil {
		return err
	}

	return istio.Spec.SetValues(values)
}

func setIfNotPresent(values map[string]interface{}, key string, value interface{}) error {
	keys := strings.Split(key, ".")
	_, found, err := unstructured.NestedString(values, keys...)
	if !found || err != nil {
		return unstructured.SetNestedField(values, value, keys...)
	}
	return nil
}
