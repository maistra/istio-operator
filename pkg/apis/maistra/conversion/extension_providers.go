package conversion

import (
	"fmt"
	"time"

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

	var extensionProvidersValues []map[string]interface{}
	for _, ext := range in.ExtensionProviders {
		if ext.Prometheus == nil && ext.EnvoyExtAuthzHTTP == nil {
			return fmt.Errorf("extension provider %s does not define any provider - it must specify one of: prometheus or envoyExtAuthzHttp", ext.Name)
		}
		if ext.Prometheus != nil && ext.EnvoyExtAuthzHTTP != nil {
			return fmt.Errorf("extension provider %s must specify only one type of provider: prometheus or envoyExtAuthzHttp", ext.Name)
		}
		if ext.Prometheus != nil {
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				// TODO: check empty string
				"name":       ext.Name,
				"prometheus": map[string]interface{}{},
			})
		}
		if ext.EnvoyExtAuthzHTTP != nil {
			envoyExtAuthzHTTPValues := map[string]interface{}{
				"service": ext.EnvoyExtAuthzHTTP.Service,
				"port":    ext.EnvoyExtAuthzHTTP.Port,
			}
			if ext.EnvoyExtAuthzHTTP.Timeout != nil {
				envoyExtAuthzHTTPValues["timeout"] = *ext.EnvoyExtAuthzHTTP.Timeout
			}
			if ext.EnvoyExtAuthzHTTP.PathPrefix != nil {
				envoyExtAuthzHTTPValues["pathPrefix"] = *ext.EnvoyExtAuthzHTTP.PathPrefix
			}
			if ext.EnvoyExtAuthzHTTP.FailOpen != nil {
				envoyExtAuthzHTTPValues["failOpen"] = *ext.EnvoyExtAuthzHTTP.FailOpen
			}
			if ext.EnvoyExtAuthzHTTP.StatusOnError != nil {
				envoyExtAuthzHTTPValues["statusOnError"] = *ext.EnvoyExtAuthzHTTP.StatusOnError
			}
			if ext.EnvoyExtAuthzHTTP.IncludeRequestHeadersInCheck != nil {
				envoyExtAuthzHTTPValues["includeRequestHeadersInCheck"] = stringToInterfaceArray(ext.EnvoyExtAuthzHTTP.IncludeRequestHeadersInCheck)
			}
			if ext.EnvoyExtAuthzHTTP.IncludeAdditionalHeadersInCheck != nil {
				envoyExtAuthzHTTPValues["includeAdditionalHeadersInCheck"] = mapOfStringToInterface(ext.EnvoyExtAuthzHTTP.IncludeAdditionalHeadersInCheck)
			}
			if ext.EnvoyExtAuthzHTTP.IncludeRequestBodyInCheck != nil {
				includeRequestBodyInCheckValues := map[string]interface{}{}
				if ext.EnvoyExtAuthzHTTP.IncludeRequestBodyInCheck.MaxRequestBytes != nil {
					includeRequestBodyInCheckValues["maxRequestBytes"] = *ext.EnvoyExtAuthzHTTP.IncludeRequestBodyInCheck.MaxRequestBytes
				}
				if ext.EnvoyExtAuthzHTTP.IncludeRequestBodyInCheck.AllowPartialMessage != nil {
					includeRequestBodyInCheckValues["allowPartialMessage"] = *ext.EnvoyExtAuthzHTTP.IncludeRequestBodyInCheck.AllowPartialMessage
				}
				if ext.EnvoyExtAuthzHTTP.IncludeRequestBodyInCheck.PackAsBytes != nil {
					includeRequestBodyInCheckValues["packAsBytes"] = *ext.EnvoyExtAuthzHTTP.IncludeRequestBodyInCheck.PackAsBytes
				}
				envoyExtAuthzHTTPValues["includeRequestBodyInCheck"] = includeRequestBodyInCheckValues
			}
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":              ext.Name,
				"envoyExtAuthzHttp": envoyExtAuthzHTTPValues,
			})
		}
	}
	if err := setHelmMapSliceValue(values, "meshConfig.extensionProviders", extensionProvidersValues); err != nil {
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
						envoyExtAuthzHTTPConfig := &v2.ExtensionProviderEnvoyExternalAuthorizationHTTPConfig{}
						if service, ok := envoyExtAuthzHTTP["service"]; ok {
							envoyExtAuthzHTTPConfig.Service = service.(string)
						} else {
							return fmt.Errorf("extension provider envoyExtAuthzHTTP '%s' must specify field 'service'", extProviderName)
						}
						if rawPort, ok := envoyExtAuthzHTTP["port"]; ok {
							if port, ok := rawPort.(int64); ok {
								envoyExtAuthzHTTPConfig.Port = port
							} else {
								return fmt.Errorf("extension provider envoyExtAuthzHTTP '%s' must specify field 'port' of type '%T'; got type '%T'",
									extProviderName, envoyExtAuthzHTTPConfig.Port, port)
							}
						}
						if rawTimeout, ok := envoyExtAuthzHTTP["timeout"]; ok {
							timeout := rawTimeout.(string)
							if _, err := time.ParseDuration(rawTimeout.(string)); err == nil {
								envoyExtAuthzHTTPConfig.Timeout = &timeout
							} else {
								return fmt.Errorf("extension provider envoyExtAuthzHTTP '%s' must specify field 'timeout' of type '%T'; got type '%T'",
									extProviderName, envoyExtAuthzHTTPConfig.Timeout, timeout)
							}
						}
						if rawPathPrefix, ok := envoyExtAuthzHTTP["pathPrefix"]; ok {
							envoyExtAuthzHTTPConfig.PathPrefix = strPtr(rawPathPrefix.(string))
						}
						if rawFailOpen, ok := envoyExtAuthzHTTP["failOpen"]; ok {
							if failOpen, ok := rawFailOpen.(bool); ok {
								envoyExtAuthzHTTPConfig.FailOpen = &failOpen
							} else {
								return fmt.Errorf("extension provider envoyExtAuthzHTTP '%s' must specify field 'failOpen' of type '%T'; got type '%T'",
									extProviderName, envoyExtAuthzHTTPConfig.FailOpen, failOpen)
							}
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
