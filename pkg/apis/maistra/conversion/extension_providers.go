package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateExtensionProvidersValues(in *v2.ControlPlaneSpec, allValues map[string]interface{}) error {
	if in.ExtensionProviders == nil {
		return nil
	}

	var extensionProvidersValues []map[string]interface{}
	for _, ext := range in.ExtensionProviders {
		if ext.Prometheus != nil {
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":       ext.Name,
				"prometheus": map[string]interface{}{},
			})
		}
		if ext.EnvoyExtAuthzHTTP != nil {
			extAuthz := ext.EnvoyExtAuthzHTTP
			values := map[string]interface{}{
				"service": extAuthz.Service,
				"port":    extAuthz.Port,
			}
			if extAuthz.Timeout != nil {
				values["timeout"] = *extAuthz.Timeout
			}
			if extAuthz.PathPrefix != nil {
				values["pathPrefix"] = *extAuthz.PathPrefix
			}
			if extAuthz.FailOpen != nil {
				values["failOpen"] = *extAuthz.FailOpen
			}
			if extAuthz.StatusOnError != nil {
				values["statusOnError"] = *extAuthz.StatusOnError
			}
			if extAuthz.IncludeRequestHeadersInCheck != nil {
				values["includeRequestHeadersInCheck"] = stringToInterfaceArray(extAuthz.IncludeRequestHeadersInCheck)
			}
			if extAuthz.IncludeAdditionalHeadersInCheck != nil {
				values["includeAdditionalHeadersInCheck"] = mapOfStringToInterface(extAuthz.IncludeAdditionalHeadersInCheck)
			}
			if extAuthz.IncludeRequestBodyInCheck != nil {
				includeRequestBodyInCheckValues := map[string]interface{}{}
				if extAuthz.IncludeRequestBodyInCheck.MaxRequestBytes != nil {
					includeRequestBodyInCheckValues["maxRequestBytes"] = *extAuthz.IncludeRequestBodyInCheck.MaxRequestBytes
				}
				if extAuthz.IncludeRequestBodyInCheck.AllowPartialMessage != nil {
					includeRequestBodyInCheckValues["allowPartialMessage"] = *extAuthz.IncludeRequestBodyInCheck.AllowPartialMessage
				}
				if extAuthz.IncludeRequestBodyInCheck.PackAsBytes != nil {
					includeRequestBodyInCheckValues["packAsBytes"] = *extAuthz.IncludeRequestBodyInCheck.PackAsBytes
				}
				values["includeRequestBodyInCheck"] = includeRequestBodyInCheckValues
			}
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":              ext.Name,
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
	rawProviders, ok, err := in.GetSlice("meshConfig.extensionProviders")
	if err != nil {
		return err
	} else if !ok {
		return nil
	}

	out.ExtensionProviders = []*v2.ExtensionProviderConfig{}
	for _, rawProvider := range rawProviders {
		if provider, ok := rawProvider.(map[string]interface{}); ok {
			if _, ok := provider["prometheus"]; ok {
				out.ExtensionProviders = append(out.ExtensionProviders, &v2.ExtensionProviderConfig{
					Name:       provider["name"].(string),
					Prometheus: &v2.ExtensionProviderPrometheusConfig{},
				})
			}
			if rawExtAuthz, ok := provider["envoyExtAuthzHttp"]; ok {
				if extAuthz, ok := rawExtAuthz.(map[string]interface{}); ok {
					extAuthzConfig := &v2.ExtensionProviderEnvoyExternalAuthorizationHTTPConfig{
						Service: extAuthz["service"].(string),
						Port:    extAuthz["port"].(int64),
					}
					if rawTimeout, ok := extAuthz["timeout"]; ok {
						extAuthzConfig.Timeout = strPtr(rawTimeout.(string))
					}
					if rawPathPrefix, ok := extAuthz["pathPrefix"]; ok {
						extAuthzConfig.PathPrefix = strPtr(rawPathPrefix.(string))
					}
					if rawFailOpen, ok := extAuthz["failOpen"]; ok {
						extAuthzConfig.FailOpen = boolPtr(rawFailOpen.(bool))
					}
					if statusOnError, ok := extAuthz["statusOnError"]; ok {
						extAuthzConfig.StatusOnError = strPtr(statusOnError.(string))
					}
					if rawIncludeRequestHeadersInCheck, ok := extAuthz["includeRequestHeadersInCheck"]; ok {
						extAuthzConfig.IncludeRequestHeadersInCheck = interfaceToStringArray(rawIncludeRequestHeadersInCheck.([]interface{}))
					}
					if rawIncludeAdditionalHeadersInCheck, ok := extAuthz["includeAdditionalHeadersInCheck"]; ok {
						extAuthzConfig.IncludeAdditionalHeadersInCheck = mapOfInterfaceToString(rawIncludeAdditionalHeadersInCheck.(map[string]interface{}))
					}
					if rawIncludeRequestBodyInCheck, ok := extAuthz["includeRequestBodyInCheck"]; ok {
						if includeRequestBodyInCheck, ok := rawIncludeRequestBodyInCheck.(map[string]interface{}); ok {
							extAuthzConfig.IncludeRequestBodyInCheck = &v2.ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig{}
							if maxRequestBytes, ok := includeRequestBodyInCheck["maxRequestBytes"]; ok {
								extAuthzConfig.IncludeRequestBodyInCheck.MaxRequestBytes = int64Ptr(maxRequestBytes.(int64))
							}
							if allowPartialMessage, ok := includeRequestBodyInCheck["allowPartialMessage"]; ok {
								extAuthzConfig.IncludeRequestBodyInCheck.AllowPartialMessage = boolPtr(allowPartialMessage.(bool))
							}
							if packAsBytes, ok := includeRequestBodyInCheck["packAsBytes"]; ok {
								extAuthzConfig.IncludeRequestBodyInCheck.PackAsBytes = boolPtr(packAsBytes.(bool))
							}
						}
					}
					out.ExtensionProviders = append(out.ExtensionProviders, &v2.ExtensionProviderConfig{
						Name:              provider["name"].(string),
						EnvoyExtAuthzHTTP: extAuthzConfig,
					})
				}
			}
		}
	}

	return nil
}
