package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateGeneralValues(general *v2.GeneralConfig, values map[string]interface{}) error {
	if general == nil {
		return nil
	}
	if err := populateControlPlaneLogging(general.Logging, values); err != nil {
		return err
	}

	if general.ValidationMessages != nil {
		if err := setHelmBoolValue(values, "galley.enableAnalysis", *general.ValidationMessages); err != nil {
			return err
		}
		if err := setHelmBoolValue(values, "global.istiod.enableAnalysis", *general.ValidationMessages); err != nil {
			return err
		}
	}
	return nil
}

func populateGeneralConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	if err := populateControlPlaneLoggingConfig(in, out); err != nil {
		return err
	}
	if enableAnalysis, ok, err := in.GetBool("global.istiod.enableAnalysis"); ok {
		if out.General == nil {
			out.General = &v2.GeneralConfig{}
		}
		out.General.ValidationMessages = &enableAnalysis
	} else if err != nil {
		return err
	} else if enableAnalysis, ok, err := in.GetBool("galley.enableAnalysis"); ok {
		if out.General == nil {
			out.General = &v2.GeneralConfig{}
		}
		out.General.ValidationMessages = &enableAnalysis
	} else if err != nil {
		return err
	}
	return nil
}
