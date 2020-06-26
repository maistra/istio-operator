package conversion

import (
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateLoggingValues(logging *v2.LoggingConfig, values map[string]interface{}) error {
	if logging == nil {
		return nil
	}
	if logging.Level != "" {
		if err := setHelmValue(values, "logLevel", string(logging.Level)); err != nil {
			return err
		}
	}
	if len(logging.ComponentLevel) > 0 {
		componentLogLevel := make(map[string]interface{})
		for component, level := range logging.ComponentLevel {
			if err := setHelmValue(componentLogLevel, string(component), string(level)); err != nil {
				return err
			}

		}
    }
    
    // XXX: LogAsJSON is a global :(

	return nil
}
