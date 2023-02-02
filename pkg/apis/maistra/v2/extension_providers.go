package v2

type ExtensionProvidersConfig struct {
	// Name is just a name
	Name string `json:"name,omitempty"`
	Prometheus *PrometheusExtensionProviderConfig `json:"prometheus,omitempty"`
}
type PrometheusExtensionProviderConfig struct {

}
