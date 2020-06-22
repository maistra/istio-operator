package v2

// KialiAddonConfig is used to configure a kiali instance for use with the mesh
type KialiAddonConfig struct {
	// Name of Kiali CR, Namespace must match control plane namespace
	Name string `json:"name,omitempty"`
	// Install a Kiali resource if the named Kiali resource is not present.
	Install *KialiInstallConfig `json:"install,omitempty"`
}

// KialiInstallConfig is used to configure a kiali installation
type KialiInstallConfig struct {
	// Config is used to configure the behavior of the kiali installation
	Config KialiConfig `json:"config,omitempty"`
	// Service is used to configure the k8s Service associated with the kiali
	// installation.
	// XXX: provided for upstream support, only ingress is used, and then only
	// for enablement and contextPath
	Service ComponentServiceConfig `json:"service,omitempty"`
	// Runtime is used to customize the deployment/pod for the kiali installation.
	// XXX: largely unused, only image pull policy and image pull secrets are
	// relevant for maistra
	Runtime *ComponentRuntimeConfig `json:"runtime,omitempty"`
}

// KialiConfig configures the behavior of the kiali installation
type KialiConfig struct {
	// Dashboard configures the behavior of the kiali dashboard.
	Dashboard KialiDashboardConfig `json:"dashboard,omitempty"`
}

// KialiDashboardConfig configures the behavior of the kiali dashboard
type KialiDashboardConfig struct {
	// ViewOnly configures view_only_mode for the dashboard
	// .Values.kiali.dashboard.viewOnlyMode
	ViewOnly bool `json:"viewOnly,omitempty"`
	// XXX: should the user have a choice here, or should these be configured
	// automatically if they are enabled for the control plane installation?
	// Grafana endpoint will be configured based on Grafana configuration
	EnableGrafana bool `json:"enableGrafana,omitempty"`
	// Prometheus endpoint will be configured based on Prometheus configuration
	EnablePrometheus bool `json:"enablePrometheus,omitempty"`
	// Tracing endpoint will be configured based on Tracing configuration
	EnableTracing bool `json:"enableTracing,omitempty"`
}
