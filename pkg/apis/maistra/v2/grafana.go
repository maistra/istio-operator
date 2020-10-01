package v2

// GrafanaAddonConfig configures a grafana instance for use with the mesh. Only
// one of install or address may be specified
type GrafanaAddonConfig struct {
	Enablement `json:",inline"`
	// `Install` installs a new grafana instance and manage with control plane
	// +optional
	Install *GrafanaInstallConfig `json:"install,omitempty"`
	// `Address` is the address of an existing grafana installation
	// implies .Values.kiali.dashboard.grafanaURL
	// +optional
	Address *string `json:"address,omitempty"`
}

// `GrafanaInstallConfig` is used to configure a new installation of grafana.
type GrafanaInstallConfig struct {
	// `SelfManaged`, indicates whether the Grafana install is managed by Maistra or the Grafana custom resource. Set to true to use the Maistra installer. No support is provided for the Grafana custom resource.
	// +optional
	SelfManaged bool `json:"selfManaged,omitempty"`
	// `Config` configures the behavior of the grafana installation
	// +optional
	Config *GrafanaConfig `json:"config,omitempty"`
	// `Service` configures the k8s Service associated with the grafana installation
	// +optional
	Service *ComponentServiceConfig `json:"service,omitempty"`
	// `Persistence` configures a PersistentVolume associated with the grafana installation
	// `.Values.grafana.persist`, true if not null
	// +optional
	Persistence *ComponentPersistenceConfig `json:"persistence,omitempty"`
	// `Security` is used to secure the grafana service.
	// `.Values.grafana.security.enabled`, true if not null
	// +optional
	Security *GrafanaSecurityConfig `json:"security,omitempty"`
}

// `GrafanaConfig` configures the behavior of the grafana installation
type GrafanaConfig struct {
	// `Env` allows specification of various grafana environment variables to be
	// configured on the Grafana container.
	// .Values.grafana.env
	// +optional
	Env map[string]string `json:"env,omitempty"`
	// `EnvSecrets` allows specification of secret fields into grafana environment
	// variables to be configured on the grafana container
	// `.Values.grafana.envSecrets`
	// +optional
	EnvSecrets map[string]string `json:"envSecrets,omitempty"`
}

// `GrafanaSecurityConfig` is used to secure access to grafana
type GrafanaSecurityConfig struct {
	Enablement `json:",inline"`
	// `SecretName` is the name of a secret containing the username/password that
	// should be used to access grafana.
	// +optional
	SecretName string `json:"secretName,omitempty"`
	// `UsernameKey` is the name of the key within the secret identifying the username.
	// +optional
	UsernameKey string `json:"usernameKey,omitempty"`
	// `PassphraseKey` is the name of the key within the secret identifying the password.
	// +optional
	PassphraseKey string `json:"passphraseKey,omitempty"`
}
