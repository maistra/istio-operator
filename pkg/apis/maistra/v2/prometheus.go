package v2

// `PrometheusAddonConfig` configures a Prometheus instance to be used by the
// control plane.  Only one of `Install` or `Address` may be specified
type PrometheusAddonConfig struct {
	Enablement `json:",inline"`
	// `MetricsExpiryDuration` is the duration to hold metrics. (mixer/v1 only) Defaults to 10m
	// `.Values.mixer.adapters.prometheus.metricsExpiryDuration`
	// +optional
	MetricsExpiryDuration string `json:"metricsExpiryDuration,omitempty"`
	// Scrape metrics from the pod if true. (maistra-2.0+). Defaults to true
	// `.Values.meshConfig.enablePrometheusMerge`
	// +optional
	Scrape *bool `json:"scrape,omitempty"`
	// Install configuration if not using an existing Prometheus installation.
	// `.Values.prometheus.enabled`, if not null
	// +optional
	Install *PrometheusInstallConfig `json:"install,omitempty"`
	// Address of existing prometheus installation
	// implies .Values.kiali.prometheusAddr
	// +optional
	Address *string `json:"address,omitempty"`
}

// `PrometheusInstallConfig` represents the configuration to be applied when
// installing a new instance of Prometheus for use with the mesh.
type PrometheusInstallConfig struct {
	// `SelfManaged` specifies whether or not the Maistra operator or Prometheus operator should manage the Prometheus installation. Set `SelfManaged` to `true` to use the Maistra operator or `false` to use the Prometheus operator. No support is provided for the Prometheus operator.
	// Governs use of either prometheus charts or prometheusOperator charts.
	// +optional
	SelfManaged bool `json:"selfManaged,omitempty"`
	// `Retention` specifies how long metrics should be retained by Prometheus. Defaults to 6h.
	// `.Values.prometheus.retention`
	// +optional
	Retention string `json:"retention,omitempty"`
	// `ScrapeInterval` specifies how frequently Prometheus should scrape pods for
	// metrics. Defaults to 15s.
	// `.Values.prometheus.scrapeInterval`
	// +optional
	ScrapeInterval string `json:"scrapeInterval,omitempty"`
	// `Service` allows for customization of the Kubernetes Service associated with the
	// Prometheus installation.
	// +optional
	Service *ComponentServiceConfig `json:"service,omitempty"`

	//ProvisionCert bool
	// this seems to overlap with provision cert, as this manifests something similar to the above
	// .Values.prometheus.security.enabled, version < 1.6

	//EnableSecurity bool
	// +optional

	// `UseTLS` configures whether the Prometheus server should use TLS.
	// `.Values.prometheus.provisionPrometheusCert`
	// 1.6+
	UseTLS *bool `json:"useTLS,omitempty"`
}
