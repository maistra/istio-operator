package conversion

import (
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateGrafanaAddonValues(grafana *v2.GrafanaAddonConfig, values map[string]interface{}) error {
	if grafana == nil {
		return nil
	}

	if grafana.Address != nil {
		if err := setHelmBoolValue(values, "grafana.enabled", false); err != nil {
			return err
		}
		return setHelmStringValue(values, "kiali.dashboard.grafanaURL", *grafana.Address)
	} else if grafana.Install == nil {
		// we don't want to process the charts
		return setHelmBoolValue(values, "grafana.enabled", false)
	}
	grafanaValues := make(map[string]interface{})
	if grafana.Enabled != nil {
		if err := setHelmBoolValue(grafanaValues, "enabled", *grafana.Enabled); err != nil {
			return err
		}
	}
	if len(grafana.Install.Config.Env) > 0 {
		if err := setHelmMapValue(grafanaValues, "env", grafana.Install.Config.Env); err != nil {
			return err
		}
	}
	if len(grafana.Install.Config.EnvSecrets) > 0 {
		if err := setHelmMapValue(grafanaValues, "envSecrets", grafana.Install.Config.EnvSecrets); err != nil {
			return err
		}
	}
	if grafana.Install.Persistence != nil {
		if grafana.Install.Persistence.Enabled != nil {
			if err := setHelmBoolValue(grafanaValues, "persist", *grafana.Install.Persistence.Enabled); err != nil {
				return err
			}
		}
		if grafana.Install.Persistence.StorageClassName != "" {
			if err := setHelmStringValue(grafanaValues, "storageClassName", grafana.Install.Persistence.StorageClassName); err != nil {
				return err
			}
		}
		if grafana.Install.Persistence.AccessMode != "" {
			if err := setHelmStringValue(grafanaValues, "accessMode", string(grafana.Install.Persistence.AccessMode)); err != nil {
				return err
			}
		}
		if len(grafana.Install.Persistence.Capacity) > 0 {
			if capacityValues, err := toValues(grafana.Install.Persistence.Capacity); err == nil {
				if len(capacityValues) > 0 {
					if err := setHelmValue(grafanaValues, "storageCapacity", capacityValues); err != nil {
						return err
					}
				}
			} else {
				return err
			}
		}
	}
	if grafana.Install.Service.Ingress != nil {
		ingressValues := make(map[string]interface{})
		if err := populateAddonIngressValues(grafana.Install.Service.Ingress, ingressValues); err == nil {
			if len(ingressValues) > 0 {
				if err := setHelmValue(grafanaValues, "ingress", ingressValues); err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	// XXX: skipping most service settings for now
	if len(grafana.Install.Service.Metadata.Annotations) > 0 {
		if err := setHelmMapValue(grafanaValues, "service.annotations", grafana.Install.Service.Metadata.Annotations); err != nil {
			return err
		}
	}
	if grafana.Install.Security != nil {
		if err := setHelmBoolValue(grafanaValues, "security.enabled", true); err != nil {
			return err
		}
		if grafana.Install.Security.SecretName != "" {
			if err := setHelmStringValue(grafanaValues, "security.secretName", grafana.Install.Security.SecretName); err != nil {
				return err
			}
		}
		if grafana.Install.Security.UsernameKey != "" {
			if err := setHelmStringValue(grafanaValues, "security.usernameKey", grafana.Install.Security.UsernameKey); err != nil {
				return err
			}
		}
		if grafana.Install.Security.PassphraseKey != "" {
			if err := setHelmStringValue(grafanaValues, "security.passphraseKey", grafana.Install.Security.PassphraseKey); err != nil {
				return err
			}
		}
	}

	if err := populateRuntimeValues(grafana.Install.Runtime, grafanaValues); err != nil {
		return err
	}

	if len(grafanaValues) > 0 {
		if err := setHelmValue(values, "grafana", grafanaValues); err != nil {
			return err
		}
	}

	return nil
}
