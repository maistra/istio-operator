package conversion

import (
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var techPreviewTestCases []conversionTestCase

func techPreviewTestCasesV2(version versions.Version) []conversionTestCase{
	ver := version.String()
	return []conversionTestCase{
		{
			name: "wasm-extensions." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
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
}

func init() {
	for _, v := range versions.AllV2Versions {
		techPreviewTestCases = append(techPreviewTestCases, techPreviewTestCasesV2(v)...)
	}
}
