package v2

import v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

// TelemetryConfig for the mesh
type TelemetryConfig struct {
	// Type of telemetry implementation to use.
	Type TelemetryType `json:"type,omitempty"`
	// Mixer represents legacy, v1 telemetry.
	// implies .Values.telemetry.v1.enabled, if not null
	// +optional
	Mixer *MixerTelemetryConfig `json:"mixer,omitempty"`
	// Remote represents a remote, legacy, v1 telemetry.
	// +optional
	Remote *RemoteTelemetryConfig `json:"remote,omitempty"`
	// Istiod represents istiod, v2 telemetry
	// +optional
	Istiod *IstiodTelemetryConfig `json:"istiod,omitempty"`
}

// TelemetryType represents the telemetry implementation used.
type TelemetryType string

const (
	// TelemetryTypeNone disables telemetry
	TelemetryTypeNone TelemetryType = "None"
	// TelemetryTypeMixer represents mixer telemetry, v1
	TelemetryTypeMixer TelemetryType = "Mixer"
	// TelemetryTypeRemote represents remote mixer telemetry server, v1
	TelemetryTypeRemote TelemetryType = "Remote"
	// TelemetryTypeIstiod represents istio, v2
	TelemetryTypeIstiod TelemetryType = "Istiod"
)

// MixerTelemetryConfig is the configuration for legacy, v1 mixer telemetry.
// .Values.telemetry.v1.enabled
type MixerTelemetryConfig struct {
	// SessionAffinity configures session affinity for sidecar telemetry connections.
	// .Values.mixer.telemetry.sessionAffinityEnabled, maps to MeshConfig.sidecarToTelemetrySessionAffinity
	// +optional
	SessionAffinity *bool `json:"sessionAffinity,omitempty"`
	// Batching settings used when sending telemetry.
	// +optional
	Batching TelemetryBatchingConfig `json:"batching,omitempty"`
	// Runtime configuration to apply to the mixer telemetry deployment.
	// +optional
	Runtime *ComponentRuntimeConfig `json:"runtime,omitempty"`
	// Adapters configures the adapters used by mixer telemetry.
	// +optional
	Adapters *MixerTelemetryAdaptersConfig `json:"adapters,omitempty"`
}

// TelemetryBatchingConfig configures how telemetry data is batched.
type TelemetryBatchingConfig struct {
	// MaxEntries represents the maximum number of entries to collect before sending them to mixer.
	// .Values.mixer.telemetry.reportBatchMaxEntries, maps to MeshConfig.reportBatchMaxEntries
	// Set reportBatchMaxEntries to 0 to use the default batching behavior (i.e., every 100 requests).
	// A positive value indicates the number of requests that are batched before telemetry data
	// is sent to the mixer server
	// +optional
	MaxEntries *int32 `json:"maxEntries,omitempty"`
	// MaxTime represents the maximum amount of time to hold entries before sending them to mixer.
	// .Values.mixer.telemetry.reportBatchMaxTime, maps to MeshConfig.reportBatchMaxTime
	// Set reportBatchMaxTime to 0 to use the default batching behavior (i.e., every 1 second).
	// A positive time value indicates the maximum wait time since the last request will telemetry data
	// be batched before being sent to the mixer server
	// +optional
	MaxTime string `json:"maxTime,omitempty"`
}

// MixerTelemetryAdaptersConfig is the configuration for mixer telemetry adapters.
type MixerTelemetryAdaptersConfig struct {
	// UseAdapterCRDs specifies whether or not mixer should support deprecated CRDs.
	// .Values.mixer.adapters.useAdapterCRDs, removed in istio 1.4, defaults to false
	// XXX: i think this can be removed completely
	// +optional
	UseAdapterCRDs *bool `json:"useAdapterCRDs,omitempty"`
	// KubernetesEnv enables support for the kubernetesenv adapter.
	// .Values.mixer.adapters.kubernetesenv.enabled, defaults to true
	// +optional
	KubernetesEnv *bool `json:"kubernetesenv,omitempty"`
	// Stdio enables and configures the stdio adapter.
	// .Values.mixer.adapters.stdio.enabled, defaults to false (null)
	// +optional
	Stdio *MixerTelemetryStdioConfig `json:"stdio,omitempty"`
	// Prometheus enables and configures the prometheus adapter.
	// .Values.mixer.adapters.prometheus.enabled, defaults to true (non-null)
	// XXX: should this be defined through prometheus add-on, as opposed to here?
	// +optional
	Prometheus *MixerTelemetryPrometheusConfig `json:"prometheus,omitempty"`
	// Stackdriver enables and configures the stackdriver apdater.
	// .Values.mixer.adapters.stackdriver.enabled, defaults to false (null)
	// XXX: should this be defined through stackdriver add-on, as opposed to here?
	// +optional
	Stackdriver *MixerTelemetryStackdriverConfig `json:"stackdriver,omitempty"`
}

// MixerTelemetryStdioConfig configures the stdio adapter for mixer telemetry.
type MixerTelemetryStdioConfig struct {
	// OutputAsJSON if true.
	// .Values.mixer.adapters.stdio.outputAsJson, defaults to false
	// +optional
	OutputAsJSON bool `json:"outputAsJSON,omitempty"`
}

// MixerTelemetryPrometheusConfig configures the prometheus adapter for mixer telemetry.
type MixerTelemetryPrometheusConfig struct {
	// MetricsExpiryDuration is the duration to hold metrics.
	// .Values.mixer.adapters.prometheus.metricsExpiryDuration, defaults to 10m
	// +optional
	MetricsExpiryDuration string `json:"metricsExpiryDuration,omitempty"`
}

// MixerTelemetryStackdriverConfig configures the stackdriver adapter for mixer telemetry.
type MixerTelemetryStackdriverConfig struct {
	// Auth configuration for stackdriver adapter
	// +optional
	Auth *MixerTelemetryStackdriverAuthConfig `json:"auth,omitempty"`
	// Tracer configuration for stackdriver adapter
	// .Values.mixer.adapters.stackdriver.tracer.enabled, defaults to false (null)
	// +optional
	Tracer *MixerTelemetryStackdriverTracerConfig `json:"tracer,omitempty"`
	// EnableContextGraph for stackdriver adapter
	// .Values.mixer.adapters.stackdriver.contextGraph.enabled, defaults to false
	// +optional
	EnableContextGraph bool `json:"enableContextGraph,omitempty"`
	// EnableLogging for stackdriver adapter
	// .Values.mixer.adapters.stackdriver.logging.enabled, defaults to true
	// +optional
	EnableLogging bool `json:"enableLogging,omitempty"`
	// EnableMetrics for stackdriver adapter
	// .Values.mixer.adapters.stackdriver.metrics.enabled, defaults to true
	// +optional
	EnableMetrics bool `json:"enableMetrics,omitempty"`
}

// MixerTelemetryStackdriverAuthConfig is the auth config for stackdriver.  Only one field may be set
type MixerTelemetryStackdriverAuthConfig struct {
	// AppCredentials if true, use default app credentials.
	// .Values.mixer.adapters.stackdriver.auth.appCredentials, defaults to false
	// +optional
	AppCredentials bool `json:"appCredentials,omitempty"`
	// APIKey use the specified key.
	// .Values.mixer.adapters.stackdriver.auth.apiKey
	// +optional
	APIKey string `json:"apiKey,omitempty"`
	// ServiceAccountPath use the path to the service account.
	// .Values.mixer.adapters.stackdriver.auth.serviceAccountPath
	// +optional
	ServiceAccountPath string `json:"serviceAccountPath,omitempty"`
}

// MixerTelemetryStackdriverTracerConfig tracer config for stackdriver mixer telemetry adapter
type MixerTelemetryStackdriverTracerConfig struct {
	// SampleProbability to use for tracer data.
	// .Values.mixer.adapters.stackdriver.tracer.sampleProbability
	// +optional
	SampleProbability int `json:"sampleProbability,omitempty"`
}

// RemoteTelemetryConfig configures a remote, legacy, v1 mixer telemetry.
// .Values.telemetry.v1.enabled true
type RemoteTelemetryConfig struct {
	// Address is the address of the remote telemetry server
	// .Values.global.remoteTelemetryAddress, maps to MeshConfig.mixerReportServer
	Address string `json:"address,omitempty"`
	// CreateService for the remote server.
	// .Values.global.createRemoteSvcEndpoints
	// +optional
	CreateService bool `json:"createService,omitempty"`
	// Batching settings used when sending telemetry.
	// +optional
	Batching TelemetryBatchingConfig `json:"batching,omitempty"`
}

// IstiodTelemetryConfig configures v2 telemetry using istiod
// .Values.telemetry.v2.enabled
type IstiodTelemetryConfig struct {
	// MetadataExchange configuration for v2 telemetry.
	// always enabled
	// +optional
	MetadataExchange *MetadataExchangeConfig `json:"metadataExchange,omitempty"`
	// PrometheusFilter configures the prometheus filter for v2 telemetry.
	// .Values.telemetry.v2.prometheus.enabled
	// XXX: should this be defined through prometheus add-on, as opposed to here?
	// +optional
	PrometheusFilter *PrometheusFilterConfig `json:"prometheusFilter,omitempty"`
	// StackDriverFilter configures the stackdriver filter for v2 telemetry.
	// .Values.telemetry.v2.stackdriver.enabled
	// XXX: should this be defined through stackdriver add-on, as opposed to here?
	// +optional
	StackDriverFilter *StackDriverFilterConfig `json:"stackDriverFilter,omitempty"`
	// AccessLogTelemetryFilter configures the access logging filter for v2 telemetry.
	// .Values.telemetry.v2.accessLogPolicy.enabled
	// +optional
	AccessLogTelemetryFilter *AccessLogTelemetryFilterConfig `json:"accessLogTelemetryFilter,omitempty"`
}

// MetadataExchangeConfig for v2 telemetry.
type MetadataExchangeConfig struct {
	// WASMEnabled for metadata exchange.
	// .Values.telemetry.v2.metadataExchange.wasmEnabled
	// Indicates whether to enable WebAssembly runtime for metadata exchange filter.
	// +optional
	WASMEnabled bool `json:"wasmEnabled,omitempty"`
}

// PrometheusFilterConfig for v2 telemetry.
// previously enablePrometheusMerge
// annotates injected pods with prometheus.io annotations (scrape, path, port)
// overridden through prometheus.istio.io/merge-metrics
type PrometheusFilterConfig struct {
	// Scrape metrics from the pod if true.
	// defaults to true
	// .Values.meshConfig.enablePrometheusMerge
	// +optional
	Scrape bool `json:"scrape,omitempty"`
	// WASMEnabled for prometheus filter.
	// Indicates whether to enable WebAssembly runtime for stats filter.
	// .Values.telemetry.v2.prometheus.wasmEnabled
	// +optional
	WASMEnabled bool `json:"wasmEnabled,omitempty"`
}

// StackDriverFilterConfig for v2 telemetry.
type StackDriverFilterConfig struct {
	// all default to false
	// +optional
	Logging         bool              `json:"logging,omitempty"`
	// +optional
	Monitoring      bool              `json:"monitoring,omitempty"`
	// +optional
	Topology        bool              `json:"topology,omitempty"`
	// +optional
	DisableOutbound bool              `json:"disableOutbound,omitempty"`
	// +optional
	ConfigOverride  *v1.HelmValues `json:"configOverride,omitempty"`
}

// AccessLogTelemetryFilterConfig for v2 telemetry.
type AccessLogTelemetryFilterConfig struct {
	// LogWindoDuration configures the log window duration for access logs.
	// defaults to 43200s
	// To reduce the number of successful logs, default log window duration is
	// set to 12 hours.
	// +optional
	LogWindoDuration string `json:"logWindowDuration,omitempty"`
}
