package conversion

import (
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateExtensionProvidersValues(in *v2.ControlPlaneSpec, allValues map[string]interface{}) error {
	if in.ExtensionProviders == nil {
		return nil
	}

	var extensionProvidersValues []map[string]interface{}
	for _, provider := range in.ExtensionProviders {
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
