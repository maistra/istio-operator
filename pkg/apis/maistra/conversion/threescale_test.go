package conversion

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	threeScaleTestListenAddr               = int32(3333)
	threeScaleTestMetricsPort              = int32(8080)
	threeScaleTestCacheMaxSize             = int64(1000)
	threeScaleTestCacheRefreshRetries      = int32(1)
	threeScaleTestCacheRefreshInterval     = int32(180)
	threeScaleTestCacheTTL                 = int32(300)
	threeScaleTestClientTimeout            = int32(10)
	threeScaleTestGRPCMaxConnTimeout       = int32(60)
	threeScaleTestBackendCachFlushInterval = int32(15)
)

var threeScaleTestCases []conversionTestCase

func threeScaleTestCasesV2(version versions.Version) []conversionTestCase{
	ver := version.String()
	return []conversionTestCase{
		{
			name: "nil." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "full." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Addons: &v2.AddonsConfig{
					ThreeScale: &v2.ThreeScaleAddonConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
						ListenAddr: &threeScaleTestListenAddr,
						LogGRPC:    &featureDisabled,
						LogJSON:    &featureEnabled,
						LogLevel:   "info",
						Metrics: &v2.ThreeScaleMetricsConfig{
							Port:   &threeScaleTestMetricsPort,
							Report: &featureEnabled,
						},
						System: &v2.ThreeScaleSystemConfig{
							CacheMaxSize:         &threeScaleTestCacheMaxSize,
							CacheRefreshRetries:  &threeScaleTestCacheRefreshRetries,
							CacheRefreshInterval: &threeScaleTestCacheRefreshInterval,
							CacheTTL:             &threeScaleTestCacheTTL,
						},
						Client: &v2.ThreeScaleClientConfig{
							AllowInsecureConnections: &featureDisabled,
							Timeout:                  &threeScaleTestClientTimeout,
						},
						GRPC: &v2.ThreeScaleGRPCConfig{
							MaxConnTimeout: &threeScaleTestGRPCMaxConnTimeout,
						},
						Backend: &v2.ThreeScaleBackendConfig{
							EnableCache:        &featureDisabled,
							CacheFlushInterval: &threeScaleTestBackendCachFlushInterval,
							PolicyFailClosed:   &featureEnabled,
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
			isolatedThreeScale: v1.NewHelmValues(map[string]interface{}{
				"enabled":                                               true,
				"PARAM_THREESCALE_LISTEN_ADDR":                          3333,
				"PARAM_THREESCALE_LOG_LEVEL":                            "info",
				"PARAM_THREESCALE_LOG_JSON":                             true,
				"PARAM_THREESCALE_LOG_GRPC":                             false,
				"PARAM_THREESCALE_REPORT_METRICS":                       true,
				"PARAM_THREESCALE_METRICS_PORT":                         8080,
				"PARAM_THREESCALE_CACHE_TTL_SECONDS":                    300,
				"PARAM_THREESCALE_CACHE_REFRESH_SECONDS":                180,
				"PARAM_THREESCALE_CACHE_ENTRIES_MAX":                    1000,
				"PARAM_THREESCALE_CACHE_REFRESH_RETRIES":                1,
				"PARAM_THREESCALE_ALLOW_INSECURE_CONN":                  false,
				"PARAM_THREESCALE_CLIENT_TIMEOUT_SECONDS":               10,
				"PARAM_THREESCALE_GRPC_CONN_MAX_SECONDS":                60,
				"PARAM_THREESCALE_BACKEND_CACHE_FLUSH_INTERVAL_SECONDS": 15,
				"PARAM_THREESCALE_BACKEND_CACHE_POLICY_FAIL_CLOSED":     true,
				"PARAM_THREESCALE_USE_CACHED_BACKEND":                   false,
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
		},
		{
			name: "runtime." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				Runtime: &v2.ControlPlaneRuntimeConfig{
					Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
						"3scale": {
							Container: &v2.ContainerConfig{
								CommonContainerConfig: v2.CommonContainerConfig{
									ImageRegistry:   "custom-registry",
									ImageTag:        "test",
									ImagePullPolicy: "Always",
									ImagePullSecrets: []corev1.LocalObjectReference{
										{
											Name: "pull-secret",
										},
									},
									Resources: &corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("100m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("10m"),
											corev1.ResourceMemory: resource.MustParse("64Mi"),
										},
									},
								},
								Image: "custom-3scale",
							},
						},
					},
				},
				Addons: &v2.AddonsConfig{
					ThreeScale: &v2.ThreeScaleAddonConfig{
						Enablement: v2.Enablement{
							Enabled: &featureEnabled,
						},
						ListenAddr: &threeScaleTestListenAddr,
						LogGRPC:    &featureDisabled,
						LogJSON:    &featureEnabled,
						LogLevel:   "info",
						Metrics: &v2.ThreeScaleMetricsConfig{
							Port:   &threeScaleTestMetricsPort,
							Report: &featureEnabled,
						},
						System: &v2.ThreeScaleSystemConfig{
							CacheMaxSize:         &threeScaleTestCacheMaxSize,
							CacheRefreshRetries:  &threeScaleTestCacheRefreshRetries,
							CacheRefreshInterval: &threeScaleTestCacheRefreshInterval,
							CacheTTL:             &threeScaleTestCacheTTL,
						},
						Client: &v2.ThreeScaleClientConfig{
							AllowInsecureConnections: &featureDisabled,
							Timeout:                  &threeScaleTestClientTimeout,
						},
						GRPC: &v2.ThreeScaleGRPCConfig{
							MaxConnTimeout: &threeScaleTestGRPCMaxConnTimeout,
						},
						Backend: &v2.ThreeScaleBackendConfig{
							EnableCache:        &featureDisabled,
							CacheFlushInterval: &threeScaleTestBackendCachFlushInterval,
							PolicyFailClosed:   &featureEnabled,
						},
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{}),
			isolatedThreeScale: v1.NewHelmValues(map[string]interface{}{
				"enabled":                                               true,
				"PARAM_THREESCALE_LISTEN_ADDR":                          3333,
				"PARAM_THREESCALE_LOG_LEVEL":                            "info",
				"PARAM_THREESCALE_LOG_JSON":                             true,
				"PARAM_THREESCALE_LOG_GRPC":                             false,
				"PARAM_THREESCALE_REPORT_METRICS":                       true,
				"PARAM_THREESCALE_METRICS_PORT":                         8080,
				"PARAM_THREESCALE_CACHE_TTL_SECONDS":                    300,
				"PARAM_THREESCALE_CACHE_REFRESH_SECONDS":                180,
				"PARAM_THREESCALE_CACHE_ENTRIES_MAX":                    1000,
				"PARAM_THREESCALE_CACHE_REFRESH_RETRIES":                1,
				"PARAM_THREESCALE_ALLOW_INSECURE_CONN":                  false,
				"PARAM_THREESCALE_CLIENT_TIMEOUT_SECONDS":               10,
				"PARAM_THREESCALE_GRPC_CONN_MAX_SECONDS":                60,
				"PARAM_THREESCALE_BACKEND_CACHE_FLUSH_INTERVAL_SECONDS": 15,
				"PARAM_THREESCALE_BACKEND_CACHE_POLICY_FAIL_CLOSED":     true,
				"PARAM_THREESCALE_USE_CACHED_BACKEND":                   false,
			}),
			completeIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"multiCluster":  globalMultiClusterDefaults,
					"meshExpansion": globalMeshExpansionDefaults,
				},
			}),
			completeThreeScale: v1.NewHelmValues(map[string]interface{}{
				"hub":             "custom-registry",
				"tag":             "test",
				"image":           "custom-3scale",
				"imagePullPolicy": "Always",
				"imagePullSecrets": []interface{}{
					"pull-secret",
				},
				"resources": map[string]interface{}{
					"limits": map[string]interface{}{
						"cpu":    "100m",
						"memory": "128Mi",
					},
					"requests": map[string]interface{}{
						"cpu":    "10m",
						"memory": "64Mi",
					},
				},
			}),
		},
	}
}

func init() {
	for _, v := range versions.AllV2Versions {
		threeScaleTestCases = append(threeScaleTestCases, threeScaleTestCasesV2(v)...)
	}
}

func TestThreeScaleConversionFromV2(t *testing.T) {
	for _, tc := range threeScaleTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateAddonsValues(specCopy, helmValues.GetContent()); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			expected := tc.isolatedIstio.DeepCopy()
			if tc.isolatedThreeScale != nil {
				expected.SetField("3scale", tc.isolatedThreeScale.DeepCopy().GetContent())
			}
			if !reflect.DeepEqual(expected.DeepCopy(), helmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", expected.GetContent(), helmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), helmValues.GetContent())
			if tc.isolatedThreeScale != nil {
				helmValues.SetField("3scale", tc.isolatedThreeScale.DeepCopy().GetContent())
			}
			if err := populateAddonsConfig(helmValues.DeepCopy(), specv2); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			assertEquals(t, tc.spec.Addons, specv2.Addons)
		})
	}
}
