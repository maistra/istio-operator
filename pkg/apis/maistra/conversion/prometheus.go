package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func populatePrometheusAddonValues(in *v2.ControlPlaneSpec, values map[string]interface{}) (reterr error) {
	prometheus := in.Addons.Metrics.Prometheus
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
				if err := setHelmValue(values, "prometheus", prometheusValues); err != nil {
					reterr = err
				}
			}
		}
	}()
	// install takes precedence
	if prometheus.Install == nil {
		if prometheus.Address != nil {
			return setHelmStringValue(values, "kiali.prometheusAddr", *prometheus.Address)
		}
		return nil
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
	// Deployment specific settings
	runtime := prometheus.Install.Runtime
	if runtime != nil {
		if err := populateRuntimeValues(runtime, prometheusValues); err != nil {
			return err
		}

		// set image and resources
		if runtime.Pod.Containers != nil {
			if container, ok := runtime.Pod.Containers["prometheus"]; ok {
				if err := populateContainerConfigValues(&container, prometheusValues); err != nil {
					return err
				}
			}
		}
	}
	if err := populateComponentServiceValues(&prometheus.Install.Service, prometheusValues); err != nil {
		return err
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
	setInstall := false

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
	container := v2.ContainerConfig{}
	// non-istiod
	if applied, err := populateContainerConfig(prometheusValues, &container); err != nil {
		return err
	} else if applied {
		if install.Runtime == nil {
			install.Runtime = runtime
			runtime.Pod.Containers = make(map[string]v2.ContainerConfig)
		} else if runtime.Pod.Containers == nil {
			runtime.Pod.Containers = make(map[string]v2.ContainerConfig)
		}
		install.Runtime.Pod.Containers["prometheus"] = container
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
