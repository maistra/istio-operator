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
				if err := setHelmValue(values, "grafana", grafanaValues); err != nil {
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
	if err := populateComponentServiceValues(&grafana.Install.Service, grafanaValues); err != nil {
		return err
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

	runtime := grafana.Install.Runtime
	if runtime != nil {
		if err := populateRuntimeValues(runtime, grafanaValues); err != nil {
			return err
		}

		// set image and resources
		if runtime.Pod.Containers != nil {
			if container, ok := runtime.Pod.Containers["grafana"]; ok {
				if err := populateContainerConfigValues(&container, grafanaValues); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func populateGrafanaAddonConfig(in *v1.HelmValues, out *v2.AddonsConfig) error {
	rawGrafanaValues, ok, err := in.GetMap("grafana")
	if err != nil {
		return err
	} else if !ok || len(rawGrafanaValues) == 0 {
		// nothing to do
		// check to see if grafana.Address should be set
		if address, ok, err := in.GetString("kiali.dashboard.grafanaURL"); ok {
			// If grafana URL is set, assume we're using an existing grafana install
			out.Visualization.Grafana = &v2.GrafanaAddonConfig{
				Address: &address,
			}
		} else if err != nil {
			return err
		}
		return nil
	}

	if out.Visualization.Grafana == nil {
		out.Visualization.Grafana = &v2.GrafanaAddonConfig{}
	}
	grafana := out.Visualization.Grafana
	grafanaValues := v1.NewHelmValues(rawGrafanaValues)
	if enabled, ok, err := grafanaValues.GetBool("enabled"); ok {
		grafana.Enabled = &enabled
	} else if err != nil {
		return err
	}

	if address, ok, err := in.GetString("kiali.dashboard.grafanaURL"); ok {
		// If grafana URL is set, assume we're using an existing grafana install
		grafana.Address = &address
		grafana.Install = nil
		return nil
	} else if err != nil {
		return err
	}

	install := &v2.GrafanaInstallConfig{}
	setInstall := false

	if rawEnv, ok, err := grafanaValues.GetMap("env"); ok {
		setInstall = true
		install.Config.Env = make(map[string]string)
		for key, value := range rawEnv {
			if stringValue, ok := value.(string); ok {
				install.Config.Env[key] = stringValue
			} else {
				return fmt.Errorf("error casting env value to string")
			}
		}
	} else if err != nil {
		return err
	}
	if rawEnv, ok, err := grafanaValues.GetMap("envSecrets"); ok {
		setInstall = true
		install.Config.EnvSecrets = make(map[string]string)
		for key, value := range rawEnv {
			if stringValue, ok := value.(string); ok {
				install.Config.EnvSecrets[key] = stringValue
			} else {
				return fmt.Errorf("error casting envSecrets value to string")
			}
		}
	} else if err != nil {
		return err
	}

	persistenceConfig := v2.ComponentPersistenceConfig{}
	setPersistenceConfig := false
	if enabled, ok, err := grafanaValues.GetBool("persist"); ok {
		persistenceConfig.Enabled = &enabled
		setPersistenceConfig = true
	} else if err != nil {
		return err
	}
	if stoargeClassName, ok, err := grafanaValues.GetString("storageClassName"); ok {
		persistenceConfig.StorageClassName = stoargeClassName
		setPersistenceConfig = true
	} else if err != nil {
		return err
	}
	if accessMode, ok, err := grafanaValues.GetString("accessMode"); ok {
		persistenceConfig.AccessMode = corev1.PersistentVolumeAccessMode(accessMode)
		setPersistenceConfig = true
	} else if err != nil {
		return err
	}
	if resourcesValues, ok, err := grafanaValues.GetMap("persistenceResources"); ok {
		resources := &corev1.ResourceRequirements{}
		if err := fromValues(resourcesValues, resources); err != nil {
			return err
		}
		persistenceConfig.Resources = resources
		setPersistenceConfig = true
	} else if err != nil {
		return err
	}
	if setPersistenceConfig {
		install.Persistence = &persistenceConfig
		setInstall = true
	}

	if applied, err := populateComponentServiceConfig(grafanaValues, &install.Service); err != nil {
		return err
	} else if applied {
		setInstall = true
	}

	securityConfig := v2.GrafanaSecurityConfig{}
	setSecurityConfig := false
	if enabled, ok, err := grafanaValues.GetBool("security.enabled"); ok {
		securityConfig.Enabled = &enabled
		setSecurityConfig = true
	} else if err != nil {
		return err
	}
	if secretName, ok, err := grafanaValues.GetString("security.secretName"); ok {
		securityConfig.SecretName = secretName
		setSecurityConfig = true
	} else if err != nil {
		return err
	}
	if usernameKey, ok, err := grafanaValues.GetString("security.usernameKey"); ok {
		securityConfig.UsernameKey = usernameKey
		setSecurityConfig = true
	} else if err != nil {
		return err
	}
	if passphraseKey, ok, err := grafanaValues.GetString("security.passphraseKey"); ok {
		securityConfig.PassphraseKey = passphraseKey
		setSecurityConfig = true
	} else if err != nil {
		return err
	}
	if setSecurityConfig {
		install.Security = &securityConfig
		setInstall = true
	}

	runtime := &v2.ComponentRuntimeConfig{}
	if applied, err := runtimeValuesToComponentRuntimeConfig(grafanaValues, runtime); err != nil {
		return err
	} else if applied {
		install.Runtime = runtime
		setInstall = true
	}
	container := v2.ContainerConfig{}
	if applied, err := populateContainerConfig(grafanaValues, &container); err != nil {
		return err
	} else if applied {
		if install.Runtime == nil {
			install.Runtime = runtime
		}
		install.Runtime.Pod.Containers = map[string]v2.ContainerConfig{
			"grafana": container,
		}
		setInstall = true
	}

	if setInstall {
		grafana.Install = install
	}

	return nil
}
