package bootstrap

import (
	"context"
	"os"
	"path"
	goruntime "runtime"
	"testing"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery/fake"

	"github.com/maistra/istio-operator/controllers/common"
	"github.com/maistra/istio-operator/controllers/common/cni"
	"github.com/maistra/istio-operator/controllers/common/test"
	"github.com/maistra/istio-operator/controllers/common/test/assert"
	"github.com/maistra/istio-operator/controllers/versions"
)

func TestCNISupportedVersionRendering(t *testing.T) {
	operatorNamespace := "istio-operator"
	InitializeGlobals(operatorNamespace)()

	ctx := context.Background()

	testCases := []struct {
		name              string
		supportedVersions []versions.Version
		instanceVersion   versions.Version
		containerNames    []string
		daemonsetName     string
	}{
		{
			name:              "Default Supported Versions SMCP v2.2",
			supportedVersions: versions.GetSupportedVersions(),
			instanceVersion:   versions.V2_2.Version(),
			containerNames:    []string{"install-cni-v2-0", "install-cni-v2-1", "install-cni-v2-2"},
			daemonsetName:     "istio-cni-node",
		},
		{
			name:              "Default Supported Versions SMCP v2.3",
			supportedVersions: versions.GetSupportedVersions(),
			instanceVersion:   versions.V2_3.Version(),
			containerNames:    []string{"install-cni"},
			daemonsetName:     "istio-cni-node-v2-3",
		},
		{
			name:              "v2.0 only",
			supportedVersions: []versions.Version{versions.V2_0},
			instanceVersion:   versions.V2_0.Version(),
			containerNames:    []string{"install-cni-v2-0"},
			daemonsetName:     "istio-cni-node",
		},
		{
			name:              "v2.1 only",
			supportedVersions: []versions.Version{versions.V2_1},
			instanceVersion:   versions.V2_1.Version(),
			containerNames:    []string{"install-cni-v2-1"},
			daemonsetName:     "istio-cni-node",
		},
		{
			name:              "v2.2 only",
			supportedVersions: []versions.Version{versions.V2_2},
			instanceVersion:   versions.V2_2.Version(),
			containerNames:    []string{"install-cni-v2-2"},
			daemonsetName:     "istio-cni-node",
		},
		{
			name:              "v2.3 only",
			supportedVersions: []versions.Version{versions.V2_3},
			instanceVersion:   versions.V2_3.Version(),
			containerNames:    []string{"install-cni"},
			daemonsetName:     "istio-cni-node-v2-3",
		},
	}

	config := cni.Config{
		Enabled:   true,
		UseMultus: true,
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cl, tracker := test.CreateClient()
			dc := fake.FakeDiscovery{Fake: &tracker.Fake, FakedServerVersion: test.DefaultKubeVersion}
			renderings, err := internalRenderCNI(ctx, cl, config, &dc, tc.supportedVersions, tc.instanceVersion)
			assert.Success(err, "internalRenderCNI", t)
			assert.True(renderings != nil, "renderings should not be nil", t)
			cniManifests := renderings["istio_cni"]
			assert.True(cniManifests != nil, "CNI manifests are not present", t)

			var foundDaemonSet bool

			for _, manifest := range cniManifests {
				if manifest.Head.Kind == "DaemonSet" {
					foundDaemonSet = true
					json, err := yaml.YAMLToJSON([]byte(manifest.Content))
					assert.Success(err, "YAMLToJSON", t)
					resource := &unstructured.Unstructured{}
					_, _, err = unstructured.UnstructuredJSONScheme.Decode(json, nil, resource)
					assert.Success(err, "resource decoding", t)

					dsName, found, err := unstructured.NestedString(resource.UnstructuredContent(), "metadata", "name")
					assert.Success(err, "unstructured.NestedString", t)
					assert.True(found, "Could not find metadata name", t)
					assert.DeepEquals(dsName, tc.daemonsetName, "Unexpected daemonset name found", t)

					containers, found, err := unstructured.NestedSlice(resource.UnstructuredContent(), "spec", "template", "spec", "containers")
					assert.Success(err, "unstructured.NestedSlice", t)
					assert.True(found, "Could not find containers", t)
					assert.True(len(containers) > 0, "No containers in resource", t)
					names := []string{}
					for _, container := range containers {
						val, ok := container.(map[string]interface{})
						assert.True(ok, "Converting container to map", t)
						names = append(names, val["name"].(string))
					}
					assert.DeepEquals(names, tc.containerNames, "Unexpected container name set", t)
				}
			}
			assert.True(foundDaemonSet, "Daemon Set was not in Manifest list", t)
		})
	}
}

// InitializeGlobals returns a function which initializes global variables used
// by the system under test.  operatorNamespace is the namespace within which
// the operator is installed.
func InitializeGlobals(operatorNamespace string) func() {
	return func() {
		// make sure globals are initialized for testing
		common.Config.OLM.Images.V1_1.CNI = "istio-cni-test-1_1"
		common.Config.OLM.Images.V2_0.CNI = "istio-cni-test-2_0"
		common.Config.OLM.Images.V2_1.CNI = "istio-cni-test-2_1"
		common.Config.OLM.Images.V1_1.ThreeScale = "injected-3scale-v1.1"
		common.Config.OLM.Images.V2_0.ThreeScale = "injected-3scale-v2.0"
		common.Config.OLM.Images.V2_1.ThreeScale = "injected-3scale-v2.1"
		common.Config.OLM.Images.V1_1.Citadel = "injected-citadel-v1.1"
		common.Config.OLM.Images.V1_1.Galley = "injected-galley-v1.1"
		common.Config.OLM.Images.V1_1.Grafana = "injected-grafana-v1.1"
		common.Config.OLM.Images.V2_0.Grafana = "injected-grafana-v2.0"
		common.Config.OLM.Images.V2_1.Grafana = "injected-grafana-v2.1"
		common.Config.OLM.Images.V1_1.Mixer = "injected-mixer-v1.1"
		common.Config.OLM.Images.V2_0.Mixer = "injected-mixer-v2.0"
		common.Config.OLM.Images.V1_1.Pilot = "injected-pilot-v1.1"
		common.Config.OLM.Images.V2_0.Pilot = "injected-pilot-v2.0"
		common.Config.OLM.Images.V2_1.Pilot = "injected-pilot-v2.1"
		common.Config.OLM.Images.V1_1.Prometheus = "injected-prometheus-v1.1"
		common.Config.OLM.Images.V2_0.Prometheus = "injected-prometheus-v2.0"
		common.Config.OLM.Images.V2_1.Prometheus = "injected-prometheus-v2.1"
		common.Config.OLM.Images.V1_1.ProxyInit = "injected-proxy-init-v1.1"
		common.Config.OLM.Images.V2_0.ProxyInit = "injected-proxy-init-v2.0"
		common.Config.OLM.Images.V2_1.ProxyInit = "injected-proxy-init-v2.1"
		common.Config.OLM.Images.V1_1.ProxyV2 = "injected-proxyv2-v1.1"
		common.Config.OLM.Images.V2_0.ProxyV2 = "injected-proxyv2-v2.0"
		common.Config.OLM.Images.V2_1.ProxyV2 = "injected-proxyv2-v2.1"
		common.Config.OLM.Images.V1_1.SidecarInjector = "injected-sidecar-injector-v1.1"
		common.Config.OLM.Images.V1_1.IOR = "injected-ior-v1.1"
		common.Config.OLM.Images.V2_0.WASMCacher = "injected-wasm-cacher-v2.0"
		common.Config.OLM.Images.V2_1.WASMCacher = "injected-wasm-cacher-v2.1"
		os.Setenv("POD_NAMESPACE", operatorNamespace)
		common.GetOperatorNamespace()
		if _, filename, _, ok := goruntime.Caller(0); ok {
			common.Config.Rendering.ResourceDir = path.Join(path.Dir(filename), "../../resources")
			common.Config.Rendering.ChartsDir = path.Join(common.Config.Rendering.ResourceDir, "helm")
			common.Config.Rendering.DefaultTemplatesDir = path.Join(common.Config.Rendering.ResourceDir, "smcp-templates")
		} else {
			panic("could not initialize common.ResourceDir")
		}
	}
}
