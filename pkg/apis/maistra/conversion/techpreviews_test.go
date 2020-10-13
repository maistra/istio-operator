package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var techPreviewTestCases = []conversionTestCase{
	{
		name: "wasm-extensions." + versions.V2_0.String(),
		spec: &v2.ControlPlaneSpec{
			Version: versions.V2_0.String(),
			TechPreview: v1.NewHelmValues(map[string]interface{}{
				"wasmExtensions": map[string]interface{}{
					"enabled": true,
				},
			}),
		},
		isolatedIstio: v1.NewHelmValues(map[string]interface{}{
			"wasmExtensions": map[string]interface{}{
				"enabled": true,
			},
		}),
		completeIstio: v1.NewHelmValues(map[string]interface{}{
			"global": map[string]interface{}{
				"multiCluster":  globalMultiClusterDefaults,
				"meshExpansion": globalMeshExpansionDefaults,
			},
		}),
	},
}
