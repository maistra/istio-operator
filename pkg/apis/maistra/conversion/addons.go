package conversion

import (
	"fmt"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateAddonsValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	if in.Addons == nil {
		return nil
	}

	// do kiali first, so it doesn't override any kiali.* settings added by other addons (e.g. prometheus, grafana, jaeger)
	// XXX: not sure how important this is, as these settings should be updated as part of reconcilation
	if in.Addons.Visualization.Kiali != nil {
		if err := populateKialiAddonValues(in.Addons.Visualization.Kiali, values); err != nil {
			return err
		}
	}

	if in.Addons.Metrics.Prometheus != nil {
		if err := populatePrometheusAddonValues(in, values); err != nil {
			return err
		}
	}
	switch in.Addons.Tracing.Type {
	case v2.TracerTypeNone:
		if err := setHelmBoolValue(values, "tracing.enabled", false); err != nil {
			return err
		}
	case v2.TracerTypeJaeger:
		if err := populateJaegerAddonValues(in.Addons.Tracing.Jaeger, values); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Unknown tracer type: %s", in.Addons.Tracing.Type)
	}

	if in.Addons.Visualization.Grafana != nil {
		if err := populateGrafanaAddonValues(in.Addons.Visualization.Grafana, values); err != nil {
			return err
		}
	}

	return nil
}

func populateAddonIngressValues(ingress *v2.ComponentIngressConfig, values map[string]interface{}) error {
	if ingress == nil {
		return nil
	}
	if ingress.Enabled != nil {
		if err := setHelmBoolValue(values, "enabled", *ingress.Enabled); err != nil {
			return err
		}
		if !*ingress.Enabled {
			return nil
		}
	}

	if err := setHelmBoolValue(values, "enabled", true); err != nil {
		return err
	}
	if ingress.ContextPath != "" {
		if err := setHelmStringValue(values, "contextPath", ingress.ContextPath); err != nil {
			return err
		}
	}
	if len(ingress.Hosts) > 0 {
		if err := setHelmSliceValue(values, "hosts", ingress.Hosts); err != nil {
			return err
		}
	}
	if len(ingress.Metadata.Annotations) > 0 {
		if err := setHelmMapValue(values, "annotations", ingress.Metadata.Annotations); err != nil {
			return err
		}
	}
	if len(ingress.TLS) > 0 {
		if err := setHelmMapValue(values, "tls", ingress.TLS); err != nil {
			return err
		}
	}
	return nil
}
