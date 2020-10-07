package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func populatePrometheusAddonValues(in *v2.ControlPlaneSpec, values map[string]interface{}) (reterr error) {
	if in.Addons == nil {
		return nil
	}
	prometheus := in.Addons.Prometheus
	if prometheus == nil {
		return nil
	}
	prometheusValues := make(map[string]interface{})
	if prometheus.Enabled != nil {
		if err := setHelmBoolValue(prometheusValues, "enabled", *prometheus.Enabled); err != nil {
			return err
		}
	}
	defer func() {
		if reterr == nil {
			if len(prometheusValues) > 0 {
				if err := overwriteHelmValues(values, prometheusValues, "prometheus"); err != nil {
					reterr = err
				}
			}
		}
	}()

	if prometheus.Scrape != nil {
		if err := setHelmBoolValue(values, "meshConfig.enablePrometheusMerge", *prometheus.Scrape); err != nil {
			return err
		}
	}

	// telemetry
	if err := populatePrometheusTelemetryValues(prometheus, values); err != nil {
		return err
	}

	// install takes precedence
	if prometheus.Install == nil {
		if prometheus.Address != nil {
			return setHelmStringValue(values, "kiali.prometheusAddr", *prometheus.Address)
		}
		return nil
	}

	if prometheus.Install.Retention != "" {
		if err := setHelmStringValue(prometheusValues, "retention", prometheus.Install.Retention); err != nil {
			return err
		}
	}
	if prometheus.Install.ScrapeInterval != "" {
		if err := setHelmStringValue(prometheusValues, "scrapeInterval", prometheus.Install.ScrapeInterval); err != nil {
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

	if err := populateComponentServiceValues(&prometheus.Install.Service, prometheusValues); err != nil {
		return err
	}
	return nil
}

func populatePrometheusTelemetryValues(in *v2.PrometheusAddonConfig, values map[string]interface{}) error {
	if in == nil {
		return nil
	}
	if in.Enabled != nil {
		if err := setHelmBoolValue(values, "mixer.adapters.prometheus.enabled", *in.Enabled); err != nil {
			return err
		}
		if err := setHelmBoolValue(values, "telemetry.v2.prometheus.enabled", *in.Enabled); err != nil {
			return err
		}
	}
	telemetry := in.TelemetryConfig
	if telemetry == nil {
		return nil
	}
	if telemetry.MetricsExpiryDuration != "" {
		if err := setHelmStringValue(values, "mixer.adapters.prometheus.metricsExpiryDuration", telemetry.MetricsExpiryDuration); err != nil {
			return err
		}
	}
	return nil
}

func populatePrometheusAddonConfig(in *v1.HelmValues, out *v2.PrometheusAddonConfig) (bool, error) {
	prometheus := out
	setPrometheus := false

	if enablePrometheusMerge, ok, err := in.GetAndRemoveBool("meshConfig.enablePrometheusMerge"); ok {
		prometheus.Scrape = &enablePrometheusMerge
		setPrometheus = true
	} else if err != nil {
		return false, err
	}

	telemetry := &v2.PrometheusTelemetryConfig{}
	if updated, err := populatePrometheusTelemetryConfig(in, telemetry); updated {
		prometheus.TelemetryConfig = telemetry
		setPrometheus = true
	} else if err != nil {
		return false, err
	}

	// check to see if prometheus.Address should be set
	if address, ok, err := in.GetAndRemoveString("kiali.prometheusAddr"); ok {
		// If grafana URL is set, assume we're using an existing grafana install
		prometheus.Address = &address
		setPrometheus = true
	} else if err != nil {
		return false, err
	}

	rawPrometheusValues, ok, err := in.GetMap("prometheus")
	if err != nil {
		return false, err
	} else if !ok || len(rawPrometheusValues) == 0 {
		// nothing to do
		return setPrometheus, nil
	}
	prometheusValues := v1.NewHelmValues(rawPrometheusValues)

	if enabled, ok, err := prometheusValues.GetAndRemoveBool("enabled"); ok {
		prometheus.Enabled = &enabled
		setPrometheus = true
	} else if err != nil {
		return false, err
	}

	install := &v2.PrometheusInstallConfig{}
	setInstall := false

	if retention, ok, err := prometheusValues.GetAndRemoveString("retention"); ok {
		install.Retention = retention
		setInstall = true
	} else if err != nil {
		return false, err
	}
	if scrapeInterval, ok, err := prometheusValues.GetAndRemoveString("scrapeInterval"); ok {
		install.ScrapeInterval = scrapeInterval
		setInstall = true
	} else if err != nil {
		return false, err
	}

	if securityEnabled, ok, err := prometheusValues.GetAndRemoveBool("security.enabled"); ok {
		// v1_0 and v1_0
		install.UseTLS = &securityEnabled
		setInstall = true
	} else if err != nil {
		return false, err
	} else if provisionPrometheusCert, ok, err := prometheusValues.GetAndRemoveBool("provisionPrometheusCert"); ok {
		// v2_0
		install.UseTLS = &provisionPrometheusCert
		setInstall = true
	} else if err != nil {
		return false, err
	}
	if applied, err := populateComponentServiceConfig(prometheusValues, &install.Service); err == nil {
		setInstall = setInstall || applied
	} else {
		return false, err
	}

	if setInstall {
		prometheus.Install = install
		setPrometheus = true
	}
	// update the kiali settings
	if len(prometheusValues.GetContent()) == 0 {
		in.RemoveField("prometheus")
	} else if err := in.SetField("prometheus", prometheusValues.GetContent()); err != nil {
		return false, err
	}

	return setPrometheus, nil
}

func populatePrometheusTelemetryConfig(in *v1.HelmValues, telemetry *v2.PrometheusTelemetryConfig) (bool, error) {
	// remove auto-populated fields
	in.RemoveField("mixer.adapters.prometheus.enabled")
	in.RemoveField("telemetry.v2.prometheus.enabled")

	if metricsExpiryDuration, ok, err := in.GetAndRemoveString("mixer.adapters.prometheus.metricsExpiryDuration"); ok {
		telemetry.MetricsExpiryDuration = metricsExpiryDuration
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}
