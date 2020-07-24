package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func populatePrometheusAddonValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	prometheus := in.Addons.Metrics.Prometheus
	if prometheus == nil {
		return nil
	}
	// install takes precedence
	if prometheus.Install == nil {
		// XXX: not sure if this is correct. we don't want the charts processed,
		// but telemetry might be configured incorrectly
		if err := setHelmBoolValue(values, "prometheus.enabled", false); err != nil {
			return err
		}
		if prometheus.Address != nil {
			return setHelmStringValue(values, "kiali.prometheusAddr", *prometheus.Address)
		}
		return nil
	}

	prometheusValues := make(map[string]interface{})
	if prometheus.Enabled != nil {
		if err := setHelmBoolValue(prometheusValues, "enabled", *prometheus.Enabled); err != nil {
			return err
		}
	}
	if prometheus.Install.Config.Retention != "" {
		if err := setHelmStringValue(prometheusValues, "retention", prometheus.Install.Config.Retention); err != nil {
			return err
		}
	}
	if prometheus.Install.Config.ScrapeInterval != "" {
		if err := setHelmStringValue(prometheusValues, "scrapeInterval", prometheus.Install.Config.ScrapeInterval); err != nil {
			return err
		}
	}
	if prometheus.Install.UseTLS != nil {
		if in.Version == "" || in.Version == versions.V1_0.String() || in.Version == versions.V1_1.String() {
			if err := setHelmBoolValue(prometheusValues, "security.enabled", *prometheus.Install.UseTLS); err != nil {
				return err
			}
		} else {
			if err := setHelmBoolValue(prometheusValues, "provisionPrometheusCert", *prometheus.Install.UseTLS); err != nil {
				return err
			}
		}
	}
	if err := populateRuntimeValues(prometheus.Install.Runtime, prometheusValues); err != nil {
		return err
	}
	if len(prometheus.Install.Service.Metadata.Annotations) > 0 {
		if err := setHelmStringMapValue(prometheusValues, "service.annotations", prometheus.Install.Service.Metadata.Annotations); err != nil {
			return err
		}
	}
	if prometheus.Install.Service.NodePort != nil {
		if *prometheus.Install.Service.NodePort == 0 {
			if err := setHelmBoolValue(prometheusValues, "service.nodePort.enabled", false); err != nil {
				return err
			}
		} else {
			if err := setHelmBoolValue(prometheusValues, "service.nodePort.enabled", true); err != nil {
				return err
			}
			if err := setHelmIntValue(prometheusValues, "service.nodePort.port", int64(*prometheus.Install.Service.NodePort)); err != nil {
				return err
			}
		}
	}
	ingressValues := make(map[string]interface{})
	if err := populateAddonIngressValues(prometheus.Install.Service.Ingress, ingressValues); err == nil {
		if len(ingressValues) > 0 {
			if err := setHelmValue(prometheusValues, "ingress", ingressValues); err != nil {
				return err
			}
		}
	} else {
		return err
	}

	if len(prometheusValues) > 0 {
		if err := setHelmValue(values, "prometheus", prometheusValues); err != nil {
			return err
		}
	}
	return nil
}

func populatePrometheusAddonConfig(in *v1.HelmValues, out *v2.AddonsConfig) error {
	rawPrometheusValues, ok, err := in.GetMap("prometheus")
	if err != nil {
		return err
	} else if !ok || len(rawPrometheusValues) == 0 {
		// nothing to do
		// check to see if grafana.Address should be set
		if address, ok, err := in.GetString("kiali.prometheusAddr"); ok {
			// If grafana URL is set, assume we're using an existing grafana install
			out.Metrics.Prometheus = &v2.PrometheusAddonConfig{
				Address: &address,
			}
		} else if err != nil {
			return err
		}
		return nil
	}
	prometheusValues := v1.NewHelmValues(rawPrometheusValues)

	if out.Metrics.Prometheus == nil {
		out.Metrics.Prometheus = &v2.PrometheusAddonConfig{}
	}
	prometheus := out.Metrics.Prometheus

	if enabled, ok, err := prometheusValues.GetBool("enabled"); ok {
		prometheus.Enabled = &enabled
	} else if err != nil {
		return err
	}

	install := &v2.PrometheusInstallConfig{}
	// for v1, there's no way to disable install, so always create install
	setInstall := true

	if retention, ok, err := prometheusValues.GetString("retention"); ok {
		install.Config.Retention = retention
		setInstall = true
	} else if err != nil {
		return err
	}
	if scrapeInterval, ok, err := prometheusValues.GetString("scrapeInterval"); ok {
		install.Config.ScrapeInterval = scrapeInterval
		setInstall = true
	} else if err != nil {
		return err
	}

	if securityEnabled, ok, err := prometheusValues.GetBool("security.enabled"); ok {
		// v1_0 and v1_0
		install.UseTLS = &securityEnabled
		setInstall = true
	} else if err != nil {
		return err
	} else if provisionPrometheusCert, ok, err := prometheusValues.GetBool("provisionPrometheusCert"); ok {
		// v2_0
		install.UseTLS = &provisionPrometheusCert
		setInstall = true
	} else if err != nil {
		return err
	}

	runtime := &v2.ComponentRuntimeConfig{}
	if applied, err := runtimeValuesToComponentRuntimeConfig(prometheusValues, runtime); err != nil {
		return err
	} else if applied {
		install.Runtime = runtime
		setInstall = true
	}

	if applied, err := populateComponentServiceConfig(prometheusValues, &install.Service); err == nil {
		setInstall = setInstall || applied
	} else {
		return err
	}

	if setInstall {
		prometheus.Install = install
	}

	return nil
}
