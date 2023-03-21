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
		if ext.Prometheus == nil && ext.EnvoyExtAuthzHttp == nil {
			return fmt.Errorf("extension provider %s does not define any provider - it must specify one of: prometheus or envoyExtAuthzHttp", ext.Name)
		}
		if ext.Prometheus != nil && ext.EnvoyExtAuthzHttp != nil {
			return fmt.Errorf("extension provider %s must specify only one type of provider: prometheus or envoyExtAuthzHttp", ext.Name)
		}
		if ext.Prometheus != nil {
			if err := populateExtensionProviderPrometheusValues(ext, values); err != nil {
				return err
			}
		}
		if ext.EnvoyExtAuthzHttp != nil {
			if err := populateExtensionProviderEnvoyExtAuthzHttpValues(ext, values); err != nil {
				return err
			}
		}
	}
	return nil
}

func populateExtensionProviderPrometheusValues(ext *v2.ExtensionProviderConfig, values map[string]interface{}) error {
	prometheus := []map[string]interface{}{
		{
			"name":       ext.Name,
			"prometheus": map[string]interface{}{},
		},
	}
	if err := setHelmMapSliceValue(values, "meshConfig.extensionProviders", prometheus); err != nil {
		return err
	}
	return nil
}

func populateExtensionProviderEnvoyExtAuthzHttpValues(ext *v2.ExtensionProviderConfig, values map[string]interface{}) error {
	envoyExtAuthzHttp := []map[string]interface{}{
		{
			"name": ext.Name,
			"envoyExtAuthzHttp": map[string]interface{}{
				// TODO: Handle empty string and 0
				"service": ext.EnvoyExtAuthzHttp.Service,
				"port":    ext.EnvoyExtAuthzHttp.Port,
			},
		},
	}
	if err := setHelmMapSliceValue(values, "meshConfig.extensionProviders", envoyExtAuthzHttp); err != nil {
		return err
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
				if _, ok := extProvider["prometheus"]; ok {
					out.ExtensionProviders = append(out.ExtensionProviders, &v2.ExtensionProviderConfig{
						Name:       extProvider["name"].(string),
						Prometheus: &v2.ExtensionProviderPrometheusConfig{},
					})
				}
				if rawEnvoyExtAuthzHttp, ok := extProvider["envoyExtAuthzHttp"]; ok {
					if envoyExtAuthzHttp, ok := rawEnvoyExtAuthzHttp.(map[string]interface{}); ok {
						out.ExtensionProviders = append(out.ExtensionProviders, &v2.ExtensionProviderConfig{
							Name: extProvider["name"].(string),
							EnvoyExtAuthzHttp: &v2.ExtensionProviderEnvoyExternalAuthorizationHttpConfig{
								// TODO: handle wrong types safely
								Service: envoyExtAuthzHttp["service"].(string),
								Port:    envoyExtAuthzHttp["port"].(int64),
							},
						})
					}
				}
			}
		}
	} else if err != nil {
		return err
	}

	return nil
}
