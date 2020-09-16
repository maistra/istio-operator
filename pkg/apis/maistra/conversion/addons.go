package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateAddonsValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	if in.Addons == nil {
		return nil
	}

	// do kiali first, so it doesn't override any kiali.* settings added by other addons (e.g. prometheus, grafana, jaeger)
	// XXX: not sure how important this is, as these settings should be updated as part of reconcilation
	if in.Addons.Kiali != nil {
		if err := populateKialiAddonValues(in.Addons.Kiali, values); err != nil {
			return err
		}
	}
	if in.Addons.Grafana != nil {
		if err := populateGrafanaAddonValues(in.Addons.Grafana, values); err != nil {
			return err
		}
	}

	if in.Addons.Prometheus != nil {
		if err := populatePrometheusAddonValues(in, values); err != nil {
			return err
		}
	}

	if in.Addons.Jaeger != nil {
		if err := populateJaegerAddonValues(in.Addons.Jaeger, values); err != nil {
			return err
		}
	}

	if in.Addons.Stackdriver != nil {
		if err := populateStackDriverAddonValues(in.Addons.Stackdriver, values); err != nil {
			return err
		}
	}

	if in.Addons.ThreeScale != nil {
		if err := populateThreeScaleAddonValues(in.Addons.ThreeScale, values); err != nil {
			return err
		}
	}

	return nil
}

func populateAddonIngressValues(ingress *v2.ComponentIngressConfig, addonIngressValues map[string]interface{}) error {
	if ingress == nil {
		return nil
	}
	if ingress.Enabled != nil {
		if err := setHelmBoolValue(addonIngressValues, "enabled", *ingress.Enabled); err != nil {
			return err
		}
		if !*ingress.Enabled {
			return nil
		}
	}

	if ingress.ContextPath != "" {
		if err := setHelmStringValue(addonIngressValues, "contextPath", ingress.ContextPath); err != nil {
			return err
		}
	}
	if len(ingress.Hosts) > 0 {
		if err := setHelmStringSliceValue(addonIngressValues, "hosts", ingress.Hosts); err != nil {
			return err
		}
	}
	if len(ingress.Metadata.Annotations) > 0 {
		if err := setHelmStringMapValue(addonIngressValues, "annotations", ingress.Metadata.Annotations); err != nil {
			return err
		}
	}
	if len(ingress.Metadata.Labels) > 0 {
		if err := setHelmStringMapValue(addonIngressValues, "labels", ingress.Metadata.Labels); err != nil {
			return err
		}
	}
	if len(ingress.TLS.GetContent()) > 0 {
		if err := setHelmValue(addonIngressValues, "tls", ingress.TLS.GetContent()); err != nil {
			return err
		}
	}
	return nil
}

func populateAddonsConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	addonsConfig := &v2.AddonsConfig{}
	setAddons := false
	kiali := &v2.KialiAddonConfig{}
	if updated, err := populateKialiAddonConfig(in, kiali); updated {
		addonsConfig.Kiali = kiali
		setAddons = true
	} else if err != nil {
		return err
	}
	prometheus := &v2.PrometheusAddonConfig{}
	if updated, err := populatePrometheusAddonConfig(in, prometheus); updated {
		addonsConfig.Prometheus = prometheus
		setAddons = true
	} else if err != nil {
		return err
	}
	jaeger := &v2.JaegerAddonConfig{}
	if updated, err := populateJaegerAddonConfig(in, jaeger); updated {
		addonsConfig.Jaeger = jaeger
		setAddons = true
	} else if err != nil {
		return err
	}
	stackdriver := &v2.StackdriverAddonConfig{}
	if updated, err := populateStackdriverAddonConfig(in, stackdriver); updated {
		addonsConfig.Stackdriver = stackdriver
		setAddons = true
	} else if err != nil {
		return err
	}
	grafana := &v2.GrafanaAddonConfig{}
	if updated, err := populateGrafanaAddonConfig(in, grafana); updated {
		addonsConfig.Grafana = grafana
		setAddons = true
	} else if err != nil {
		return err
	}
	threeScale := &v2.ThreeScaleAddonConfig{}
	if updated, err := populateThreeScaleAddonConfig(in, threeScale); updated {
		addonsConfig.ThreeScale = threeScale
		setAddons = true
	} else if err != nil {
		return err
	}

	if setAddons {
		out.Addons = addonsConfig
	}

	// HACK - remove grafana component's runtime env, as it is incorporated into
	// the grafana config directly
	if out.Runtime != nil && out.Runtime.Components != nil {
		if grafanaComponentConfig, ok := out.Runtime.Components[v2.ControlPlaneComponentNameGrafana]; ok && grafanaComponentConfig.Container != nil {
			grafanaComponentConfig.Container.Env = nil
		}
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
