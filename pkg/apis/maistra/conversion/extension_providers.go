package conversion

import (
	"fmt"

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

		if provider.Zipkin != nil {
			config := provider.Zipkin
			values := map[string]interface{}{
				"service": config.Service,
				"port":    config.Port,
			}
			if config.MaxTagLength != nil {
				values["maxTagLength"] = *config.MaxTagLength
			}
			if config.Enable64bitTraceID != nil {
				values["enable64bitTraceId"] = *config.Enable64bitTraceID
			}
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":   provider.Name,
				"zipkin": values,
			})
		}

		if provider.Opentelemetry != nil {
			config := provider.Opentelemetry
			values := map[string]interface{}{
				"service": config.Service,
				"port":    config.Port,
			}
			if config.MaxTagLength != nil {
				values["maxTagLength"] = *config.MaxTagLength
			}
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":          provider.Name,
				"opentelemetry": values,
			})
		}

		if provider.EnvoyOtelAls != nil {
			config := provider.EnvoyOtelAls
			values := map[string]interface{}{
				"service": config.Service,
				"port":    config.Port,
			}
			if config.LogName != nil {
				values["logName"] = *config.LogName
			}
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":         provider.Name,
				"envoyOtelAls": values,
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
			convertIncludeRequestBodyInCheckConfigToValues(config.IncludeRequestBodyInCheck, values)
			if config.HeadersToUpstreamOnAllow != nil {
				values["headersToUpstreamOnAllow"] = stringToInterfaceArray(config.HeadersToUpstreamOnAllow)
			}
			if config.HeadersToDownstreamOnDeny != nil {
				values["headersToDownstreamOnDeny"] = stringToInterfaceArray(config.HeadersToDownstreamOnDeny)
			}
			if config.HeadersToDownstreamOnAllow != nil {
				values["headersToDownstreamOnAllow"] = stringToInterfaceArray(config.HeadersToDownstreamOnAllow)
			}
			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":              provider.Name,
				"envoyExtAuthzHttp": values,
			})
		}
		if provider.EnvoyExtAuthzGRPC != nil {
			config := provider.EnvoyExtAuthzGRPC
			values := map[string]interface{}{
				"service": config.Service,
				"port":    config.Port,
			}
			if config.Timeout != nil {
				values["timeout"] = *config.Timeout
			}
			if config.FailOpen != nil {
				values["failOpen"] = *config.FailOpen
			}
			if config.StatusOnError != nil {
				values["statusOnError"] = *config.StatusOnError
			}
			convertIncludeRequestBodyInCheckConfigToValues(config.IncludeRequestBodyInCheck, values)

			extensionProvidersValues = append(extensionProvidersValues, map[string]interface{}{
				"name":              provider.Name,
				"envoyExtAuthzGrpc": values,
			})
		}
	}
	if err := setHelmMapSliceValue(allValues, "meshConfig.extensionProviders", extensionProvidersValues); err != nil {
		return err
	}
	return nil
}

func convertIncludeRequestBodyInCheckConfigToValues(config *v2.ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig, values map[string]interface{}) {
	if config != nil {
		includeRequestBodyInCheckValues := map[string]interface{}{}
		if config.MaxRequestBytes != nil {
			includeRequestBodyInCheckValues["maxRequestBytes"] = *config.MaxRequestBytes
		}
		if config.AllowPartialMessage != nil {
			includeRequestBodyInCheckValues["allowPartialMessage"] = *config.AllowPartialMessage
		}
		if config.PackAsBytes != nil {
			includeRequestBodyInCheckValues["packAsBytes"] = *config.PackAsBytes
		}
		values["includeRequestBodyInCheck"] = includeRequestBodyInCheckValues
	}
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
			config, err := convertProviderValuesToConfig(v1.NewHelmValues(provider))
			if err != nil {
				return err
			}
			out.MeshConfig.ExtensionProviders = append(out.MeshConfig.ExtensionProviders, &config)
		} else {
			return fmt.Errorf("could not cast extensionProviders entry to map[string]interface{}")
		}
	}
	return nil
}

func convertProviderValuesToConfig(values *v1.HelmValues) (v2.ExtensionProviderConfig, error) {
	var config v2.ExtensionProviderConfig

	if name, ok, err := values.GetString("name"); ok {
		config.Name = name
	} else if err != nil {
		return config, err
	}

	if _, found, err := values.GetMap("prometheus"); found {
		config.Prometheus = &v2.ExtensionProviderPrometheusConfig{}
	} else if err != nil {
		return config, err
	}

	if rawConfig, found, err := values.GetMap("zipkin"); found {
		config.Zipkin, err = convertZipkinValuesToConfig(v1.NewHelmValues(rawConfig))
		if err != nil {
			return config, err
		}
	} else if err != nil {
		return config, err
	}

	if rawConfig, found, err := values.GetMap("opentelemetry"); found {
		config.Opentelemetry, err = convertOtelValuesToConfig(v1.NewHelmValues(rawConfig))
		if err != nil {
			return config, err
		}
	} else if err != nil {
		return config, err
	}

	if rawConfig, found, err := values.GetMap("envoyOtelAls"); found {
		config.EnvoyOtelAls, err = convertEnvoyOtelAlsValuesToConfig(v1.NewHelmValues(rawConfig))
		if err != nil {
			return config, err
		}
	} else if err != nil {
		return config, err
	}

	if rawEnvoyExtAuthzHTTP, found, err := values.GetMap("envoyExtAuthzHttp"); found {
		config.EnvoyExtAuthzHTTP, err = convertEnvoyExtAuthzHTTPValuesToConfig(v1.NewHelmValues(rawEnvoyExtAuthzHTTP))
		if err != nil {
			return config, err
		}
	} else if err != nil {
		return config, err
	}

	if rawEnvoyExtAuthzGRPC, found, err := values.GetMap("envoyExtAuthzGrpc"); found {
		config.EnvoyExtAuthzGRPC, err = convertEnvoyExtAuthzGRPCValuesToConfig(v1.NewHelmValues(rawEnvoyExtAuthzGRPC))
		if err != nil {
			return config, err
		}
	} else if err != nil {
		return config, err
	}

	return config, nil
}

func convertZipkinValuesToConfig(values *v1.HelmValues) (*v2.ExtensionProviderZipkinTracingConfig, error) {
	config := &v2.ExtensionProviderZipkinTracingConfig{}
	if value, ok, err := values.GetString("service"); ok {
		config.Service = value
	} else if err != nil {
		return config, err
	} else {
		return config, fmt.Errorf("service is required for zipkin")
	}
	if value, ok, err := values.GetInt64("port"); ok {
		config.Port = value
	} else if err != nil {
		return config, err
	} else {
		return config, fmt.Errorf("port is required for zipkin")
	}
	if value, ok, err := values.GetInt64("maxTagLength"); ok {
		config.MaxTagLength = int64Ptr(value)
	} else if err != nil {
		return config, err
	}
	if value, ok, err := values.GetBool("enable64bitTraceId"); ok {
		config.Enable64bitTraceID = boolPtr(value)
	} else if err != nil {
		return config, err
	}

	return config, nil
}

func convertOtelValuesToConfig(values *v1.HelmValues) (*v2.ExtensionProviderOtelTracingConfig, error) {
	config := &v2.ExtensionProviderOtelTracingConfig{}
	if value, ok, err := values.GetString("service"); ok {
		config.Service = value
	} else if err != nil {
		return config, err
	} else {
		return config, fmt.Errorf("service is required for opentelemetry")
	}
	if value, ok, err := values.GetInt64("port"); ok {
		config.Port = value
	} else if err != nil {
		return config, err
	} else {
		return config, fmt.Errorf("port is required for opentelemetry")
	}
	if value, ok, err := values.GetInt64("maxTagLength"); ok {
		config.MaxTagLength = int64Ptr(value)
	} else if err != nil {
		return config, err
	}

	return config, nil
}

func convertEnvoyOtelAlsValuesToConfig(values *v1.HelmValues) (*v2.ExtensionProviderEnvoyOtelLogConfig, error) {
	config := &v2.ExtensionProviderEnvoyOtelLogConfig{}
	if value, ok, err := values.GetString("service"); ok {
		config.Service = value
	} else if err != nil {
		return config, err
	} else {
		return config, fmt.Errorf("service is required for envoyOtelAls")
	}
	if value, ok, err := values.GetInt64("port"); ok {
		config.Port = value
	} else if err != nil {
		return config, err
	} else {
		return config, fmt.Errorf("port is required for envoyOtelAls")
	}
	if value, ok, err := values.GetString("logName"); ok {
		config.LogName = strPtr(value)
	} else if err != nil {
		return config, err
	}

	return config, nil
}

func convertEnvoyExtAuthzHTTPValuesToConfig(values *v1.HelmValues) (*v2.ExtensionProviderEnvoyExternalAuthorizationHTTPConfig, error) {
	config := &v2.ExtensionProviderEnvoyExternalAuthorizationHTTPConfig{}

	if value, ok, err := values.GetString("service"); ok {
		config.Service = value
	} else if err != nil {
		return config, err
	} else {
		return config, fmt.Errorf("service is required for envoyExtAuthzHttp")
	}

	if value, ok, err := values.GetInt64("port"); ok {
		config.Port = value
	} else if err != nil {
		return config, err
	} else {
		return config, fmt.Errorf("port is required for envoyExtAuthzHttp")
	}

	if value, ok, err := values.GetString("timeout"); ok {
		config.Timeout = strPtr(value)
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetString("pathPrefix"); ok {
		config.PathPrefix = strPtr(value)
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetBool("failOpen"); ok {
		config.FailOpen = boolPtr(value)
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetString("statusOnError"); ok {
		config.StatusOnError = strPtr(value)
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetStringSlice("includeRequestHeadersInCheck"); ok {
		config.IncludeRequestHeadersInCheck = value
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetStringMap("includeAdditionalHeadersInCheck"); ok {
		config.IncludeAdditionalHeadersInCheck = value
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetMap("includeRequestBodyInCheck"); ok {
		config.IncludeRequestBodyInCheck, err = convertIncludeRequestBodyInCheckValuesToConfig(v1.NewHelmValues(value))
		if err != nil {
			return config, err
		}
	} else if err != nil {
		return config, err
	}
	if value, ok, err := values.GetStringSlice("headersToUpstreamOnAllow"); ok {
		config.HeadersToUpstreamOnAllow = value
	} else if err != nil {
		return config, err
	}
	if value, ok, err := values.GetStringSlice("headersToDownstreamOnDeny"); ok {
		config.HeadersToDownstreamOnDeny = value
	} else if err != nil {
		return config, err
	}
	if value, ok, err := values.GetStringSlice("headersToDownstreamOnAllow"); ok {
		config.HeadersToDownstreamOnAllow = value
	} else if err != nil {
		return config, err
	}

	return config, nil
}

func convertEnvoyExtAuthzGRPCValuesToConfig(values *v1.HelmValues) (*v2.ExtensionProviderEnvoyExternalAuthorizationGRPCConfig, error) {
	config := &v2.ExtensionProviderEnvoyExternalAuthorizationGRPCConfig{}

	if value, ok, err := values.GetString("service"); ok {
		config.Service = value
	} else if err != nil {
		return config, err
	} else {
		return config, fmt.Errorf("service is required for envoyExtAuthzGRPC")
	}

	if value, ok, err := values.GetInt64("port"); ok {
		config.Port = value
	} else if err != nil {
		return config, err
	} else {
		return config, fmt.Errorf("port is required for envoyExtAuthzGRPC")
	}

	if value, ok, err := values.GetString("timeout"); ok {
		config.Timeout = strPtr(value)
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetBool("failOpen"); ok {
		config.FailOpen = boolPtr(value)
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetString("statusOnError"); ok {
		config.StatusOnError = strPtr(value)
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetMap("includeRequestBodyInCheck"); ok {
		config.IncludeRequestBodyInCheck, err = convertIncludeRequestBodyInCheckValuesToConfig(v1.NewHelmValues(value))
		if err != nil {
			return config, err
		}
	} else if err != nil {
		return config, err
	}

	return config, nil
}

func convertIncludeRequestBodyInCheckValuesToConfig(values *v1.HelmValues) (*v2.ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig, error) {
	config := &v2.ExtensionProviderEnvoyExternalAuthorizationRequestBodyConfig{}

	if value, ok, err := values.GetInt64("maxRequestBytes"); ok {
		config.MaxRequestBytes = int64Ptr(value)
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetBool("allowPartialMessage"); ok {
		config.AllowPartialMessage = boolPtr(value)
	} else if err != nil {
		return config, err
	}

	if value, ok, err := values.GetBool("packAsBytes"); ok {
		config.PackAsBytes = boolPtr(value)
	} else if err != nil {
		return config, err
	}

	return config, nil
}
