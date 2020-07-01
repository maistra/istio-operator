package conversion

import (
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateGrafanaAddonValues(grafana *v2.GrafanaAddonConfig, values map[string]interface{}) error {
	if grafana == nil {
		return setHelmValue(values, "grafana.enabled", false)
	}
	if grafana.Address != nil {
		if err := setHelmValue(values, "grafana.enabled", false); err != nil {
			return err
		}
		return setHelmValue(values, "kiali.dashboard.grafanaURL", *grafana.Address)
	} else if grafana.Install == nil {
		return nil
	}
	grafanaValues := make(map[string]interface{})
	if err := setHelmValue(grafanaValues, "enabled", true); err != nil {
		return err
	}
	if len(grafana.Install.Config.Env) > 0 {
		if err := setHelmValue(grafanaValues, "env", grafana.Install.Config.Env); err != nil {
			return err
		}
	}
	if len(grafana.Install.Config.EnvSecrets) > 0 {
		if err := setHelmValue(grafanaValues, "envSecrets", grafana.Install.Config.Env); err != nil {
			return err
		}
	}
	if grafana.Install.Persistence != nil {
		if err := setHelmValue(grafanaValues, "persist", true); err != nil {
			return err
		}
		if grafana.Install.Persistence.StorageClassName != "" {
			if err := setHelmValue(grafanaValues, "storageClassName", true); err != nil {
				return err
			}
		}
		if grafana.Install.Persistence.AccessMode != "" {
			if err := setHelmValue(grafanaValues, "accessMode", string(grafana.Install.Persistence.AccessMode)); err != nil {
				return err
			}
		}
		if len(grafana.Install.Persistence.Capacity) > 0 {
			if capacityValues, err := toValues(grafana.Install.Persistence.Capacity); err == nil {
				if err := setHelmValue(values, "storageCapacity", capacityValues); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	if grafana.Install.Service.Ingress != nil {
		ingressValues := make(map[string]interface{})
		if err := populateAddonIngressValues(grafana.Install.Service.Ingress, ingressValues); err == nil {
			if err := setHelmValue(grafanaValues, "ingress", ingressValues); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	// XXX: skipping most service settings for now
	if len(grafana.Install.Service.Metadata.Annotations) > 0 {
		if err := setHelmValue(grafanaValues, "service.annotations", grafana.Install.Service.Metadata.Annotations); err != nil {
			return err
		}
	}
	if grafana.Install.Security != nil {
		if err := setHelmValue(grafanaValues, "security.enabled", true); err != nil {
			return err
		}
		if grafana.Install.Security.SecretName != "" {
			if err := setHelmValue(grafanaValues, "security.secretName", grafana.Install.Security.SecretName); err != nil {
				return err
			}
		}
		if grafana.Install.Security.UsernameKey != "" {
			if err := setHelmValue(grafanaValues, "security.usernameKey", grafana.Install.Security.UsernameKey); err != nil {
				return err
			}
		}
		if grafana.Install.Security.PassphraseKey != "" {
			if err := setHelmValue(grafanaValues, "security.passphraseKey", grafana.Install.Security.PassphraseKey); err != nil {
				return err
			}
		}
	}

	if err := populateRuntimeValues(grafana.Install.Runtime, grafanaValues); err != nil {
		return err
	}

	return nil
}
