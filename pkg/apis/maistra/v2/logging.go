package v2

// LoggingConfig configures logging for a component
type LoggingConfig struct {
	// Level the log level
	// .Values.global.proxy.logLevel, overridden by sidecar.istio.io/logLevel
	Level LogLevel `json:"level,omitempty"`
	// ComponentLevel configures log level for specific envoy components
	// .Values.global.proxy.componentLogLevel, overridden by sidecar.istio.io/componentLogLevel
	// map of <component>:<level>
	ComponentLevel map[EnvoyComponent]LogLevel `json:"componentLevel,omitempty"`
	// LogAsJSON enables JSON logging
	// .Values.global.logAsJson
	LogAsJSON bool `json:"logAsJSON,omitempty"`
}

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
