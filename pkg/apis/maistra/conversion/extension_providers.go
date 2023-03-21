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
		if ext.Prometheus == nil && ext.EnvoyExtAuthzHttp == nil {
			return fmt.Errorf("extension provider %s does not define any provider - it must specify one of: prometheus or envoyExtAuthzHttp", ext.Name)
		}
		if ext.Prometheus != nil && ext.EnvoyExtAuthzHttp != nil {
			return fmt.Errorf("extension provider %s must specify only one type of provider: prometheus or envoyExtAuthzHttp", ext.Name)
		}
		if ext.Prometheus != nil {
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				// TODO: check empty string
				"name":       ext.Name,
				"prometheus": map[string]interface{}{},
			})
		}
		if ext.EnvoyExtAuthzHttp != nil {
			envoyExtAuthzHttpValues := map[string]interface{}{}
			if ext.EnvoyExtAuthzHttp.Service != "" {
				envoyExtAuthzHttpValues["service"] = ext.EnvoyExtAuthzHttp.Service
			} else {
				// TODO
			}
			if ext.EnvoyExtAuthzHttp.Port != 0 {
				envoyExtAuthzHttpValues["port"] = ext.EnvoyExtAuthzHttp.Port
			} else {
				// Todo
			}
			// TODO: change to pointer
			if ext.EnvoyExtAuthzHttp.Timeout != nil {
				envoyExtAuthzHttpValues["timeout"] = *ext.EnvoyExtAuthzHttp.Timeout
			}
			if ext.EnvoyExtAuthzHttp.PathPrefix != nil {
				envoyExtAuthzHttpValues["pathPrefix"] = *ext.EnvoyExtAuthzHttp.PathPrefix
			}
			if ext.EnvoyExtAuthzHttp.FailOpen != nil {
				envoyExtAuthzHttpValues["failOpen"] = *ext.EnvoyExtAuthzHttp.FailOpen
			}
			if ext.EnvoyExtAuthzHttp.StatusOnError != nil {
				envoyExtAuthzHttpValues["statusOnError"] = *ext.EnvoyExtAuthzHttp.StatusOnError
			}
			if ext.EnvoyExtAuthzHttp.IncludeRequestHeadersInCheck != nil {
				envoyExtAuthzHttpValues["includeRequestHeadersInCheck"] = stringToInterfaceArray(ext.EnvoyExtAuthzHttp.IncludeRequestHeadersInCheck)
			}
			if ext.EnvoyExtAuthzHttp.IncludeAdditionalHeadersInCheck != nil {
				envoyExtAuthzHttpValues["includeAdditionalHeadersInCheck"] = mapOfStringToInterface(ext.EnvoyExtAuthzHttp.IncludeAdditionalHeadersInCheck)
			}
			if ext.EnvoyExtAuthzHttp.IncludeRequestBodyInCheck != nil {
				includeRequestBodyInCheckValues := map[string]interface{}{}
				if ext.EnvoyExtAuthzHttp.IncludeRequestBodyInCheck.MaxRequestBytes != nil {
					includeRequestBodyInCheckValues["maxRequestBytes"] = *ext.EnvoyExtAuthzHttp.IncludeRequestBodyInCheck.MaxRequestBytes
				}
				if ext.EnvoyExtAuthzHttp.IncludeRequestBodyInCheck.AllowPartialMessage != nil {
					includeRequestBodyInCheckValues["allowPartialMessage"] = *ext.EnvoyExtAuthzHttp.IncludeRequestBodyInCheck.AllowPartialMessage
				}
				if ext.EnvoyExtAuthzHttp.IncludeRequestBodyInCheck.PackAsBytes != nil {
					includeRequestBodyInCheckValues["packAsBytes"] = *ext.EnvoyExtAuthzHttp.IncludeRequestBodyInCheck.PackAsBytes
				}
				envoyExtAuthzHttpValues["includeRequestBodyInCheck"] = includeRequestBodyInCheckValues
			}
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":              ext.Name,
				"envoyExtAuthzHttp": envoyExtAuthzHttpValues,
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
				if rawEnvoyExtAuthzHttp, ok := extProvider["envoyExtAuthzHttp"]; ok {
					if envoyExtAuthzHttp, ok := rawEnvoyExtAuthzHttp.(map[string]interface{}); ok {
						extProviderName := extProvider["name"].(string)
						envoyExtAuthzHttpConfig := &v2.ExtensionProviderEnvoyExternalAuthorizationHttpConfig{}
						if service, ok := envoyExtAuthzHttp["service"]; ok {
							envoyExtAuthzHttpConfig.Service = service.(string)
						} else {
							return fmt.Errorf("extension provider envoyExtAuthzHttp '%s' must specify field 'service'", extProviderName)
						}
						if rawPort, ok := envoyExtAuthzHttp["port"]; ok {
							if port, ok := rawPort.(int64); ok {
								envoyExtAuthzHttpConfig.Port = port
							} else {
								return fmt.Errorf("extension provider envoyExtAuthzHttp '%s' must specify field 'port' of type '%T'; got type '%T'",
									extProviderName, envoyExtAuthzHttpConfig.Port, port)
							}
						}
						if rawTimeout, ok := envoyExtAuthzHttp["timeout"]; ok {
							timeout := rawTimeout.(string)
							if _, err := time.ParseDuration(rawTimeout.(string)); err == nil {
								envoyExtAuthzHttpConfig.Timeout = &timeout
							} else {
								return fmt.Errorf("extension provider envoyExtAuthzHttp '%s' must specify field 'timeout' of type '%T'; got type '%T'",
									extProviderName, envoyExtAuthzHttpConfig.Timeout, timeout)
							}
						}
						if rawPathPrefix, ok := envoyExtAuthzHttp["pathPrefix"]; ok {
							envoyExtAuthzHttpConfig.PathPrefix = strPtr(rawPathPrefix.(string))
						}
						if rawFailOpen, ok := envoyExtAuthzHttp["failOpen"]; ok {
							if failOpen, ok := rawFailOpen.(bool); ok {
								envoyExtAuthzHttpConfig.FailOpen = &failOpen
							} else {
								return fmt.Errorf("extension provider envoyExtAuthzHttp '%s' must specify field 'failOpen' of type '%T'; got type '%T'",
									extProviderName, envoyExtAuthzHttpConfig.FailOpen, failOpen)
							}
						}
						if statusOnError, ok := envoyExtAuthzHttp["statusOnError"]; ok {
							envoyExtAuthzHttpConfig.StatusOnError = strPtr(statusOnError.(string))
						}
						if rawIncludeRequestHeadersInCheck, ok := envoyExtAuthzHttp["includeRequestHeadersInCheck"]; ok {
							envoyExtAuthzHttpConfig.IncludeRequestHeadersInCheck = interfaceToStringArray(rawIncludeRequestHeadersInCheck.([]interface{}))
						}
						if rawIncludeAdditionalHeadersInCheck, ok := envoyExtAuthzHttp["includeAdditionalHeadersInCheck"]; ok {
							envoyExtAuthzHttpConfig.IncludeAdditionalHeadersInCheck = mapOfInterfaceToString(rawIncludeAdditionalHeadersInCheck.(map[string]interface{}))
						}
						if rawIncludeRequestBodyInCheck, ok := envoyExtAuthzHttp["includeRequestBodyInCheck"]; ok {
							if includeRequestBodyInCheck, ok := rawIncludeRequestBodyInCheck.(map[string]interface{}); ok {
								envoyExtAuthzHttpConfig.IncludeRequestBodyInCheck = &v2.ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig{}
								if maxRequestBytes, ok := includeRequestBodyInCheck["maxRequestBytes"]; ok {
									envoyExtAuthzHttpConfig.IncludeRequestBodyInCheck.MaxRequestBytes = int64Ptr(maxRequestBytes.(int64))
								}
								if allowPartialMessage, ok := includeRequestBodyInCheck["allowPartialMessage"]; ok {
									envoyExtAuthzHttpConfig.IncludeRequestBodyInCheck.AllowPartialMessage = boolPtr(allowPartialMessage.(bool))
								}
								if packAsBytes, ok := includeRequestBodyInCheck["packAsBytes"]; ok {
									envoyExtAuthzHttpConfig.IncludeRequestBodyInCheck.PackAsBytes = boolPtr(packAsBytes.(bool))
								}
							}
						}
						out.ExtensionProviders = append(out.ExtensionProviders, &v2.ExtensionProviderConfig{
							Name:              extProviderName,
							EnvoyExtAuthzHttp: envoyExtAuthzHttpConfig,
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
