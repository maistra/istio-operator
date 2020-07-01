package conversion

import (
	"fmt"
	"strings"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateControlPlaneLogging(logging *v2.LoggingConfig, values map[string]interface{}) error {
	componentLevels := componentLogLevelsToString(logging.ComponentLevels)
	if componentLevels != "" {
		if err := setHelmValue(values, "global.logging.level", componentLevels); err != nil {
			return err
		}
	}
	if logging.LogAsJSON != nil {
		if err := setHelmValue(values, "global.logAsJson", *logging.LogAsJSON); err != nil {
			return err
		}
	}
	return nil
}

func populateProxyLogging(logging *v2.ProxyLoggingConfig, values map[string]interface{}) error {
	if logging == nil {
		return nil
	}
	if logging.Level != "" {
		if err := setHelmValue(values, "logLevel", string(logging.Level)); err != nil {
			return err
		}
	}
	componentLevels := componentLogLevelsToString(logging.ComponentLevels)
	if componentLevels != "" {
		if err := setHelmValue(values, "componentLogLevel", componentLevels); err != nil {
			return err
		}
	}
	return nil
}

func componentLogLevelsToString(logLevels v2.ComponentLogLevels) string {
	componentLogLevels := make([]string, 0, len(logLevels))
	for component, level := range logLevels {
		componentLogLevels = append(componentLogLevels, fmt.Sprintf("%s:%s", component, level))
	}
	return strings.Join(componentLogLevels, ",")
}
