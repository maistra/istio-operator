package conversion

import (
	"fmt"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateTracingValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	if in.Tracing != nil {
		if in.Tracing.Sampling != nil {
			if err := setHelmFloatValue(values, "pilot.traceSampling", float64(*in.Tracing.Sampling)/100.0); err != nil {
				return err
			}
		}
		switch in.Tracing.Type {
		case v2.TracerTypeNone:
			if err := setHelmBoolValue(values, "tracing.enabled", false); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "global.enableTracing", false); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "meshConfig.enableTracing", false); err != nil {
				return err
			}
			if err := setHelmStringValue(values, "tracing.provider", "none"); err != nil {
				return err
			}
			// Istio doesn't support "none". "zipkin" is the default.
			// See https://github.com/istio/istio/blob/d4d148f1c923bd9226c581b81067965daa4060df/operator/pkg/apis/istio/v1alpha1/values_types.pb.go#L66-L70
			if err := setHelmStringValue(values, "global.proxy.tracer", "zipkin"); err != nil {
				return err
			}
		case v2.TracerTypeJaeger:
			if err := setHelmStringValue(values, "tracing.provider", "jaeger"); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "tracing.enabled", true); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "global.enableTracing", true); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "meshConfig.enableTracing", true); err != nil {
				return err
			}
			if err := setHelmStringValue(values, "global.proxy.tracer", "zipkin"); err != nil {
				return err
			}
		case v2.TracerTypeStackdriver:
			if err := setHelmStringValue(values, "tracing.provider", "stackdriver"); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "tracing.enabled", true); err != nil {
				return err
			}
			if err := setHelmBoolValue(values, "global.enableTracing", true); err != nil {
				return err
			}
			if err := setHelmStringValue(values, "global.proxy.tracer", "stackdriver"); err != nil {
				return err
			}
		case "":
			// nothing to do
		default:
			return fmt.Errorf("unknown tracer type: %s", in.Tracing.Type)
		}
	}
	return nil
}

func populateTracingConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	tracing := &v2.TracingConfig{}
	setTracing := false
	if tracer, ok, err := in.GetAndRemoveString("tracing.provider"); ok && tracer != "" {
		if tracing.Type, err = tracerTypeFromString(tracer); err != nil {
			return err
		}
		setTracing = true
	} else if err != nil {
		return err
	} else if tracer, ok, err := in.GetAndRemoveString("global.proxy.tracer"); ok && tracer != "" {
		if tracing.Type, err = tracerTypeFromString(tracer); err != nil {
			return err
		}
		setTracing = true
	} else if err != nil {
		return err
	} else if traceEnabled, ok, err := in.GetAndRemoveBool("tracing.enabled"); ok {
		if traceEnabled {
			// default to jaeger if enabled and no proxy.tracer specified
			tracing.Type = v2.TracerTypeJaeger
		} else if zipkinAddress, ok, err := in.GetString("global.tracer.zipkin.address"); ok && zipkinAddress != "" {
			// configuration for external jaeger cr
			tracing.Type = v2.TracerTypeJaeger
		} else if err == nil {
			tracing.Type = v2.TracerTypeNone
		} else {
			return err
		}
		setTracing = true
	} else if err != nil {
		return err
	}

	if rawSampling, ok, err := in.GetAndRemoveFloat64("pilot.traceSampling"); ok {
		sampling := int32(rawSampling * 100.0)
		tracing.Sampling = &sampling
		setTracing = true
	} else if rawSampling, ok, newErr := in.GetAndRemoveInt64("pilot.traceSampling"); ok {
		// sampling: 0 - 100% = 0 - 10000, i.e. 1% = 100
		sampling := int32(rawSampling * 100)
		tracing.Sampling = &sampling
		setTracing = true
	} else if newErr != nil {
		return err
	}

	if setTracing {
		out.Tracing = tracing
	}

	// remove auto-populated fields
	in.RemoveField("global.enableTracing")
	in.RemoveField("meshConfig.enableTracing")
	in.RemoveField("global.proxy.tracer")
	in.RemoveField("tracing.enabled")

	return nil
}

func tracerTypeFromString(tracer string) (v2.TracerType, error) {
	switch strings.ToLower(tracer) {
	case "zipkin":
		return v2.TracerTypeJaeger, nil
	case strings.ToLower(string(v2.TracerTypeJaeger)):
		return v2.TracerTypeJaeger, nil
	case strings.ToLower(string(v2.TracerTypeStackdriver)):
		return v2.TracerTypeStackdriver, nil
	case strings.ToLower(string(v2.TracerTypeNone)):
		return v2.TracerTypeNone, nil
	}
	return v2.TracerTypeNone, fmt.Errorf("unknown tracer type %s", tracer)
}
