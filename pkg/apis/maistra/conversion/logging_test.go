package conversion

import (
	"reflect"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var loggingTestCases []conversionTestCase

func loggingTestCasesV2(version versions.Version) []conversionTestCase {
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
			name: "defaults." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				General: &v2.GeneralConfig{
					Logging: &v2.LoggingConfig{},
				},
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
			name: "all." + ver,
			spec: &v2.ControlPlaneSpec{
				Version: ver,
				General: &v2.GeneralConfig{
					Logging: &v2.LoggingConfig{
						ComponentLevels: v2.ComponentLogLevels{
							v2.EnvoyComponentAdmin:  v2.LogLevelDebug,
							v2.EnvoyComponentClient: v2.LogLevelTrace,
						},
						LogAsJSON: &featureEnabled,
					},
				},
			},
			isolatedIstio: v1.NewHelmValues(map[string]interface{}{
				"global": map[string]interface{}{
					"logAsJson": true,
					"logging": map[string]interface{}{
						"level": "admin:debug,client:trace",
					},
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
		loggingTestCases = append(loggingTestCases, loggingTestCasesV2(v)...)
	}
}

func TestLoggingConversionFromV2(t *testing.T) {
	for _, tc := range loggingTestCases {
		t.Run(tc.name, func(t *testing.T) {
			specCopy := tc.spec.DeepCopy()
			var loggingConfig *v2.LoggingConfig
			if specCopy.General != nil {
				loggingConfig = specCopy.General.Logging
			}
			helmValues := v1.NewHelmValues(make(map[string]interface{}))
			if err := populateControlPlaneLogging(loggingConfig, helmValues.GetContent()); err != nil {
				t.Fatalf("error converting to values: %s", err)
			}
			if !reflect.DeepEqual(tc.isolatedIstio.DeepCopy(), helmValues.DeepCopy()) {
				t.Errorf("unexpected output converting v2 to values:\n\texpected:\n%#v\n\tgot:\n%#v", tc.isolatedIstio.GetContent(), helmValues.GetContent())
			}
			specv2 := &v2.ControlPlaneSpec{}
			// use expected values
			helmValues = tc.isolatedIstio.DeepCopy()
			mergeMaps(tc.completeIstio.DeepCopy().GetContent(), helmValues.GetContent())
			if err := populateControlPlaneLoggingConfig(helmValues.DeepCopy(), specv2); err != nil {
				t.Fatalf("error converting from values: %s", err)
			}
			assertEquals(t, tc.spec.General, specv2.General)
		})
	}
}

func TestComponentLogLevelsFromString(t *testing.T) {
	testCases := []struct {
		logLevelsString   string
		expectError       bool
		expectedLogLevels v2.ComponentLogLevels
	}{
		{
			logLevelsString:   "",
			expectedLogLevels: nil,
		},
		{
			logLevelsString: "admin:info",
			expectedLogLevels: v2.ComponentLogLevels{
				v2.EnvoyComponentAdmin: v2.LogLevelInfo,
			},
		},
		{
			logLevelsString: "admin:info,client:debug",
			expectedLogLevels: v2.ComponentLogLevels{
				v2.EnvoyComponentAdmin:  v2.LogLevelInfo,
				v2.EnvoyComponentClient: v2.LogLevelDebug,
			},
		},
		{
			logLevelsString: "unknown_component:info",
			expectedLogLevels: v2.ComponentLogLevels{
				"unknown_component": v2.LogLevelInfo,
			},
		},
		{
			logLevelsString: "admin:non_standard_level",
			expectedLogLevels: v2.ComponentLogLevels{
				v2.EnvoyComponentAdmin: "non_standard_level",
			},
		},
		{
			logLevelsString: "bad_format",
			expectError:     true,
		},
		{
			logLevelsString: "no_level:",
			expectError:     true,
		},
		{
			logLevelsString: ":no_component",
			expectError:     true,
		},
		{
			logLevelsString: ":",
			expectError:     true,
		},
		{
			logLevelsString: "consecutive_commas:info,,",
			expectError:     true,
		},
		{
			logLevelsString: ",,,",
			expectError:     true,
		},
		{
			logLevelsString: ":,:,:",
			expectError:     true,
		},
		{
			logLevelsString: "bad_format,bad_format",
			expectError:     true,
		},
	}
	for _, tc := range testCases {
		t.Run("string="+tc.logLevelsString, func(t *testing.T) {
			logLevels, err := componentLogLevelsFromString(tc.logLevelsString)
			if tc.expectError {
				if err == nil {
					t.Fatal("Expected function call to fail, but it didn't")
				}
			} else {
				assertEquals(t, tc.expectedLogLevels, logLevels)
			}
		})
	}
}
