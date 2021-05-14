package controlplane

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

const (
	PRODUCT_IMAGE_2_0   = "2.0.5"
	COMMUNITY_IMAGE_2_0 = "2.0.5"
	PRODUCT_IMAGE_1_1   = "1.1.15"
)

func TestProfiles(t *testing.T) {

	profileTestCases := []struct {
		name            string
		v               versions.Version
		input           *v2.ServiceMeshControlPlane
		expected        *v2.ServiceMeshControlPlane
		patchupExpected func(*v2.ServiceMeshControlPlane)
		skip            bool
	}{
		{
			name: "maistra-profile-1.1",
			v:    versions.V1_1,
			input: &v2.ServiceMeshControlPlane{
				Spec: v2.ControlPlaneSpec{
					Profiles: []string{"maistra"},
				},
			},
			expected: &v2.ServiceMeshControlPlane{
				Spec: v1_1_ExpectedSpec,
			},
		},
		{
			name: "servicemesh-profile-1.1",
			v:    versions.V1_1,
			input: &v2.ServiceMeshControlPlane{
				Spec: v2.ControlPlaneSpec{
					Profiles: []string{"servicemesh"},
				},
			},
			expected: &v2.ServiceMeshControlPlane{
				Spec: v1_1_ExpectedSpec,
			},
			patchupExpected: func(smcp *v2.ServiceMeshControlPlane) {
				// the only thing changing here should be image names/tags/registries
				smcp.Spec.Runtime.Defaults.Container.ImageRegistry = "registry.redhat.io/openshift-service-mesh"
				smcp.Spec.Runtime.Components[v2.ControlPlaneComponentNameThreeScale].Container.ImageRegistry = "registry.redhat.io/openshift-service-mesh"
				smcp.Spec.Runtime.Components[v2.ControlPlaneComponentNameThreeScale].Container.ImageTag = "1.0.0"
				smcp.Spec.Runtime.Components[v2.ControlPlaneComponentNameGlobalOauthProxy].Container.ImageRegistry = "registry.redhat.io/openshift4"
				smcp.Spec.Runtime.Components[v2.ControlPlaneComponentNameGlobalOauthProxy].Container.Image = "ose-oauth-proxy"
			},
		},
		{
			name: "maistra-profile-2.0",
			v:    versions.V2_0,
			input: &v2.ServiceMeshControlPlane{
				Spec: v2.ControlPlaneSpec{
					Profiles: []string{"maistra"},
				},
			},
			expected: &v2.ServiceMeshControlPlane{
				Spec: v2_0_ExpectedSpec,
			},
		},
		{
			name: "servicemesh-profile-2.0",
			v:    versions.V2_0,
			input: &v2.ServiceMeshControlPlane{
				Spec: v2.ControlPlaneSpec{
					Profiles: []string{"servicemesh"},
				},
			},
			expected: &v2.ServiceMeshControlPlane{
				Spec: v2_0_ExpectedSpec,
			},
			patchupExpected: func(smcp *v2.ServiceMeshControlPlane) {
				// the only thing changing here should be image names/tags/registries
				smcp.Spec.Runtime.Defaults.Container.ImageRegistry = "registry.redhat.io/openshift-service-mesh"
				smcp.Spec.Runtime.Components[v2.ControlPlaneComponentNameThreeScale].Container.ImageRegistry = "registry.redhat.io/openshift-service-mesh"
				smcp.Spec.Runtime.Components[v2.ControlPlaneComponentNameThreeScale].Container.ImageTag = "2.0.0"
				smcp.Spec.Runtime.Components[v2.ControlPlaneComponentNameGlobalOauthProxy].Container.ImageRegistry = "registry.redhat.io/openshift4"
				smcp.Spec.Runtime.Components[v2.ControlPlaneComponentNameGlobalOauthProxy].Container.Image = "ose-oauth-proxy"
				smcp.Spec.Runtime.Components[v2.ControlPlaneComponentNameGlobalOauthProxy].Container.ImageTag = "v4.4"
				smcp.Spec.Runtime.Defaults.Container.ImageTag = PRODUCT_IMAGE_2_0
			},
		},
	}
	testNamespace := "test-namespace"
	testName := "test"
	testCR := &common.ControllerResources{
		Scheme: test.GetScheme(),
	}
	InitializeGlobals("operator-namespace")()
	for _, tc := range profileTestCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skip()
			}
			var err error
			testSMCP := tc.input.DeepCopy()
			testSMCP.Spec.Version = tc.v.String()
			testSMCP.Name = testName
			testSMCP.Namespace = testNamespace
			v1smcp := &v1.ServiceMeshControlPlane{}
			if err = v1smcp.ConvertFrom(testSMCP); err != nil {
				t.Fatalf("unexpected error converting input: %v", err)
			}
			appliedV1SMCP := &v1.ServiceMeshControlPlane{}
			if appliedV1SMCP.Spec, err = tc.v.Strategy().ApplyProfiles(context.TODO(), testCR, &v1smcp.Spec, testNamespace); err != nil {
				t.Fatalf("unexpected error applying profiles: %v", err)
			}
			appliedV2SMCP := &v2.ServiceMeshControlPlane{}
			if err = appliedV1SMCP.ConvertTo(appliedV2SMCP); err != nil {
				t.Fatalf("unexpected error converting output: %v", err)
			}
			expected := tc.expected.DeepCopy()
			expected.Spec.Version = tc.v.String()
			expected.Spec.Profiles = append([]string(nil), tc.input.Spec.Profiles...)
			if tc.patchupExpected != nil {
				tc.patchupExpected(expected)
			}
			if diff := cmp.Diff(expected.Spec, appliedV2SMCP.Spec, cmp.AllowUnexported(v1.HelmValues{})); diff != "" {
				t.Errorf("TestProfiles() case %s mismatch (-want +got):\n%s", tc.name, diff)
			}
		})
	}
}

var (
	v1_1_ExpectedSpec = v2.ControlPlaneSpec{
		Proxy: &v2.ProxyConfig{
			Networking: &v2.ProxyNetworkingConfig{
				Initialization: &v2.ProxyNetworkInitConfig{
					InitContainer: &v2.ProxyInitContainerConfig{
						Runtime: &v2.ContainerConfig{
							Image: "injected-proxy-init-v1.1",
						},
					},
				},
				DNS: &v2.ProxyDNSConfig{
					RefreshRate: "300s",
				},
			},
			Runtime: &v2.ProxyRuntimeConfig{
				Container: &v2.ContainerConfig{
					CommonContainerConfig: v2.CommonContainerConfig{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					},
					Image: "injected-proxyv2-v1.1",
				},
			},
			Injection: &v2.ProxyInjectionConfig{
				AutoInject: &falseVal,
			},
		},
		Tracing: &v2.TracingConfig{
			Type: v2.TracerTypeJaeger,
		},
		Gateways: &v2.GatewaysConfig{
			ClusterIngress: &v2.ClusterIngressGatewayConfig{
				IngressGatewayConfig: v2.IngressGatewayConfig{
					GatewayConfig: v2.GatewayConfig{
						Enablement: v2.Enablement{Enabled: &trueVal},
						Service: v2.GatewayServiceConfig{
							ServiceSpec: corev1.ServiceSpec{
								Type: corev1.ServiceTypeClusterIP,
							},
						},
						Runtime: &v2.ComponentRuntimeConfig{
							Container: &v2.ContainerConfig{
								CommonContainerConfig: v2.CommonContainerConfig{
									Resources: &corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("10m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			ClusterEgress: &v2.EgressGatewayConfig{
				GatewayConfig: v2.GatewayConfig{
					Enablement: v2.Enablement{Enabled: &trueVal},
					Runtime: &v2.ComponentRuntimeConfig{
						Container: &v2.ContainerConfig{
							CommonContainerConfig: v2.CommonContainerConfig{
								Resources: &corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
							},
						},
					},
				},
			},
			OpenShiftRoute: &v2.OpenShiftRouteConfig{
				Enablement: v2.Enablement{Enabled: &falseVal},
			},
		},
		Addons: &v2.AddonsConfig{
			Jaeger: &v2.JaegerAddonConfig{
				Install: &v2.JaegerInstallConfig{
					Storage: &v2.JaegerStorageConfig{
						Type: v2.JaegerStorageTypeMemory,
					},
					Ingress: &v2.JaegerIngressConfig{
						Enablement: v2.Enablement{Enabled: &trueVal},
					},
				},
			},
			Grafana: &v2.GrafanaAddonConfig{
				Enablement: v2.Enablement{Enabled: &trueVal},
				Install: &v2.GrafanaInstallConfig{
					Service: &v2.ComponentServiceConfig{
						Metadata: &v2.MetadataConfig{
							Annotations: map[string]string{
								"service.alpha.openshift.io/serving-cert-secret-name": "grafana-tls",
							},
						},
						Ingress: &v2.ComponentIngressConfig{
							Enablement: v2.Enablement{Enabled: &trueVal},
						},
					},
				},
			},
			Kiali: &v2.KialiAddonConfig{
				Enablement: v2.Enablement{Enabled: &trueVal},
				Install: &v2.KialiInstallConfig{
					Dashboard: &v2.KialiDashboardConfig{ViewOnly: &falseVal},
					Service: &v2.ComponentServiceConfig{
						Ingress: &v2.ComponentIngressConfig{
							Enablement: v2.Enablement{Enabled: &trueVal},
						},
					},
				},
			},
			Prometheus: &v2.PrometheusAddonConfig{
				// XXX: prometheus is not explicitly enabled in the profile
				//Enablement: v2.Enablement{Enabled: &trueVal},
				Install: &v2.PrometheusInstallConfig{
					Service: &v2.ComponentServiceConfig{
						Metadata: &v2.MetadataConfig{
							Annotations: map[string]string{
								"service.alpha.openshift.io/serving-cert-secret-name": "prometheus-tls",
							},
						},
						Ingress: &v2.ComponentIngressConfig{
							Enablement: v2.Enablement{Enabled: &trueVal},
						},
					},
				},
			},
		},
		Runtime: &v2.ControlPlaneRuntimeConfig{
			Defaults: &v2.DefaultRuntimeConfig{
				Deployment: &v2.CommonDeploymentRuntimeConfig{
					PodDisruption: &v2.PodDisruptionBudget{
						Enablement: v2.Enablement{Enabled: &falseVal},
					},
				},
				Container: &v2.CommonContainerConfig{
					ImageRegistry: "docker.io/maistra",
					ImageTag:      PRODUCT_IMAGE_1_1,
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
			Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
				v2.ControlPlaneComponentNameGlobalOauthProxy: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							ImageRegistry:   "quay.io/openshift",
							ImageTag:        "4.2",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						Image: "origin-oauth-proxy",
					},
				},
				v2.ControlPlaneComponentNameSecurity: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						Image: "injected-citadel-v1.1",
					},
				},
				v2.ControlPlaneComponentNameGalley: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						Image: "injected-galley-v1.1",
					},
				},
				v2.ControlPlaneComponentNamePilot: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						Image: "injected-pilot-v1.1",
					},
				},
				v2.ControlPlaneComponentNameMixerTelemetry: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
				v2.ControlPlaneComponentNamePrometheus: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						Image: "injected-prometheus-v1.1",
					},
				},
				v2.ControlPlaneComponentNameGrafana: {
					Container: &v2.ContainerConfig{
						Image: "injected-grafana-v1.1",
					},
				},
				v2.ControlPlaneComponentNameMixer: {
					Container: &v2.ContainerConfig{
						Image: "injected-mixer-v1.1",
					},
				},
				v2.ControlPlaneComponentNameSidecarInjectoryWebhook: {
					Container: &v2.ContainerConfig{
						Image: "injected-sidecar-injector-v1.1",
					},
				},
				v2.ControlPlaneComponentNameThreeScale: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							ImageRegistry: "quay.io/3scale",
							ImageTag:      "v1.0.0",
						},
						Image: "injected-3scale-v1.1",
					},
				},
			},
		},
		TechPreview: v1.NewHelmValues(map[string]interface{}{
			// the following is all cruft
			"gateways": map[string]interface{}{
				"istio-ingressgateway": map[string]interface{}{
					// this is not exposed in the v2 schema, the only that that really belongs
					"ior_image": "injected-ior-v1.1",
				},
			},
			// i'm not sure if this is necessary
			"istio_cni": map[string]interface{}{
				"repair": map[string]interface{}{
					"enabled": false,
				},
			},
		}),
	}

	v2_0_ExpectedSpec = v2.ControlPlaneSpec{
		Proxy: &v2.ProxyConfig{
			Networking: &v2.ProxyNetworkingConfig{
				Initialization: &v2.ProxyNetworkInitConfig{
					InitContainer: &v2.ProxyInitContainerConfig{
						Runtime: &v2.ContainerConfig{
							Image: "injected-proxy-init-v2.0",
						},
					},
				},
				DNS: &v2.ProxyDNSConfig{
					RefreshRate: "300s",
				},
				Protocol: &v2.ProxyNetworkProtocolConfig{
					AutoDetect: &v2.ProxyNetworkAutoProtocolDetectionConfig{
						Inbound:  &falseVal,
						Outbound: &falseVal,
					},
				},
			},
			Runtime: &v2.ProxyRuntimeConfig{
				Container: &v2.ContainerConfig{
					CommonContainerConfig: v2.CommonContainerConfig{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					},
					Image: "injected-proxyv2-v2.0",
				},
			},
			Injection: &v2.ProxyInjectionConfig{
				AutoInject: &falseVal,
			},
		},
		Security: &v2.SecurityConfig{
			Identity: &v2.IdentityConfig{
				Type: v2.IdentityConfigTypeKubernetes,
			},
		},
		Telemetry: &v2.TelemetryConfig{
			Type: v2.TelemetryTypeIstiod,
		},
		Policy: &v2.PolicyConfig{
			Type: v2.PolicyTypeNone,
		},
		Tracing: &v2.TracingConfig{
			Type: v2.TracerTypeJaeger,
		},
		Gateways: &v2.GatewaysConfig{
			Enablement: v2.Enablement{Enabled: &trueVal},
			ClusterIngress: &v2.ClusterIngressGatewayConfig{
				IngressEnabled: &falseVal,
				IngressGatewayConfig: v2.IngressGatewayConfig{
					GatewayConfig: v2.GatewayConfig{
						Enablement: v2.Enablement{Enabled: &trueVal},
						Service: v2.GatewayServiceConfig{
							ServiceSpec: corev1.ServiceSpec{
								Type: corev1.ServiceTypeClusterIP,
							},
						},
						Runtime: &v2.ComponentRuntimeConfig{
							Deployment: &v2.DeploymentRuntimeConfig{
								AutoScaling: &v2.AutoScalerConfig{
									Enablement: v2.Enablement{Enabled: &falseVal},
								},
							},
							Container: &v2.ContainerConfig{
								CommonContainerConfig: v2.CommonContainerConfig{
									Resources: &corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("10m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
									},
								},
							},
						},
					},
				},
			},
			ClusterEgress: &v2.EgressGatewayConfig{
				GatewayConfig: v2.GatewayConfig{
					Enablement: v2.Enablement{Enabled: &trueVal},
					Runtime: &v2.ComponentRuntimeConfig{
						Deployment: &v2.DeploymentRuntimeConfig{
							AutoScaling: &v2.AutoScalerConfig{
								Enablement: v2.Enablement{Enabled: &falseVal},
							},
						},
						Container: &v2.ContainerConfig{
							CommonContainerConfig: v2.CommonContainerConfig{
								Resources: &corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
							},
						},
					},
				},
			},
			OpenShiftRoute: &v2.OpenShiftRouteConfig{
				Enablement: v2.Enablement{Enabled: &trueVal},
			},
		},
		General: &v2.GeneralConfig{
			Logging: &v2.LoggingConfig{
				ComponentLevels: v2.ComponentLogLevels{
					"default": "warn",
				},
			},
		},
		Addons: &v2.AddonsConfig{
			Jaeger: &v2.JaegerAddonConfig{
				Name: "jaeger",
				Install: &v2.JaegerInstallConfig{
					Storage: &v2.JaegerStorageConfig{
						Type: v2.JaegerStorageTypeMemory,
					},
					Ingress: &v2.JaegerIngressConfig{
						Enablement: v2.Enablement{Enabled: &trueVal},
					},
				},
			},
			Grafana: &v2.GrafanaAddonConfig{
				Enablement: v2.Enablement{Enabled: &trueVal},
				Install: &v2.GrafanaInstallConfig{
					Service: &v2.ComponentServiceConfig{
						Metadata: &v2.MetadataConfig{
							Annotations: map[string]string{
								"service.alpha.openshift.io/serving-cert-secret-name": "grafana-tls",
							},
						},
						Ingress: &v2.ComponentIngressConfig{
							Enablement: v2.Enablement{Enabled: &trueVal},
						},
					},
				},
			},
			Kiali: &v2.KialiAddonConfig{
				Enablement: v2.Enablement{Enabled: &trueVal},
				Name:       "kiali",
				Install: &v2.KialiInstallConfig{
					Dashboard: &v2.KialiDashboardConfig{ViewOnly: &falseVal},
					Service: &v2.ComponentServiceConfig{
						Ingress: &v2.ComponentIngressConfig{
							Enablement: v2.Enablement{Enabled: &trueVal},
						},
					},
				},
			},
			Prometheus: &v2.PrometheusAddonConfig{
				Enablement: v2.Enablement{Enabled: &trueVal},
				Install: &v2.PrometheusInstallConfig{
					Service: &v2.ComponentServiceConfig{
						Metadata: &v2.MetadataConfig{
							Annotations: map[string]string{
								"service.alpha.openshift.io/serving-cert-secret-name": "prometheus-tls",
							},
						},
						Ingress: &v2.ComponentIngressConfig{
							Enablement: v2.Enablement{Enabled: &trueVal},
						},
					},
				},
			},
		},
		Runtime: &v2.ControlPlaneRuntimeConfig{
			Defaults: &v2.DefaultRuntimeConfig{
				Deployment: &v2.CommonDeploymentRuntimeConfig{
					PodDisruption: &v2.PodDisruptionBudget{
						Enablement: v2.Enablement{Enabled: &falseVal},
					},
				},
				Container: &v2.CommonContainerConfig{
					ImageRegistry: "docker.io/maistra",
					ImageTag:      COMMUNITY_IMAGE_2_0,
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
			Components: map[v2.ControlPlaneComponentName]*v2.ComponentRuntimeConfig{
				v2.ControlPlaneComponentNameGlobalOauthProxy: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							ImageRegistry:   "quay.io/openshift",
							ImageTag:        "4.4",
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						Image: "origin-oauth-proxy",
					},
				},
				v2.ControlPlaneComponentNamePilot: {
					Deployment: &v2.DeploymentRuntimeConfig{
						AutoScaling: &v2.AutoScalerConfig{
							Enablement: v2.Enablement{Enabled: &falseVal},
						},
					},
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						Image: "injected-pilot-v2.0",
					},
				},
				v2.ControlPlaneComponentNameMixer: {
					Container: &v2.ContainerConfig{
						Image: "injected-mixer-v2.0",
					},
				},
				v2.ControlPlaneComponentNameMixerTelemetry: {
					Deployment: &v2.DeploymentRuntimeConfig{
						AutoScaling: &v2.AutoScalerConfig{
							Enablement: v2.Enablement{Enabled: &falseVal},
						},
					},
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						Image: "injected-mixer-v2.0",
					},
				},
				v2.ControlPlaneComponentNameMixerPolicy: {
					Deployment: &v2.DeploymentRuntimeConfig{
						AutoScaling: &v2.AutoScalerConfig{
							Enablement: v2.Enablement{Enabled: &falseVal},
						},
					},
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						Image: "injected-mixer-v2.0",
					},
				},
				v2.ControlPlaneComponentNamePrometheus: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						Image: "injected-prometheus-v2.0",
					},
				},
				v2.ControlPlaneComponentNameGrafana: {
					Container: &v2.ContainerConfig{
						Image: "injected-grafana-v2.0",
					},
				},
				v2.ControlPlaneComponentNameThreeScale: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							ImageRegistry: "quay.io/3scale",
							ImageTag:      "v2.0.0",
						},
						Image: "injected-3scale-v2.0",
					},
				},
				v2.ControlPlaneComponentNameWASMCacher: {
					Container: &v2.ContainerConfig{
						CommonContainerConfig: v2.CommonContainerConfig{
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						Image: "injected-wasm-cacher-v2.0",
					},
				},
			},
		},
		TechPreview: v1.NewHelmValues(map[string]interface{}{
			"sidecarInjectorWebhook": map[string]interface{}{
				"objectSelector": map[string]interface{}{
					"enabled": false,
				},
			},
			"wasmExtensions": map[string]interface{}{
				"enabled": false,
			},
		}),
	}
)
