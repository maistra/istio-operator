package conversion

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateDiscoverySelectorsValues(in *v2.ControlPlaneSpec, out map[string]interface{}) error {
	if in.MeshConfig == nil || in.MeshConfig.DiscoverySelectors == nil {
		return nil
	}

	untypedSlice := make([]interface{}, len(in.MeshConfig.DiscoverySelectors))
	for index, value := range in.MeshConfig.DiscoverySelectors {
		untypedSlice[index] = value
	}
	if discoverySelectors, err := sliceToValues(untypedSlice); err == nil {
		return setHelmValue(out, "meshConfig.discoverySelectors", discoverySelectors)
	} else {
		return err

	}

}

func populateDiscoverySelectorsConfig(in *v1.HelmValues, out *v2.ControlPlaneSpec) error {
	var discoverySelectors []*metav1.LabelSelector
	if ds, ok, err := in.GetAndRemoveSlice("meshConfig.discoverySelectors"); ok {
		if err := fromValues(ds, &discoverySelectors); err != nil {
			return err
		}
		if out.MeshConfig == nil {
			out.MeshConfig = &v2.MeshConfig{}
		}
		out.MeshConfig.DiscoverySelectors = discoverySelectors
	} else if err != nil {
		return err
	}

}
