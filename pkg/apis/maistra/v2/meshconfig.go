package v2

// MeshConfig TODO: add description
type MeshConfig struct {
	// ExtensionProviders defines a list of extension providers that extend Istio's functionality. For example,
	// the AuthorizationPolicy can be used with an extension provider to delegate the authorization decision
	// to a custom authorization system.
	ExtensionProviders []*ExtensionProviderConfig `json:"extensionProviders,omitempty"`
}
