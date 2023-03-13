package v2

type ExtensionProviderConfig struct {
	// A unique name identifying the extension provider.
	Name       string                             `json:"name"`
	Prometheus *ExtensionProviderPrometheusConfig `json:"prometheus,omitempty"`
}

// ExtensionProviderPrometheusConfig configures a Prometheus metrics provider.
type ExtensionProviderPrometheusConfig struct{}
