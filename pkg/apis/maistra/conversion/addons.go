package conversion

import (
	"fmt"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
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
		if err := setHelmStringValue(values, "tracing.provider", ""); err != nil {
			return err
		}
	case v2.TracerTypeJaeger:
		if err := populateJaegerAddonValues(in.Addons.Tracing.Jaeger, values); err != nil {
			return err
		}
	case "":
		// nothing to do
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

	if ingress.ContextPath != "" {
		if err := setHelmStringValue(values, "contextPath", ingress.ContextPath); err != nil {
			return err
		}
	}
	if len(ingress.Hosts) > 0 {
		if err := setHelmStringSliceValue(values, "hosts", ingress.Hosts); err != nil {
			return err
		}
	}
	if len(ingress.Metadata.Annotations) > 0 {
		if err := setHelmStringMapValue(values, "annotations", ingress.Metadata.Annotations); err != nil {
			return err
		}
	}
	if len(ingress.Metadata.Labels) > 0 {
		if err := setHelmStringMapValue(values, "labels", ingress.Metadata.Labels); err != nil {
			return err
		}
	}
	if len(ingress.TLS.GetContent()) > 0 {
		if err := setHelmValue(values, "tls", ingress.TLS.GetContent()); err != nil {
			return err
		}
	}
	return nil
}

func populateAddonsConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	addonsConfig := &v2.AddonsConfig{}
	if err := populateKialiAddonConfig(in, addonsConfig); err != nil {
		return err
	}
	if err := populatePrometheusAddonConfig(in, addonsConfig); err != nil {
		return err
	}
	if err := populateTracingAddonConfig(in, addonsConfig); err != nil {
		return err
	}
	if err := populateGrafanaAddonConfig(in, addonsConfig); err != nil {
		return err
	}

	if addonsConfig.Metrics.Prometheus != nil || addonsConfig.Tracing.Jaeger != nil ||
		addonsConfig.Visualization.Grafana != nil || addonsConfig.Visualization.Kiali != nil {
		out.Addons = addonsConfig
	}
	return nil
}

func populateAddonIngressConfig(in *v1.HelmValues, out *v2.ComponentIngressConfig) (bool, error) {
	setValues := false
	if enabled, ok, err := in.GetBool("enabled"); ok {
		out.Enabled = &enabled
		setValues = true
	} else if err != nil {
		return false, err
	}

	if contextPath, ok, err := in.GetString("contextPath"); ok {
		out.ContextPath = contextPath
		setValues = true
	} else if err != nil {
		return false, err
	}
	if hosts, ok, err := in.GetStringSlice("hosts"); ok {
		out.Hosts = append([]string{}, hosts...)
		setValues = true
	} else if err != nil {
		return false, err
	}

	if rawAnnotations, ok, err := in.GetMap("annotations"); ok && len(rawAnnotations) > 0 {
		if err := setMetadataAnnotations(rawAnnotations, &out.Metadata); err != nil {
			return false, err
		}
		setValues = true
	} else if err != nil {
		return false, err
	}

	if rawLabels, ok, err := in.GetMap("labels"); ok && len(rawLabels) > 0 {
		if err := setMetadataLabels(rawLabels, &out.Metadata); err != nil {
			return false, err
		}
		setValues = true
	} else if err != nil {
		return false, err
	}

	if tls, ok, err := in.GetMap("tls"); ok && len(tls) > 0 {
		out.TLS = v1.NewHelmValues(tls)
		setValues = true
	} else if err != nil {
		return false, err
	}

	return setValues, nil
}
