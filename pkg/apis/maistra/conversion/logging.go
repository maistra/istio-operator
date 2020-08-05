package conversion

import (
	"fmt"
	"sort"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateControlPlaneLogging(logging *v2.LoggingConfig, values map[string]interface{}) error {
	if logging == nil {
		return nil
	}
	componentLevels := componentLogLevelsToString(logging.ComponentLevels)
	if componentLevels != "" {
		if err := setHelmStringValue(values, "global.logging.level", componentLevels); err != nil {
			return err
		}
	}
	if logging.LogAsJSON != nil {
		if err := setHelmBoolValue(values, "global.logAsJson", *logging.LogAsJSON); err != nil {
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
		if err := setHelmStringValue(values, "logLevel", string(logging.Level)); err != nil {
			return err
		}
	}
	componentLevels := componentLogLevelsToString(logging.ComponentLevels)
	if componentLevels != "" {
		if err := setHelmStringValue(values, "componentLogLevel", componentLevels); err != nil {
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
	sort.Strings(componentLogLevels)
	return strings.Join(componentLogLevels, ",")
}

func componentLogLevelsFromString(levels string) v2.ComponentLogLevels {
	componentLevels := strings.Split(levels, ",")
	if len(componentLevels) == 0 {
		return nil
	}
	logLevels := v2.ComponentLogLevels{}
	for _, componentLevel := range componentLevels {
		pair := strings.SplitN(componentLevel, ":", 2)
		logLevels[v2.EnvoyComponent(pair[0])] = v2.LogLevel(pair[1])
	}
	return logLevels
}

func populateControlPlaneLoggingConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	logging := &v2.LoggingConfig{}
	setLogging := false

	if componentLevels, ok, err := in.GetString("global.logging.level"); ok && len(componentLevels) > 0 {
		logging.ComponentLevels = componentLogLevelsFromString(componentLevels)
		setLogging = true
	} else if err != nil {
		return err
	}
	if logAsJSON, ok, err := in.GetBool("global.logAsJson"); ok {
		logging.LogAsJSON = &logAsJSON
		setLogging = true
	} else if err != nil {
		return err
	}

	if setLogging {
		out.Logging = logging
	}

	return nil
}

func populateProxyLoggingConfig(proxyValues *v1.HelmValues, logging *v2.ProxyLoggingConfig) (bool, error) {
	setLogging := false
	if level, ok, err := proxyValues.GetString("logLevel"); ok {
		logging.Level = v2.LogLevel(level)
		setLogging = true
	} else if err != nil {
		return false, err
	}
	if componentLevels, ok, err := proxyValues.GetString("componentLogLevel"); ok && len(componentLevels) > 0 {
		logging.ComponentLevels = componentLogLevelsFromString(componentLevels)
		setLogging = true
	} else if err != nil {
		return false, err
	}
	return setLogging, nil
}
