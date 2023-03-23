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
	if rawExtProviders, ok, err := in.GetSlice("meshConfig.extensionProviders"); ok {
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
				if rawEnvoyExtAuthzHTTP, ok := extProvider["envoyExtAuthzHttp"]; ok {
					if envoyExtAuthzHTTP, ok := rawEnvoyExtAuthzHTTP.(map[string]interface{}); ok {
						extProviderName := extProvider["name"].(string)
						envoyExtAuthzHTTPConfig := &v2.ExtensionProviderEnvoyExternalAuthorizationHTTPConfig{
							Service: envoyExtAuthzHTTP["service"].(string),
							Port:    envoyExtAuthzHTTP["port"].(int64),
						}
						if rawTimeout, ok := envoyExtAuthzHTTP["timeout"]; ok {
							envoyExtAuthzHTTPConfig.Timeout = strPtr(rawTimeout.(string))
						}
						if rawPathPrefix, ok := envoyExtAuthzHTTP["pathPrefix"]; ok {
							envoyExtAuthzHTTPConfig.PathPrefix = strPtr(rawPathPrefix.(string))
						}
						if rawFailOpen, ok := envoyExtAuthzHTTP["failOpen"]; ok {
							envoyExtAuthzHTTPConfig.FailOpen = boolPtr(rawFailOpen.(bool))
						}
						if statusOnError, ok := envoyExtAuthzHTTP["statusOnError"]; ok {
							envoyExtAuthzHTTPConfig.StatusOnError = strPtr(statusOnError.(string))
						}
						if rawIncludeRequestHeadersInCheck, ok := envoyExtAuthzHTTP["includeRequestHeadersInCheck"]; ok {
							envoyExtAuthzHTTPConfig.IncludeRequestHeadersInCheck = interfaceToStringArray(rawIncludeRequestHeadersInCheck.([]interface{}))
						}
						if rawIncludeAdditionalHeadersInCheck, ok := envoyExtAuthzHTTP["includeAdditionalHeadersInCheck"]; ok {
							envoyExtAuthzHTTPConfig.IncludeAdditionalHeadersInCheck = mapOfInterfaceToString(rawIncludeAdditionalHeadersInCheck.(map[string]interface{}))
						}
						if rawIncludeRequestBodyInCheck, ok := envoyExtAuthzHTTP["includeRequestBodyInCheck"]; ok {
							if includeRequestBodyInCheck, ok := rawIncludeRequestBodyInCheck.(map[string]interface{}); ok {
								envoyExtAuthzHTTPConfig.IncludeRequestBodyInCheck = &v2.ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig{}
								if maxRequestBytes, ok := includeRequestBodyInCheck["maxRequestBytes"]; ok {
									envoyExtAuthzHTTPConfig.IncludeRequestBodyInCheck.MaxRequestBytes = int64Ptr(maxRequestBytes.(int64))
								}
								if allowPartialMessage, ok := includeRequestBodyInCheck["allowPartialMessage"]; ok {
									envoyExtAuthzHTTPConfig.IncludeRequestBodyInCheck.AllowPartialMessage = boolPtr(allowPartialMessage.(bool))
								}
								if packAsBytes, ok := includeRequestBodyInCheck["packAsBytes"]; ok {
									envoyExtAuthzHTTPConfig.IncludeRequestBodyInCheck.PackAsBytes = boolPtr(packAsBytes.(bool))
								}
							}
						}
						out.ExtensionProviders = append(out.ExtensionProviders, &v2.ExtensionProviderConfig{
							Name:              extProviderName,
							EnvoyExtAuthzHTTP: envoyExtAuthzHTTPConfig,
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
