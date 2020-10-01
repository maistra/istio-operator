package v2

// `KialiAddonConfig` is used to configure a kiali instance for use with the mesh
type KialiAddonConfig struct {
	Enablement `json:",inline"`
	// Name of Kiali CR, Namespace must match control plane namespace
	Name string `json:"name,omitempty"`
	// Install a Kiali resource if the named Kiali resource is not present.
	// +optional
	Install *KialiInstallConfig `json:"install,omitempty"`
}

// `KialiInstallConfig` is used to configure a kiali installation
type KialiInstallConfig struct {
	// `Dashboard` configures the behavior of the kiali dashboard.
	// +optional
	Dashboard *KialiDashboardConfig `json:"dashboard,omitempty"`
	// `Service` is used to configure the k8s Service associated with the Kiali
	// installation. Ingress is used for enablement and context path.
	// for enablement and contextPath
	// +optional
	Service *ComponentServiceConfig `json:"service,omitempty"`
}

// KialiDashboardConfig configures the behavior of the kiali dashboard
type KialiDashboardConfig struct {
	// ViewOnly configures view_only_mode for the dashboard
	// `.Values.kiali.dashboard.viewOnlyMode`
	// +optional
	ViewOnly *bool `json:"viewOnly,omitempty"`
	// Grafana endpoint will be configured based on Grafana configuration
	// +optional
	EnableGrafana *bool `json:"enableGrafana,omitempty"`
	// Prometheus endpoint will be configured based on Prometheus configuration
	// +optional
	EnablePrometheus *bool `json:"enablePrometheus,omitempty"`
	// Tracing endpoint will be configured based on Tracing configuration
	// +optional
	EnableTracing *bool `json:"enableTracing,omitempty"`
}

// ResourceName returns the resource name for the Kiali resource. This defaults to "kiali" if the name field is not set.
func (c KialiAddonConfig) ResourceName() string {
	if c.Name == "" {
		return "kiali"
	}
	return c.Name
}
