package conversion

import (
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateKialiAddonValues(kiali *v2.KialiAddonConfig, values map[string]interface{}) error {
	if kiali == nil {
		return setHelmValue(values, "kiali.enabled", false)
	}
	if kiali.Install == nil {
		if err := setHelmValue(values, "kiali.enabled", false); err != nil {
			return err
		}
		return nil
	}

	kialiValues := make(map[string]interface{})
	if err := setHelmValue(kialiValues, "enabled", true); err != nil {
		return err
	}

	dashboardConfig := kiali.Install.Config.Dashboard
	if dashboardConfig.ViewOnly != nil {
		if err := setHelmValue(kialiValues, "dashboard.viewOnlyMode", *dashboardConfig.ViewOnly); err != nil {
			return err
		}
	}
	if dashboardConfig.EnableGrafana != nil {
		if err := setHelmValue(kialiValues, "dashboard.enableGrafana", *dashboardConfig.EnableGrafana); err != nil {
			return err
		}
	}
	if dashboardConfig.EnablePrometheus != nil {
		if err := setHelmValue(kialiValues, "dashboard.enablePrometheus", *dashboardConfig.EnablePrometheus); err != nil {
			return err
		}
	}
	if dashboardConfig.EnableTracing != nil {
		if err := setHelmValue(kialiValues, "dashboard.enableTracing", *dashboardConfig.EnableTracing); err != nil {
			return err
		}
	}
	if kiali.Install.Service.Ingress != nil {
		ingressValues := make(map[string]interface{})
		if err := populateAddonIngressValues(kiali.Install.Service.Ingress, ingressValues); err == nil {
			if err := setHelmValue(kialiValues, "ingress", ingressValues); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	if err := populateRuntimeValues(kiali.Install.Runtime, kialiValues); err != nil {
		return err
	}

    if kialiContainer, ok := kiali.Install.Runtime.Pod.Containers["kiali"]; ok {
        if kialiContainer.Image != "" {
            if err := setHelmValue(kialiValues, "image", kialiContainer.Image); err != nil {
                return err
            }
        }
        if kialiContainer.ImageRegistry != "" {
            if err := setHelmValue(kialiValues, "hub", kialiContainer.ImageRegistry); err != nil {
                return err
            }
        }
        if kialiContainer.ImageTag != "" {
            if err := setHelmValue(kialiValues, "tag", kialiContainer.ImageTag); err != nil {
                return err
            }
        }
        if kialiContainer.ImagePullPolicy != "" {
            if err := setHelmValue(kialiValues, "imagePullPolicy", kialiContainer.ImagePullPolicy); err != nil {
                return err
            }
        }
    }

    if err := setHelmValue(values, "kiali", kialiValues); err != nil {
		return err
	}
	return nil
}
