package conversion

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateGrafanaAddonValues(grafana *v2.GrafanaAddonConfig, values map[string]interface{}) (reterr error) {
	if grafana == nil {
		return nil
	}

	grafanaValues := make(map[string]interface{})
	if grafana.Enabled != nil {
		if err := setHelmBoolValue(grafanaValues, "enabled", *grafana.Enabled); err != nil {
			return err
		}
	}
	defer func() {
		if reterr == nil {
			if len(grafanaValues) > 0 {
				if err := overwriteHelmValues(values, grafanaValues, "grafana"); err != nil {
					reterr = err
				}
			}
		}
	}()

	// Install takes precedence
	if grafana.Install == nil {
		// we don't want to process the charts
		if grafana.Address != nil {
			return setHelmStringValue(values, "kiali.dashboard.grafanaURL", *grafana.Address)
		}
		return nil
	}

	if grafana.Install.Config != nil {
		if len(grafana.Install.Config.Env) > 0 {
			if err := setHelmStringMapValue(grafanaValues, "env", grafana.Install.Config.Env); err != nil {
				return err
			}
		}
		if len(grafana.Install.Config.EnvSecrets) > 0 {
			if err := setHelmStringMapValue(grafanaValues, "envSecrets", grafana.Install.Config.EnvSecrets); err != nil {
				return err
			}
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
		if grafana.Install.Persistence.Resources != nil {
			if resourcesValues, err := toValues(grafana.Install.Persistence.Resources); err == nil {
				if len(resourcesValues) > 0 {
					if err := setHelmValue(grafanaValues, "persistenceResources", resourcesValues); err != nil {
						return err
					}
				}
			} else {
				return err
			}
		}
	}
	if grafana.Install.Service != nil {
		if err := populateComponentServiceValues(grafana.Install.Service, grafanaValues); err != nil {
			return err
		}
	}
	if grafana.Install.Security != nil {
		if grafana.Install.Security.Enabled != nil {
			if err := setHelmBoolValue(grafanaValues, "security.enabled", *grafana.Install.Security.Enabled); err != nil {
				return err
			}
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

	return nil
}

func populateGrafanaAddonConfig(in *v1.HelmValues, out *v2.GrafanaAddonConfig) (bool, error) {
	rawGrafanaValues, ok, err := in.GetMap("grafana")
	if err != nil {
		return false, err
	} else if !ok || len(rawGrafanaValues) == 0 {
		// nothing to do
		// check to see if grafana.Address should be set
		if address, ok, err := in.GetAndRemoveString("kiali.dashboard.grafanaURL"); ok {
			// If grafana URL is set, assume we're using an existing grafana install
			out.Address = &address
			return true, nil
		} else if err != nil {
			return false, err
		}
		return false, nil
	}

	grafana := out
	grafanaValues := v1.NewHelmValues(rawGrafanaValues)
	if enabled, ok, err := grafanaValues.GetAndRemoveBool("enabled"); ok {
		grafana.Enabled = &enabled
	} else if err != nil {
		return false, err
	}

	if address, ok, err := in.GetAndRemoveString("kiali.dashboard.grafanaURL"); ok {
		// If grafana URL is set, assume we're using an existing grafana install
		grafana.Address = &address
		grafana.Install = nil
		return true, nil
	} else if err != nil {
		return false, err
	}

	install := &v2.GrafanaInstallConfig{}
	setInstall := false

	config := &v2.GrafanaConfig{}
	setConfig := false
	if rawEnv, ok, err := grafanaValues.GetMap("env"); ok && len(rawEnv) > 0 {
		setConfig = true
		config.Env = make(map[string]string)
		for key, value := range rawEnv {
			if stringValue, ok := value.(string); ok {
				config.Env[key] = stringValue
			} else {
				return false, fmt.Errorf("error casting env value to string")
			}
		}
	} else if err != nil {
		return false, err
	}
	grafanaValues.RemoveField("env")
	if rawEnv, ok, err := grafanaValues.GetMap("envSecrets"); ok && len(rawEnv) > 0 {
		setConfig = true
		config.EnvSecrets = make(map[string]string)
		for key, value := range rawEnv {
			if stringValue, ok := value.(string); ok {
				config.EnvSecrets[key] = stringValue
			} else {
				return false, fmt.Errorf("error casting envSecrets value to string")
			}
		}
	} else if err != nil {
		return false, err
	}
	grafanaValues.RemoveField("envSecrets")
	if setConfig {
		setInstall = true
		install.Config = config
	}

	persistenceConfig := v2.ComponentPersistenceConfig{}
	setPersistenceConfig := false
	if enabled, ok, err := grafanaValues.GetAndRemoveBool("persist"); ok {
		persistenceConfig.Enabled = &enabled
		setPersistenceConfig = true
	} else if err != nil {
		return false, err
	}
	if stoargeClassName, ok, err := grafanaValues.GetAndRemoveString("storageClassName"); ok {
		persistenceConfig.StorageClassName = stoargeClassName
		setPersistenceConfig = true
	} else if err != nil {
		return false, err
	}
	if accessMode, ok, err := grafanaValues.GetAndRemoveString("accessMode"); ok {
		persistenceConfig.AccessMode = corev1.PersistentVolumeAccessMode(accessMode)
		setPersistenceConfig = true
	} else if err != nil {
		return false, err
	}
	if resourcesValues, ok, err := grafanaValues.GetMap("persistenceResources"); ok {
		resources := &corev1.ResourceRequirements{}
		if err := decodeAndRemoveFromValues(resourcesValues, resources); err != nil {
			return false, err
		}
		persistenceConfig.Resources = resources
		setPersistenceConfig = true
		grafanaValues.RemoveField("persistenceResources")
	} else if err != nil {
		return false, err
	}
	if setPersistenceConfig {
		install.Persistence = &persistenceConfig
		setInstall = true
	}

	service := &v2.ComponentServiceConfig{}
	if applied, err := populateComponentServiceConfig(grafanaValues, service); err != nil {
		return false, err
	} else if applied {
		setInstall = true
		install.Service = service
	}

	securityConfig := v2.GrafanaSecurityConfig{}
	setSecurityConfig := false
	if enabled, ok, err := grafanaValues.GetAndRemoveBool("security.enabled"); ok {
		securityConfig.Enabled = &enabled
		setSecurityConfig = true
	} else if err != nil {
		return false, err
	}
	if secretName, ok, err := grafanaValues.GetAndRemoveString("security.secretName"); ok {
		securityConfig.SecretName = secretName
		setSecurityConfig = true
	} else if err != nil {
		return false, err
	}
	if usernameKey, ok, err := grafanaValues.GetAndRemoveString("security.usernameKey"); ok {
		securityConfig.UsernameKey = usernameKey
		setSecurityConfig = true
	} else if err != nil {
		return false, err
	}
	if passphraseKey, ok, err := grafanaValues.GetAndRemoveString("security.passphraseKey"); ok {
		securityConfig.PassphraseKey = passphraseKey
		setSecurityConfig = true
	} else if err != nil {
		return false, err
	}
	if setSecurityConfig {
		install.Security = &securityConfig
		setInstall = true
	}

	if setInstall {
		grafana.Install = install
	}
	// update the grafana settings
	if len(grafanaValues.GetContent()) == 0 {
		in.RemoveField("grafana")
	} else if err := in.SetField("grafana", grafanaValues.GetContent()); err != nil {
		return false, err
	}

	return true, nil
}
