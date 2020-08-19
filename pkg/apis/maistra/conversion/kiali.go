package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateKialiAddonValues(kiali *v2.KialiAddonConfig, values map[string]interface{}) (reterr error) {
	if kiali == nil {
		return nil
	}

	kialiValues := make(map[string]interface{})
	if kiali.Name != "" {
		if err := setHelmStringValue(kialiValues, "resourceName", kiali.Name); err != nil {
			return err
		}
	}
	if kiali.Enabled != nil {
		if err := setHelmBoolValue(kialiValues, "enabled", *kiali.Enabled); err != nil {
			return err
		}
	}
	defer func() {
		if reterr == nil {
			if len(kialiValues) > 0 {
				if err := setHelmValue(values, "kiali", kialiValues); err != nil {
					reterr = err
				}
			}
		}
	}()

	if kiali.Install == nil {
		return nil
	}

	dashboardConfig := kiali.Install.Dashboard
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
	if err := populateComponentServiceValues(&kiali.Install.Service, kialiValues); err != nil {
		return err
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

	// remove values not configured through kiali addon
	kialiValues := v1.NewHelmValues(rawKialiValues).DeepCopy()
	delete(kialiValues.GetContent(), "prometheusAddr")
	if dashboardValues, ok, err := kialiValues.GetMap("dashboard"); ok {
		delete(dashboardValues, "grafanaURL")
		if len(dashboardValues) == 0 {
			delete(kialiValues.GetContent(), "dashboard")
		}
	} else if err != nil {
		return nil
	}
	if len(kialiValues.GetContent()) == 0 {
		return nil
	}

	kiali := &v2.KialiAddonConfig{}

	if name, ok, err := kialiValues.GetString("resourceName"); ok {
		kiali.Name = name
	} else if err != nil {
		return err
	}

	if enabled, ok, err := kialiValues.GetBool("enabled"); ok {
		kiali.Enabled = &enabled
	} else if err != nil {
		return err
	}

	install := &v2.KialiInstallConfig{}
	setInstall := false
	dashboardConfig := &install.Dashboard

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

	if setInstall {
		kiali.Install = install
	}

	out.Visualization.Kiali = kiali

	return nil
}
