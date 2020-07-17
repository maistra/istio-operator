package conversion

import (
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

func populatePrometheusAddonValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	prometheus := in.Addons.Metrics.Prometheus
	if prometheus == nil {
		return nil
	}
	if prometheus.Address != nil {
		// XXX: not sure if this is correct. we don't want the charts processed,
		// but telemetry might be configured incorrectly
		if err := setHelmBoolValue(values, "prometheus.enabled", false); err != nil {
			return err
		}
		return setHelmStringValue(values, "kiali.prometheusAddr", *prometheus.Address)
	} else if prometheus.Install == nil {
		// XXX: not sure if this is correct. we don't want the charts processed,
		// but telemetry might be configured incorrectly
		return setHelmBoolValue(values, "prometheus.enabled", false)
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
