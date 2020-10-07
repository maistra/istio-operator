package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateStackDriverAddonValues(stackdriver *v2.StackdriverAddonConfig, values map[string]interface{}) (reterr error) {
	if stackdriver == nil {
		return nil
	}
	if stackdriver.Telemetry != nil {
		telemetry := stackdriver.Telemetry
		if telemetry.Enabled != nil {
			if err := setHelmBoolValue(values, "mixer.adapters.stackdriver.enabled", *telemetry.Enabled); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v2.stackdriver.enabled", *telemetry.Enabled); err != nil {
				return err
			}
		}
		if telemetry.EnableContextGraph != nil {
			if err := setHelmBoolValue(values, "mixer.adapters.stackdriver.contextGraph.enabled", *telemetry.EnableContextGraph); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v2.stackdriver.topology", *telemetry.EnableContextGraph); err != nil {
				return err
			}
		}
		if telemetry.EnableLogging != nil {
			if err := setHelmBoolValue(values, "mixer.adapters.stackdriver.logging.enabled", *telemetry.EnableLogging); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v2.stackdriver.logging", *telemetry.EnableLogging); err != nil {
				return err
			}
		}
		if telemetry.EnableMetrics != nil {
			if err := setHelmBoolValue(values, "mixer.adapters.stackdriver.metrics.enabled", *telemetry.EnableMetrics); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "telemetry.v2.stackdriver.monitoring", *telemetry.EnableMetrics); err != nil {
				return err
			}
		}
		if telemetry.ConfigOverride != nil && len(telemetry.ConfigOverride.GetContent()) > 0 {
			if err := overwriteHelmValues(values, telemetry.ConfigOverride.GetContent(), "telemetry", "v2", "stackdriver", "configOverride"); err != nil {
				return err
			}
		}
		if telemetry.Auth != nil {
			auth := telemetry.Auth
			if auth.AppCredentials != nil {
				if err := setHelmBoolValue(values, "mixer.adapters.stackdriver.auth.appCredentials", *auth.AppCredentials); err != nil {
					return err
				}
			}
			if auth.APIKey != "" {
				if err := setHelmStringValue(values, "mixer.adapters.stackdriver.auth.apiKey", auth.APIKey); err != nil {
					return err
				}
			}
			if auth.ServiceAccountPath != "" {
				if err := setHelmStringValue(values, "mixer.adapters.stackdriver.auth.serviceAccountPath", auth.ServiceAccountPath); err != nil {
					return err
				}
			}
		}
		if telemetry.AccessLogging != nil {
			accessLogging := telemetry.AccessLogging
			if accessLogging.Enabled != nil {
				if err := setHelmBoolValue(values, "telemetry.v2.accessLogPolicy.enabled", *accessLogging.Enabled); err != nil {
					return err
				}
			}
			if accessLogging.LogWindowDuration != "" {
				if err := setHelmStringValue(values, "telemetry.v2.accessLogPolicy.logWindowDuration", accessLogging.LogWindowDuration); err != nil {
					return err
				}
			}
		}
	}

	if tracerType, ok, _ := v1.NewHelmValues(values).GetString("global.proxy.tracer"); ok && tracerType == "stackdriver" {
		if err := setHelmBoolValue(values, "mixer.adapters.stackdriver.tracer.enabled", true); err != nil {
			return err
		}
		if traceSampling, ok, _ := v1.NewHelmValues(values).GetFloat64("pilot.traceSampling"); ok {
			if err := setHelmFloatValue(values, "mixer.adapters.stackdriver.tracer.sampleProbability", traceSampling); err != nil {
				return err
			}
		}
	}
	if stackdriver.Tracer != nil {
		tracer := stackdriver.Tracer
		if tracer.Debug != nil {
			if err := setHelmBoolValue(values, "global.tracer.stackdriver.debug", *tracer.Debug); err != nil {
				return err
			}
		}
		if tracer.MaxNumberOfAttributes != nil {
			if err := setHelmIntValue(values, "global.tracer.stackdriver.maxNumberOfAttributes", *tracer.MaxNumberOfAttributes); err != nil {
				return err
			}
		}
		if tracer.MaxNumberOfAnnotations != nil {
			if err := setHelmIntValue(values, "global.tracer.stackdriver.maxNumberOfAnnotations", *tracer.MaxNumberOfAnnotations); err != nil {
				return err
			}
		}
		if tracer.MaxNumberOfMessageEvents != nil {
			if err := setHelmIntValue(values, "global.tracer.stackdriver.maxNumberOfMessageEvents", *tracer.MaxNumberOfMessageEvents); err != nil {
				return err
			}
		}
	}
	return nil
}

func populateStackdriverAddonConfig(in *v1.HelmValues, out *v2.StackdriverAddonConfig) (bool, error) {
	setStackdriver := false
	telemetry := &v2.StackdriverTelemetryConfig{}
	setTelemetry := false

	if enabled, ok, err := in.GetAndRemoveBool("telemetry.v2.stackdriver.enabled"); ok {
		telemetry.Enabled = &enabled
		setTelemetry = true
	} else if err != nil {
		return false, err
	} else if enabled, ok, err := in.GetAndRemoveBool("mixer.adapters.stackdriver.enabled"); ok {
		telemetry.Enabled = &enabled
		setTelemetry = true
	} else if err != nil {
		return false, err
	}

	if contextGraph, ok, err := in.GetAndRemoveBool("telemetry.v2.stackdriver.topology"); ok {
		telemetry.EnableContextGraph = &contextGraph
		setTelemetry = true
	} else if err != nil {
		return false, err
	} else if contextGraph, ok, err := in.GetAndRemoveBool("mixer.adapters.stackdriver.contextGraph.enabled"); ok {
		telemetry.EnableContextGraph = &contextGraph
		setTelemetry = true
	} else if err != nil {
		return false, err
	}

	if logging, ok, err := in.GetAndRemoveBool("telemetry.v2.stackdriver.logging"); ok {
		telemetry.EnableLogging = &logging
		setTelemetry = true
	} else if err != nil {
		return false, err
	} else if logging, ok, err := in.GetAndRemoveBool("mixer.adapters.stackdriver.logging.enabled"); ok {
		telemetry.EnableLogging = &logging
		setTelemetry = true
	} else if err != nil {
		return false, err
	}

	if metrics, ok, err := in.GetAndRemoveBool("telemetry.v2.stackdriver.monitoring"); ok {
		telemetry.EnableMetrics = &metrics
		setTelemetry = true
	} else if err != nil {
		return false, err
	} else if metrics, ok, err := in.GetAndRemoveBool("mixer.adapters.stackdriver.metrics.enabled"); ok {
		telemetry.EnableMetrics = &metrics
		setTelemetry = true
	} else if err != nil {
		return false, err
	}

	if configOverride, ok, err := in.GetMap("telemetry.v2.stackdriver.configOverride"); ok && len(configOverride) > 0 {
		telemetry.ConfigOverride = v1.NewHelmValues(configOverride)
		setTelemetry = true
		in.RemoveField("telemetry.v2.stackdriver.configOverride")
	} else if err != nil {
		return false, err
	}

	auth := &v2.StackdriverAuthConfig{}
	setAuth := false
	if appCredentials, ok, err := in.GetAndRemoveBool("mixer.adapters.stackdriver.auth.appCredentials"); ok {
		auth.AppCredentials = &appCredentials
		setAuth = true
	} else if err != nil {
		return false, err
	}

	if apiKey, ok, err := in.GetAndRemoveString("mixer.adapters.stackdriver.auth.apiKey"); ok {
		auth.APIKey = apiKey
		setAuth = true
	} else if err != nil {
		return false, err
	}

	if serviceAccountPath, ok, err := in.GetAndRemoveString("mixer.adapters.stackdriver.auth.serviceAccountPath"); ok {
		auth.ServiceAccountPath = serviceAccountPath
		setAuth = true
	} else if err != nil {
		return false, err
	}

	if setAuth {
		telemetry.Auth = auth
		setTelemetry = true
	}

	accessLogging := &v2.StackdriverAccessLogTelemetryConfig{}
	setAccessLogging := false
	if enabled, ok, err := in.GetAndRemoveBool("telemetry.v2.accessLogPolicy.enabled"); ok {
		accessLogging.Enabled = &enabled
		setAccessLogging = true
	} else if err != nil {
		return false, err
	}
	if logWindowDuration, ok, err := in.GetAndRemoveString("telemetry.v2.accessLogPolicy.logWindowDuration"); ok {
		accessLogging.LogWindowDuration = logWindowDuration
		setAccessLogging = true
	} else if err != nil {
		return false, err
	}
	if setAccessLogging {
		telemetry.AccessLogging = accessLogging
		setTelemetry = true
	}

	if setTelemetry {
		out.Telemetry = telemetry
		setStackdriver = true
	}

	tracer := &v2.StackdriverTracerConfig{}
	setTracer := false
	if debug, ok, err := in.GetAndRemoveBool("global.tracer.stackdriver.debug"); ok {
		tracer.Debug = &debug
		setTracer = true
	} else if err != nil {
		return false, err
	}
	if maxNumberOfAttributes, ok, err := in.GetAndRemoveInt64("global.tracer.stackdriver.maxNumberOfAttributes"); ok {
		tracer.MaxNumberOfAttributes = &maxNumberOfAttributes
		setTracer = true
	} else if err != nil {
		return false, err
	}
	if maxNumberOfAnnotations, ok, err := in.GetAndRemoveInt64("global.tracer.stackdriver.maxNumberOfAnnotations"); ok {
		tracer.MaxNumberOfAnnotations = &maxNumberOfAnnotations
		setTracer = true
	} else if err != nil {
		return false, err
	}
	if maxNumberOfMessageEvents, ok, err := in.GetAndRemoveInt64("global.tracer.stackdriver.maxNumberOfMessageEvents"); ok {
		tracer.MaxNumberOfMessageEvents = &maxNumberOfMessageEvents
		setTracer = true
	} else if err != nil {
		return false, err
	}
	if setTracer {
		out.Tracer = tracer
		setStackdriver = true
	}

	// remove auto-populated/duplicate fields
	in.RemoveField("mixer.adapters.stackdriver.enabled")
	in.RemoveField("mixer.adapters.stackdriver.contextGraph.enabled")
	in.RemoveField("mixer.adapters.stackdriver.logging.enabled")
	in.RemoveField("mixer.adapters.stackdriver.metrics.enabled")
	in.RemoveField("mixer.adapters.stackdriver.tracer.enabled")
	in.RemoveField("mixer.adapters.stackdriver.tracer.sampleProbability")

	return setStackdriver, nil
}
