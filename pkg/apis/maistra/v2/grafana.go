package v2

// GrafanaAddonConfig configures a grafana instance for use with the mesh. Only
// one of install or address may be specified
type GrafanaAddonConfig struct {
	// Install a new grafana instance and manage with control plane
	Install *GrafanaInstallConfig `json:"install,omitempty"`
	// Address is the address of an existing grafana installation
	// implies .Values.kiali.dashboard.grafanaURL
	Address *string `json:"address,omitempty"`
}

// GrafanaInstallConfig is used to configure a new installation of grafana.
type GrafanaInstallConfig struct {
	// SelfManaged, true if the entire install should be managed by Maistra, false if using grafana CR (not supported)
	SelfManaged bool `json:"selfManaged,omitempty"`
	// Config configures the behavior of the grafana installation
	Config GrafanaConfig `json:"config,omitempty"`
	// Service configures the k8s Service associated with the grafana installation
	Service ComponentServiceConfig `json:"service,omitempty"`
	// Persistence configures a PersistentVolume associated with the grafana installation
	// .Values.grafana.persist, true if not null
	Persistence *ComponentPersistenceConfig `json:"persistence,omitempty"`
	// Runtime is used to customize the grafana deployment/pod
	Runtime *ComponentRuntimeConfig `json:"runtime,omitempty"`
	// Security is used to secure the grafana service.
	// .Values.grafana.security.enabled, true if not null
	// XXX: unused for maistra, as we use oauth-proxy
	Security *GrafanaSecurityConfig `json:"security,omitempty"`
}

// GrafanaConfig configures the behavior of the grafana installation
type GrafanaConfig struct {
	// Env allows specification of various grafana environment variables to be
	// configured on the grafana container.
	// .Values.grafana.env
	// XXX: This is pretty cheesy...
	Env map[string]string `json:"env,omitempty"`
	// EnvSecrets allows specification of secret fields into grafana environment
	// variables to be configured on the grafana container
	// .Values.grafana.envSecrets
	// XXX: This is pretty cheesy...
	EnvSecrets map[string]string `json:"envSecrets,omitempty"`
}

// GrafanaSecurityConfig is used to secure access to grafana
type GrafanaSecurityConfig struct {
	// SecretName is the name of a secret containing the username/password that
	// should be used to access grafana.
	SecretName string `json:"secretName,omitempty"`
	// UsernameKey is the name of the key within the secret identifying the username.
	UsernameKey string `json:"usernameKey,omitempty"`
	// PassphraseKey is the name of the key within the secret identifying the password.
	PassphraseKey string `json:"passphraseKey,omitempty"`
}
