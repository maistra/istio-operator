package conversion

import (
	"fmt"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateAddonsValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	if in.Addons == nil {
		return nil
	}
	if in.Addons.Metrics.Prometheus != nil {
		if err := populatePrometheusAddonValues(in.Addons.Metrics.Prometheus, values); err != nil {
			return err
		}
	}
	switch in.Addons.Tracing.Type {
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

	if in.Addons.Visualization.Kiali != nil {
		if err := populateKialiAddonValues(in.Addons.Visualization.Kiali, values); err != nil {
			return err
		}
	}

	return nil
}
