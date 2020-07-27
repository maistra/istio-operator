package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateKialiAddonValues(kiali *v2.KialiAddonConfig, values map[string]interface{}) error {
	if kiali == nil {
		return nil
	}
	if err := setHelmStringValue(values, "kiali.resourceName", kiali.Name); err != nil {
		return err
	}

	if kiali.Install == nil {
		// we don't want to process the charts
		if err := setHelmBoolValue(values, "kiali.enabled", false); err != nil {
			return err
		}
		return nil
	}

	kialiValues := make(map[string]interface{})
	if kiali.Enabled != nil {
		if err := setHelmBoolValue(kialiValues, "enabled", *kiali.Enabled); err != nil {
			return err
		}
	}

	dashboardConfig := kiali.Install.Config.Dashboard
	if dashboardConfig.ViewOnly != nil {
		if err := setHelmBoolValue(kialiValues, "dashboard.viewOnlyMode", *dashboardConfig.ViewOnly); err != nil {
			return err
		}
	}
	if dashboardConfig.EnableGrafana != nil {
		if err := setHelmBoolValue(kialiValues, "dashboard.enableGrafana", *dashboardConfig.EnableGrafana); err != nil {
			return err
		}
	}
	if dashboardConfig.EnablePrometheus != nil {
		if err := setHelmBoolValue(kialiValues, "dashboard.enablePrometheus", *dashboardConfig.EnablePrometheus); err != nil {
			return err
		}
	}
	if dashboardConfig.EnableTracing != nil {
		if err := setHelmBoolValue(kialiValues, "dashboard.enableTracing", *dashboardConfig.EnableTracing); err != nil {
			return err
		}
	}
	ingressValues := make(map[string]interface{})
	if err := populateAddonIngressValues(kiali.Install.Service.Ingress, ingressValues); err == nil {
		if len(ingressValues) > 0 {
			if err := setHelmValue(kialiValues, "ingress", ingressValues); err != nil {
				return err
			}
		}
	} else {
		return err
	}

	if kiali.Install.Runtime != nil {
		runtime := kiali.Install.Runtime
		if err := populateRuntimeValues(runtime, kialiValues); err != nil {
			return err
		}

		if kialiContainer, ok := runtime.Pod.Containers["kiali"]; ok {
			if kialiContainer.Image != "" {
				if err := setHelmStringValue(kialiValues, "image", kialiContainer.Image); err != nil {
					return err
				}
			}
			if kialiContainer.ImageRegistry != "" {
				if err := setHelmStringValue(kialiValues, "hub", kialiContainer.ImageRegistry); err != nil {
					return err
				}
			}
			if kialiContainer.ImageTag != "" {
				if err := setHelmStringValue(kialiValues, "tag", kialiContainer.ImageTag); err != nil {
					return err
				}
			}
			if kialiContainer.ImagePullPolicy != "" {
				if err := setHelmStringValue(kialiValues, "imagePullPolicy", string(kialiContainer.ImagePullPolicy)); err != nil {
					return err
				}
			}
		}
	}

	if len(kialiValues) > 0 {
		if err := setHelmValue(values, "kiali", kialiValues); err != nil {
			return err
		}
	}
	return nil
}

func populateKialiAddonConfig(in *v1.HelmValues, out *v2.AddonsConfig) error {
	rawKialiValues, ok, err := in.GetMap("kiali")
	if err != nil {
		return err
	} else if !ok || len(rawKialiValues) == 0 {
		return nil
	}
	kialiValues := v1.NewHelmValues(rawKialiValues)

	kiali := &v2.KialiAddonConfig{}

	if name, ok, err := kialiValues.GetString("resourceName"); err != nil {
		return err
	} else if !ok || name == "" {
		kiali.Name = "kiali"
	} else {
		kiali.Name = name
	}

	if enabled, ok, err := kialiValues.GetBool("enabled"); ok {
		kiali.Enabled = &enabled
	} else if err != nil {
		return err
	}

	install := &v2.KialiInstallConfig{}
	// for v1, there's no way to disable install, so always create install
	setInstall := true
	dashboardConfig := &install.Config.Dashboard

	if viewOnlyMode, ok, err := kialiValues.GetBool("dashboard.viewOnlyMode"); ok {
		dashboardConfig.ViewOnly = &viewOnlyMode
		setInstall = true
	} else if err != nil {
		return err
	}
	if enableGrafana, ok, err := kialiValues.GetBool("dashboard.enableGrafana"); ok {
		dashboardConfig.EnableGrafana = &enableGrafana
		setInstall = true
	} else if err != nil {
		return err
	}
	if enablePrometheus, ok, err := kialiValues.GetBool("dashboard.enablePrometheus"); ok {
		dashboardConfig.EnablePrometheus = &enablePrometheus
		setInstall = true
	} else if err != nil {
		return err
	}
	if enableTracing, ok, err := kialiValues.GetBool("dashboard.enableTracing"); ok {
		dashboardConfig.EnableTracing = &enableTracing
		setInstall = true
	} else if err != nil {
		return err
	}

	if applied, err := populateComponentServiceConfig(kialiValues, &install.Service); err == nil {
		setInstall = setInstall || applied
	} else {
		return err
	}

	runtime := &v2.ComponentRuntimeConfig{}
	if applied, err := runtimeValuesToComponentRuntimeConfig(kialiValues, runtime); err != nil {
		return err
	} else if applied {
		install.Runtime = runtime
		setInstall = true
	}
	container := v2.ContainerConfig{}
	if applied, err := populateContainerConfig(kialiValues, &container); err != nil {
		return err
	} else if applied {
		if install.Runtime == nil {
			install.Runtime = runtime
		}
		install.Runtime.Pod.Containers = map[string]v2.ContainerConfig{
			"kiali": container,
		}
		setInstall = true
	}

	if setInstall {
		kiali.Install = install
	}

	out.Visualization.Kiali = kiali

	return nil
}
