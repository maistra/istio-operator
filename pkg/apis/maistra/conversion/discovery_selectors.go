package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func populateDiscoverySelectorsValues(in *v2.ControlPlaneSpec, out map[string]interface{}) error {
	if in.MeshConfig == nil || in.MeshConfig.DiscoverySelectors == nil {
		return nil
	}

	if len(in.MeshConfig.DiscoverySelectors) > 0 {
		untypedSlice := make([]interface{}, len(in.MeshConfig.DiscoverySelectors))
		for index, value := range in.MeshConfig.DiscoverySelectors {
			untypedSlice[index] = value
		}
		if discoverySelectors, err := sliceToValues(untypedSlice); err == nil {
			if err := setHelmValue(out, "meshConfig.discoverySelectors", discoverySelectors); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func populateDiscoverySelectorsConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	setDiscoverySelectors := false

	discoverySelectors := []*metav1.LabelSelector{}
	if ds, ok, err := in.GetFieldNoCopy("meshConfig.discoverySelectors"); ok {
		if err := fromValues(ds, &discoverySelectors); err != nil {
			return err
		}
		setDiscoverySelectors = true
		in.RemoveField("meshConfig.discoverySelectors")
	} else if err != nil {
		return err
	}

	if setDiscoverySelectors {
		if out.MeshConfig == nil {
			out.MeshConfig = &v2.MeshConfig{}
		}
		out.MeshConfig.DiscoverySelectors = discoverySelectors
	}
	return nil
}
