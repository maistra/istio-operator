package v2

// AddonsConfig configures additional features for use with the mesh
type AddonsConfig struct {
	// Metrics configures metrics storage solutions for the mesh.
	// +optional
	Metrics MetricsAddonsConfig `json:"metrics,omitempty"`
	// Tracing configures tracing solutions used with the mesh.
	// +optional
	Tracing TracingConfig `json:"tracing,omitempty"`
	// Visualization configures visualization solutions used with the mesh
	// +optional
	Visualization VisualizationAddonsConfig `json:"visualization,omitempty"`
}

// MetricsAddonsConfig configures metrics storage for the mesh.
type MetricsAddonsConfig struct {
	// Prometheus configures prometheus solution for metrics storage
	// .Values.prometheus.enabled, true if not null
	// implies other settings related to prometheus, e.g. .Values.telemetry.v2.prometheus.enabled,
	// .Values.kiali.prometheusAddr, etc.
	// +optional
	Prometheus *PrometheusAddonConfig `json:"prometheus,omitempty"`
}

// TracingConfig configures tracing solutions for the mesh.
// .Values.global.enableTracing
type TracingConfig struct {
	// Type represents the type of tracer to be installed.
	Type TracerType `json:"type,omitempty"`
	// Jaeger configures Jaeger as the tracer used with the mesh.
	// .Values.tracing.jaeger.enabled, true if not null
	// implies other settings related to tracing, e.g. .Values.global.tracer.zipkin.address,
	// .Values.kiali.dashboard.jaegerURL, etc.
	// +optional
	Jaeger *JaegerTracerConfig `json:"jaeger,omitempty"`
	//Zipkin      *ZipkinTracerConfig
	//Lightstep   *LightstepTracerConfig
	//Datadog     *DatadogTracerConfig
	//Stackdriver *StackdriverTracerConfig
}

// TracerType represents the tracer type to use
type TracerType string

const (
	// TracerTypeNone is used to represent no tracer
	TracerTypeNone TracerType = "None"
	// TracerTypeJaeger is used to represent Jaeger as the tracer
	TracerTypeJaeger TracerType = "Jaeger"
	// TracerTypeZipkin      TracerType = "Zipkin"
	// TracerTypeLightstep   TracerType = "Lightstep"
	// TracerTypeDatadog     TracerType = "Datadog"
	// TracerTypeStackdriver TracerType = "Stackdriver"
)

// VisualizationAddonsConfig configures visualization addons used with the mesh.
// More than one may be specified.
type VisualizationAddonsConfig struct {
	// Grafana configures a grafana instance to use with the mesh
	// .Values.grafana.enabled, true if not null
	// +optional
	Grafana *GrafanaAddonConfig `json:"grafana,omitempty"`
	// Kiali configures a kiali instance to use with the mesh
	// .Values.kiali.enabled, true if not null
	// +optional
	Kiali *KialiAddonConfig `json:"kiali,omitempty"`
}
