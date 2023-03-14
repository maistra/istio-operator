package conversion

import (
	"fmt"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateExtensionProvidersValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	if in.ExtensionProviders == nil {
		return nil
	}

	if len(in.ExtensionProviders) == 0 {
		if err := setHelmMapSliceValue(values, "meshConfig.extensionProviders", []map[string]interface{}{}); err != nil {
			return err
		}
	}

	for _, ext := range in.ExtensionProviders {
		if ext.Prometheus == nil {
			return fmt.Errorf("extension provider entry %s does not define any provider - it must specify one of: prometheus", ext.Name)
		}
		prometheus := []map[string]interface{}{
			{
				"name":       ext.Name,
				"prometheus": map[string]interface{}{},
			},
		}
		if err := setHelmMapSliceValue(values, "meshConfig.extensionProviders", prometheus); err != nil {
			return err
		}
	}
	return nil
}

func populateExtensionProvidersConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	if rawExtProviders, ok, err := in.GetAndRemoveSlice("meshConfig.extensionProviders"); ok {
		if len(rawExtProviders) == 0 {
			out.ExtensionProviders = []*v2.ExtensionProviderConfig{}
		}
		for _, rawExtProvider := range rawExtProviders {
			if extProvider, ok := rawExtProvider.(map[string]interface{}); ok {
				out.ExtensionProviders = append(out.ExtensionProviders, &v2.ExtensionProviderConfig{
					Name:       extProvider["name"].(string),
					Prometheus: &v2.ExtensionProviderPrometheusConfig{},
				})
			}
		}
	} else if err != nil {
		return err
	}

	return nil
}
