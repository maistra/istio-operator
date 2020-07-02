package v2

// LoggingConfig for control plane components
type LoggingConfig struct {
	// ComponentLevels configures log level for specific envoy components
	// .Values.global.proxy.componentLogLevel, overridden by sidecar.istio.io/componentLogLevel
	// map of <component>:<level>
	// +optional
	ComponentLevels ComponentLogLevels `json:"componentLevels,omitempty"`
	// LogAsJSON enables JSON logging
	// .Values.global.logAsJson
	// +optional
	LogAsJSON *bool `json:"logAsJSON,omitempty"`
}

// ProxyLoggingConfig configures logging for a component
type ProxyLoggingConfig struct {
	// Level the log level
	// .Values.global.proxy.logLevel, overridden by sidecar.istio.io/logLevel
	// +optional
	Level LogLevel `json:"level,omitempty"`
	// ComponentLevels configures log level for specific envoy components
	// .Values.global.proxy.componentLogLevel, overridden by sidecar.istio.io/componentLogLevel
	// map of <component>:<level>
	// +optional
	ComponentLevels ComponentLogLevels `json:"componentLevels,omitempty"`
}

type ComponentLogLevels map[EnvoyComponent]LogLevel
// LogLevel represents the logging level
type LogLevel string

const (
	// LogLevelTrace trace logging level
	LogLevelTrace LogLevel = "trace"
	// LogLevelDebug debug logging level
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo info logging level
	LogLevelInfo LogLevel = "info"
	// LogLevelWarning warning logging level
	LogLevelWarning LogLevel = "warning"
	// LogLevelError error logging level
	LogLevelError LogLevel = "error"
	// LogLevelCritical critical logging level
	LogLevelCritical LogLevel = "critical"
	// LogLevelOff disable logging
	LogLevelOff LogLevel = "off"
)

// EnvoyComponent represents an envoy component to configure logging
type EnvoyComponent string
