package conversion

import (
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateTechPreviewsValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	if in.TechPreviews == nil {
		return nil
	}
	if in.TechPreviews.WasmExtensions != nil {
		if in.TechPreviews.WasmExtensions.Enabled != nil {
			if err := setHelmBoolValue(values, "mec.enabled", *in.TechPreviews.WasmExtensions.Enabled); err != nil {
				return err
			}
		}
	}
	return nil
}
