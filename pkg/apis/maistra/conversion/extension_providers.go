package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateExtensionProvidersValues(in *v2.ControlPlaneSpec, allValues map[string]interface{}) error {
	if in.MeshConfig == nil || in.MeshConfig.ExtensionProviders == nil {
		return nil
	}

	var extensionProvidersValues []map[string]interface{}
	for _, provider := range in.MeshConfig.ExtensionProviders {
		if provider.Prometheus != nil {
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":       provider.Name,
				"prometheus": map[string]interface{}{},
			})
		}
		if provider.EnvoyExtAuthzHTTP != nil {
			config := provider.EnvoyExtAuthzHTTP
			values := map[string]interface{}{
				"service": config.Service,
				"port":    config.Port,
			}
			if config.Timeout != nil {
				values["timeout"] = *config.Timeout
			}
			if config.PathPrefix != nil {
				values["pathPrefix"] = *config.PathPrefix
			}
			if config.FailOpen != nil {
				values["failOpen"] = *config.FailOpen
			}
			if config.StatusOnError != nil {
				values["statusOnError"] = *config.StatusOnError
			}
			if config.IncludeRequestHeadersInCheck != nil {
				values["includeRequestHeadersInCheck"] = stringToInterfaceArray(config.IncludeRequestHeadersInCheck)
			}
			if config.IncludeAdditionalHeadersInCheck != nil {
				values["includeAdditionalHeadersInCheck"] = mapOfStringToInterface(config.IncludeAdditionalHeadersInCheck)
			}
			if config.IncludeRequestBodyInCheck != nil {
				includeRequestBodyInCheckValues := map[string]interface{}{}
				if config.IncludeRequestBodyInCheck.MaxRequestBytes != nil {
					includeRequestBodyInCheckValues["maxRequestBytes"] = *config.IncludeRequestBodyInCheck.MaxRequestBytes
				}
				if config.IncludeRequestBodyInCheck.AllowPartialMessage != nil {
					includeRequestBodyInCheckValues["allowPartialMessage"] = *config.IncludeRequestBodyInCheck.AllowPartialMessage
				}
				if config.IncludeRequestBodyInCheck.PackAsBytes != nil {
					includeRequestBodyInCheckValues["packAsBytes"] = *config.IncludeRequestBodyInCheck.PackAsBytes
				}
				values["includeRequestBodyInCheck"] = includeRequestBodyInCheckValues
			}
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":              provider.Name,
				"envoyExtAuthzHttp": values,
			})
		}
	}
	if err := setHelmMapSliceValue(allValues, "meshConfig.extensionProviders", extensionProvidersValues); err != nil {
		return err
	}
	return nil
}

func populateExtensionProvidersConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	rawProviders, exists, err := in.GetSlice("meshConfig.extensionProviders")
	if err != nil {
		return err
	} else if !exists {
		return nil
	}

	if out.MeshConfig == nil {
		out.MeshConfig = &v2.MeshConfig{}
	}
	out.MeshConfig.ExtensionProviders = []*v2.ExtensionProviderConfig{}
	for _, rawProvider := range rawProviders {
		if provider, ok := rawProvider.(map[string]interface{}); ok {
			if _, ok := provider["prometheus"]; ok {
				out.MeshConfig.ExtensionProviders = append(out.MeshConfig.ExtensionProviders, &v2.ExtensionProviderConfig{
					Name:       provider["name"].(string),
					Prometheus: &v2.ExtensionProviderPrometheusConfig{},
				})
			}
			if rawExtAuthz, ok := provider["envoyExtAuthzHttp"]; ok {
				if extAuthz, ok := rawExtAuthz.(map[string]interface{}); ok {
					config := &v2.ExtensionProviderEnvoyExternalAuthorizationHTTPConfig{
						Service: extAuthz["service"].(string),
						Port:    extAuthz["port"].(int64),
					}
					if rawTimeout, ok := extAuthz["timeout"]; ok {
						config.Timeout = strPtr(rawTimeout.(string))
					}
					if rawPathPrefix, ok := extAuthz["pathPrefix"]; ok {
						config.PathPrefix = strPtr(rawPathPrefix.(string))
					}
					if rawFailOpen, ok := extAuthz["failOpen"]; ok {
						config.FailOpen = boolPtr(rawFailOpen.(bool))
					}
					if statusOnError, ok := extAuthz["statusOnError"]; ok {
						config.StatusOnError = strPtr(statusOnError.(string))
					}
					if rawIncludeRequestHeadersInCheck, ok := extAuthz["includeRequestHeadersInCheck"]; ok {
						config.IncludeRequestHeadersInCheck = interfaceToStringArray(rawIncludeRequestHeadersInCheck.([]interface{}))
					}
					if rawIncludeAdditionalHeadersInCheck, ok := extAuthz["includeAdditionalHeadersInCheck"]; ok {
						config.IncludeAdditionalHeadersInCheck = mapOfInterfaceToString(rawIncludeAdditionalHeadersInCheck.(map[string]interface{}))
					}
					if rawIncludeRequestBodyInCheck, ok := extAuthz["includeRequestBodyInCheck"]; ok {
						if includeRequestBodyInCheck, ok := rawIncludeRequestBodyInCheck.(map[string]interface{}); ok {
							config.IncludeRequestBodyInCheck = &v2.ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig{}
							if maxRequestBytes, ok := includeRequestBodyInCheck["maxRequestBytes"]; ok {
								config.IncludeRequestBodyInCheck.MaxRequestBytes = int64Ptr(maxRequestBytes.(int64))
							}
							if allowPartialMessage, ok := includeRequestBodyInCheck["allowPartialMessage"]; ok {
								config.IncludeRequestBodyInCheck.AllowPartialMessage = boolPtr(allowPartialMessage.(bool))
							}
							if packAsBytes, ok := includeRequestBodyInCheck["packAsBytes"]; ok {
								config.IncludeRequestBodyInCheck.PackAsBytes = boolPtr(packAsBytes.(bool))
							}
						}
					}
					out.MeshConfig.ExtensionProviders = append(out.MeshConfig.ExtensionProviders, &v2.ExtensionProviderConfig{
						Name:              provider["name"].(string),
						EnvoyExtAuthzHTTP: config,
					})
				}
			}
		}
	}

	return nil
}
