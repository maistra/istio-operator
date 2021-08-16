package controlplane

import (
	"fmt"
	"strings"
	"testing"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	. "github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/versions"
	"k8s.io/apimachinery/pkg/util/sets"
	clienttesting "k8s.io/client-go/testing"
)

func TestRLS(t *testing.T) {
	disabledCreateAssertions := ActionAssertions{
		Assert("create").On("deployments").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsNotSeen(),
		Assert("create").On("services").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsNotSeen(),
		Assert("create").On("configmaps").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsNotSeen(),
	}
	disabledDeleteAssertions := ActionAssertions{
		Assert("delete").On("deployments").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsNotSeen(),
		Assert("delete").On("services").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsNotSeen(),
		Assert("delete").On("configmaps").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsNotSeen(),
	}

	testCases := []IntegrationTestCase{
		{
			name: "rls.enabled",
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				Version: versions.V2_1.String(),
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"rateLimiting": map[string]interface{}{
						"rls": map[string]interface{}{
							"enabled": true,
						},
					},
				}),
			}),
			create: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("create").On("deployments").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsSeen(),
					Assert("create").On("services").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsSeen(),
					Assert("create").On("configmaps").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsSeen(),
				},
			},
			delete: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("delete").On("deployments").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsSeen(),
					Assert("delete").On("services").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsSeen(),
					Assert("delete").On("configmaps").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).IsSeen(),
				},
			},
		},
		{
			name: "rls.disabled",
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				Version: versions.V2_1.String(),
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"rateLimiting": map[string]interface{}{
						"rls": map[string]interface{}{
							"enabled": false,
						},
					},
				}),
			}),
			create: IntegrationTestValidation{
				Assertions: disabledCreateAssertions,
			},
			delete: IntegrationTestValidation{
				Assertions: disabledDeleteAssertions,
			},
		},
		{
			name: "rls.missing",
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				Version: versions.V2_1.String(),
			}),
			create: IntegrationTestValidation{
				Assertions: disabledCreateAssertions,
			},
			delete: IntegrationTestValidation{
				Assertions: disabledDeleteAssertions,
			},
		},
		{
			name: "rls.memcache",
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				Version: versions.V2_1.String(),
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"rateLimiting": map[string]interface{}{
						"rls": map[string]interface{}{
							"enabled":        true,
							"storageBackend": "memcache",
							"storageAddress": "1.2.3.4:1234",
						},
					},
				}),
			}),
			create: IntegrationTestValidation{
				Verifier: ActionVerifier(
					Verify("create").On("deployments").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).Passes(
						checkEnvVariables(
							map[string]string{
								"BACKEND_TYPE":       "memcache",
								"MEMCACHE_HOST_PORT": "1.2.3.4:1234",
							}, []string{"REDIS_URL", "REDIS_SOCKET_TYPE"},
						)),
				),
			},
		},
		{
			name: "rls.redis",
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				Version: versions.V2_1.String(),
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"rateLimiting": map[string]interface{}{
						"rls": map[string]interface{}{
							"enabled":        true,
							"storageBackend": "redis",
							"storageAddress": "1.2.3.4:1234",
						},
					},
				}),
			}),
			create: IntegrationTestValidation{
				Verifier: ActionVerifier(
					Verify("create").On("deployments").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).Passes(
						checkEnvVariables(
							map[string]string{
								"REDIS_SOCKET_TYPE": "tcp",
								"REDIS_URL":         "1.2.3.4:1234",
							},
							[]string{"BACKEND_TYPE", "MEMCACHE_HOST_PORT"},
						)),
				),
			},
		},
		{
			name: "rls.runtime",
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				Version: versions.V2_1.String(),
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"rateLimiting": map[string]interface{}{
						"rls": map[string]interface{}{
							"enabled": true,
						},
					},
				}),
				Runtime: &v2.ControlPlaneRuntimeConfig{
					Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
						v2.ControlPlaneComponentNameRateLimiting: {
							Container: &v2.ContainerConfig{
								Env: map[string]string{
									"FOO": "BAR",
								}},
						},
					},
				},
			}),
			create: IntegrationTestValidation{
				Verifier: ActionVerifier(
					Verify("create").On("deployments").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).Passes(
						checkEnvVariables(
							map[string]string{
								"FOO": "BAR",
							},
							[]string{"BACKEND_TYPE", "MEMCACHE_HOST_PORT", "REDIS_URL", "REDIS_SOCKET_TYPE"},
						)),
				),
			},
		},
		{
			name: "rls.configmap",
			smcp: NewV2SMCPResource(controlPlaneName, controlPlaneNamespace, &v2.ControlPlaneSpec{
				Version: versions.V2_1.String(),
				TechPreview: v1.NewHelmValues(map[string]interface{}{
					"rateLimiting": map[string]interface{}{
						"rls": map[string]interface{}{
							"enabled": true,
						},
						"rawRules": map[string]interface{}{
							"key1": "value1",
							"key2": "value2",
						},
					},
				}),
			}),
			create: IntegrationTestValidation{
				Verifier: ActionVerifier(
					Verify("create").On("configmaps").Named("rls-" + controlPlaneName).In(controlPlaneNamespace).Passes(func(action clienttesting.Action) error {
						createAction := action.(clienttesting.CreateAction)
						cm, err := common.ConvertObjectToConfigMap(createAction.GetObject())
						if err != nil {
							return err
						}

						content, ok := cm.Data["config.yaml"]
						if !ok {
							return fmt.Errorf("config.yaml entry not found in rls ConfigMap")
						}
						if !strings.Contains(content, "key1: value1") || !strings.Contains(content, "key2: value2") {
							return fmt.Errorf("invalid content in rls ConfigMap")
						}

						return nil
					}),
				),
			},
		},
	}
	RunSimpleInstallTest(t, testCases)
}

func checkEnvVariables(mustHaveVariables map[string]string, mustNotHaveVariables []string) func(action clienttesting.Action) error {
	return func(action clienttesting.Action) error {
		createAction := action.(clienttesting.CreateAction)
		deployment, err := common.ConvertObjectToDeployment(createAction.GetObject())
		if err != nil {
			return err
		}
		if len(deployment.Spec.Template.Spec.Containers) == 0 {
			return fmt.Errorf("invalid number of containers in rls deployment")
		}

		containerEnvFull := sets.NewString()
		containerEnvNames := sets.NewString()
		for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
			containerEnvFull.Insert(env.Name + env.Value)
			containerEnvNames.Insert(env.Name)
		}

		for name, value := range mustHaveVariables {
			if !containerEnvFull.Has(name + value) {
				return fmt.Errorf("variable %s=%s not found in container Env", name, value)
			}
		}

		for _, name := range mustNotHaveVariables {
			if containerEnvNames.Has(name) {
				return fmt.Errorf("variable %s found in container Env", name)
			}
		}

		return nil
	}
}
