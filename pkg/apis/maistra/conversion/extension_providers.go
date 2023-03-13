package conversion

import (
	"fmt"
	"github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateExtensionProvidersValues(in *v2.ControlPlaneSpec, values map[string]interface{}) error {
	if in.ExtensionProviders == nil || len(in.ExtensionProviders) == 0 {
		return nil
	}

	for _, ext := range in.ExtensionProviders {
		if ext.Prometheus == nil {
			return fmt.Errorf("extension provider entry %s does not define any provider - it must specify one of: prometheus", ext.Name)
		}
		prometheus := []map[string]interface{}{
			{
				"name":       ext.Name,
				"prometheus": map[string]interface{}{},
			},
		}
		if err := setHelmMapSliceValue(values, "meshConfig.extensionProviders", prometheus); err != nil {
			return err
		}
		//if err := setHelmStringValue(values, fmt.Sprintf("meshConfig.extensionProviders[%d].name", i), ext.Name); err != nil {
		//	return err
		//}
		//if err := setHelmValue(values, fmt.Sprintf("meshConfig.extensionProviders[%d].prometheus", i), ext.Prometheus); err != nil {
		//	return err
		//}
	}
	return nil
}

func populateExtensionProvidersConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	//rawMeshConfigValues, ok, err := in.GetMap("meshConfig")
	//if err != nil {
	//	return err
	//}
	//if !ok || len(rawMeshConfigValues) == 0 {
	//	return nil
	//}
	//
	//meshConfigValues := v1.NewHelmValues(rawMeshConfigValues)
	if _, ok, err := in.GetAndRemoveSlice("meshConfig.extensionProviders"); ok {
		//for _, rawExtProvider := range rawExtProviders {
		//	if extProvider, ok := rawExtProvider.(*v2.ExtensionProviderConfig); ok {
		//		out.ExtensionProviders = append(out.ExtensionProviders, extProvider)
		//	}
		//}
	} else if err != nil {
		return err
	}

	return nil
}
