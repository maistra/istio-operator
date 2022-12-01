package validation

import (
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiv1 "maistra.io/api/core/v1"
	webhookadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maistrav1 "github.com/maistra/istio-operator/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/apis/maistra/v2"
	"github.com/maistra/istio-operator/controllers/common/test"
	"github.com/maistra/istio-operator/controllers/common/test/assert"
	"github.com/maistra/istio-operator/controllers/versions"
)

func init() {
	os.Setenv("POD_NAMESPACE", "openshift-operators")
}

func TestDeletedControlPlaneIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", versions.V2_2.String())
	controlPlane.DeletionTimestamp = now()

	validator := createControlPlaneValidatorTestFixture()
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.True(response.Allowed, "Expected validator to allow deleted ServiceMeshControlPlane", t)
}

func TestControlPlaneOutsideWatchedNamespaceIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlaneWithVersion("my-smcp", "not-watched", versions.V2_2.String())
	validator := createControlPlaneValidatorTestFixture()
	validator.namespaceFilter = "watched-namespace"
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.True(response.Allowed, "Expected validator to allow ServiceMeshControlPlane whose namespace isn't watched", t)
}

func TestControlPlaneWithIncorrectVersionIsRejected(t *testing.T) {
	controlPlane := newControlPlaneWithVersion("my-smcp", "not-watched", versions.V2_2.String())
	controlPlane.Spec.Version = "0.0"
	validator := createControlPlaneValidatorTestFixture()
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.False(response.Allowed, "Expected validator to reject ServiceMeshControlPlane with bad version", t)
}

func TestControlPlaneNotAllowedInOperatorNamespace(t *testing.T) {
	test.PanicOnError(os.Setenv("POD_NAMESPACE", "openshift-operators")) // TODO: make it easier to set the namespace in tests
	controlPlane := newControlPlaneWithVersion("my-smcp", "openshift-operators", versions.V2_2.String())
	validator := createControlPlaneValidatorTestFixture()
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.False(response.Allowed, "Expected validator to reject ServiceMeshControlPlane in operator's namespace", t)
}

func TestOnlyOneControlPlaneIsAllowedPerNamespace(t *testing.T) {
	controlPlane1 := newControlPlaneWithVersion("my-smcp", "istio-system", versions.V2_2.String())
	validator := createControlPlaneValidatorTestFixture(controlPlane1)
	controlPlane2 := newControlPlaneWithVersion("my-smcp2", "istio-system", versions.V2_2.String())
	response := validator.Handle(ctx, createCreateRequest(controlPlane2))
	assert.False(response.Allowed, "Expected validator to reject ServiceMeshControlPlane with bad version", t)
}

func TestControlPlaneValidation(t *testing.T) {
	enabled := true
	disabled := false
	cases := []struct {
		name                string
		controlPlane        runtime.Object
		updatedControlPlane runtime.Object
		valid               bool
		resources           []runtime.Object
	}{
		{
			name:         "blank-version",
			controlPlane: newControlPlaneWithVersion("my-smcp", "istio-system", ""),
			valid:        false,
		},
		{
			name: "v2-default-v2.0",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
				},
			},
			valid: true,
		},
		{
			name: "v2-istiod-policy-v2.0",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Policy: &maistrav2.PolicyConfig{
						Type: maistrav2.PolicyTypeIstiod,
					},
				},
			},
			valid: true,
		},
		{
			name: "v2-remote-policy-v2.0-fail",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Policy: &maistrav2.PolicyConfig{
						Remote: &maistrav2.RemotePolicyConfig{
							Address: "some.address.com",
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "v2-remote-policy-v2.0-pass",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Policy: &maistrav2.PolicyConfig{
						Type: maistrav2.PolicyTypeRemote,
						Remote: &maistrav2.RemotePolicyConfig{
							Address: "some.address.com",
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "v2-istiod-telemetry-v2.0",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Telemetry: &maistrav2.TelemetryConfig{
						Type: maistrav2.TelemetryTypeIstiod,
					},
				},
			},
			valid: true,
		},
		{
			name: "v2-remote-telemetry-v2.0-fail",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Telemetry: &maistrav2.TelemetryConfig{
						Remote: &maistrav2.RemoteTelemetryConfig{
							Address: "some.address.com",
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "v2-telemetry-mixer-adapters-v2.0-fail",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Telemetry: &maistrav2.TelemetryConfig{
						Mixer: &maistrav2.MixerTelemetryConfig{
							Adapters: &maistrav2.MixerTelemetryAdaptersConfig{
								KubernetesEnv:  &enabled,
								UseAdapterCRDs: &enabled,
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "v2-telemetry-mixer-adapters-v2.0-pass",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Telemetry: &maistrav2.TelemetryConfig{
						Mixer: &maistrav2.MixerTelemetryConfig{
							Adapters: &maistrav2.MixerTelemetryAdaptersConfig{
								KubernetesEnv: &enabled,
							},
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "v1-v2.0",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: versions.V2_0.String(),
				},
			},
			valid: false,
		},
		{
			name: "protocolSniffing.inbound.v2.0.enabled",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Proxy: &maistrav2.ProxyConfig{
						Networking: &maistrav2.ProxyNetworkingConfig{
							Protocol: &maistrav2.ProxyNetworkProtocolConfig{
								AutoDetect: &maistrav2.ProxyNetworkAutoProtocolDetectionConfig{
									Inbound: &enabled,
								},
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "protocolSniffing.outbound.v2.0.enabled",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Proxy: &maistrav2.ProxyConfig{
						Networking: &maistrav2.ProxyNetworkingConfig{
							Protocol: &maistrav2.ProxyNetworkProtocolConfig{
								AutoDetect: &maistrav2.ProxyNetworkAutoProtocolDetectionConfig{
									Outbound: &enabled,
								},
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "protocolSniffing.inbound.v2.0.disabled",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Proxy: &maistrav2.ProxyConfig{
						Networking: &maistrav2.ProxyNetworkingConfig{
							Protocol: &maistrav2.ProxyNetworkProtocolConfig{
								AutoDetect: &maistrav2.ProxyNetworkAutoProtocolDetectionConfig{
									Inbound: &disabled,
								},
							},
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "protocolSniffing.outbound.v2.0.disabled",
			controlPlane: &maistrav2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav2.ControlPlaneSpec{
					Version: versions.V2_0.String(),
					Proxy: &maistrav2.ProxyConfig{
						Networking: &maistrav2.ProxyNetworkingConfig{
							Protocol: &maistrav2.ProxyNetworkProtocolConfig{
								AutoDetect: &maistrav2.ProxyNetworkAutoProtocolDetectionConfig{
									Outbound: &disabled,
								},
							},
						},
					},
				},
			},
			valid: true,
		},
		{
			name:                "smcp.upgrade.v2.0.to.v2.3",
			controlPlane:        newControlPlaneWithVersion("basic", "istio-system", versions.V2_0.String()),
			updatedControlPlane: newControlPlaneWithVersion("basic", "istio-system", versions.V2_3.String()),
			valid:               true,
		},
		{
			name:                "sme.upgrade.to.v2.3.fail",
			controlPlane:        newControlPlaneWithVersion("basic", "istio-system", versions.V2_2.String()),
			updatedControlPlane: newControlPlaneWithVersion("basic", "istio-system", versions.V2_3.String()),
			valid:               false,
			resources: []runtime.Object{
				&apiv1.ServiceMeshExtension{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "istio-system",
					},
					Spec: apiv1.ServiceMeshExtensionSpec{
						Config: apiv1.ServiceMeshExtensionConfig{
							Data: map[string]interface{}{},
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validator := createControlPlaneValidatorTestFixture(tc.resources...)
			response := validator.Handle(ctx, createCreateRequest(tc.controlPlane))
			if tc.updatedControlPlane != nil {
				response = validator.Handle(ctx, createUpdateRequest(tc.controlPlane, tc.updatedControlPlane))
			}

			if tc.valid {
				var reason string
				if response.Result != nil {
					reason = response.Result.Message
				}
				assert.True(response.Allowed, "Expected validator to accept valid ServiceMeshControlPlane, but rejected: "+reason, t)
			} else {
				assert.False(response.Allowed, "Expected validator to reject invalid ServiceMeshControlPlane", t)
			}
		})
	}
}

func TestFullAffinityOnlySupportedForKiali(t *testing.T) {
	cases := []struct {
		name                   string
		allowedForKiali        bool
		componentRuntimeConfig maistrav2.ComponentRuntimeConfig
	}{
		{
			name:            "nodeAffinity",
			allowedForKiali: true,
			componentRuntimeConfig: maistrav2.ComponentRuntimeConfig{
				Pod: &maistrav2.PodRuntimeConfig{
					Affinity: &maistrav2.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchFields: []corev1.NodeSelectorRequirement{
											{
												Key:      "key1",
												Operator: "op1",
												Values:   []string{"value11", "value12"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:            "podAffinity",
			allowedForKiali: true,
			componentRuntimeConfig: maistrav2.ComponentRuntimeConfig{
				Pod: &maistrav2.PodRuntimeConfig{
					Affinity: &maistrav2.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"fookey": "foovalue",
										},
									},
									Namespaces:  []string{"ns1", "ns2"},
									TopologyKey: "my-topology-key",
								},
							},
						},
					},
				},
			},
		},
		{
			name:            "podAntiAffinity.corev1",
			allowedForKiali: true,
			componentRuntimeConfig: maistrav2.ComponentRuntimeConfig{
				Pod: &maistrav2.PodRuntimeConfig{
					Affinity: &maistrav2.Affinity{
						PodAntiAffinity: maistrav2.PodAntiAffinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"bazkey": "bazvalue",
											},
										},
										Namespaces:  []string{"ns5", "ns6"},
										TopologyKey: "my-topology-key3",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:            "podAntiAffinity.maistra",
			allowedForKiali: false,
			componentRuntimeConfig: maistrav2.ComponentRuntimeConfig{
				Pod: &maistrav2.PodRuntimeConfig{
					Affinity: &maistrav2.Affinity{
						PodAntiAffinity: maistrav2.PodAntiAffinity{
							RequiredDuringScheduling: []maistrav2.PodAntiAffinityTerm{
								{
									LabelSelectorRequirement: metav1.LabelSelectorRequirement{
										Key:      "key1",
										Operator: "op1",
										Values:   []string{"value11", "value12"},
									},
									TopologyKey: "my-topology-key",
								},
							},
							PreferredDuringScheduling: nil,
						},
					},
				},
			},
		},
	}

	for _, component := range maistrav2.ControlPlaneComponentNames {
		for _, tc := range cases {
			t.Run(string(component)+"."+tc.name, func(t *testing.T) {
				validator := createControlPlaneValidatorTestFixture()

				controlPlane := &maistrav2.ServiceMeshControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-smcp",
						Namespace: "istio-system",
					},
					Spec: maistrav2.ControlPlaneSpec{
						Version: versions.V2_1.String(),
						Runtime: &maistrav2.ControlPlaneRuntimeConfig{
							Components: map[maistrav2.ControlPlaneComponentName]*maistrav2.ComponentRuntimeConfig{
								component: &tc.componentRuntimeConfig,
							},
						},
					},
				}

				response := validator.Handle(ctx, createCreateRequest(controlPlane))
				if (tc.allowedForKiali && component == maistrav2.ControlPlaneComponentNameKiali) ||
					(!tc.allowedForKiali && component != maistrav2.ControlPlaneComponentNameKiali) {
					var reason string
					if response.Result != nil {
						reason = response.Result.Message
					}
					assert.True(response.Allowed, "Expected validator to accept valid ServiceMeshControlPlane, but rejected: "+reason, t)
				} else {
					assert.False(response.Allowed, "Expected validator to reject invalid ServiceMeshControlPlane", t)
				}
			})
		}
	}
}

func TestInvalidVersion(t *testing.T) {
	validControlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0")
	invalidControlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "InvalidVersion")
	createValidator := createControlPlaneValidatorTestFixture()
	updateValidator := createControlPlaneValidatorTestFixture(validControlPlane)
	cases := []struct {
		name      string
		request   webhookadmission.Request
		validator *ControlPlaneValidator
	}{
		{
			name:      "create",
			request:   createCreateRequest(invalidControlPlane),
			validator: createValidator,
		},
		{
			name:      "update",
			request:   createUpdateRequest(validControlPlane, invalidControlPlane),
			validator: updateValidator,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := tc.validator.Handle(ctx, tc.request)
			assert.False(response.Allowed, "Expected validator to reject invalid ServiceMeshControlPlane", t)
		})
	}
}

// v1.1 is deprecated and skip TestVersionValidation
func TestVersionValidation(t *testing.T) {
	t.Skip("v1.1 is deprecated and skip TestVersionValidation")
	type subcase struct {
		name      string
		smcp      *maistrav2.ServiceMeshControlPlane
		configure func(smcp *maistrav2.ServiceMeshControlPlane)
		allowed   bool
	}

	cases := []struct {
		name  string
		cases []subcase
	}{
		{
			name: "v1.1",
			cases: []subcase{
				{
					name:      "valid",
					smcp:      newControlPlaneWithVersion("my-smcp", "istio-system", "v1.1"),
					configure: func(smcp *maistrav2.ServiceMeshControlPlane) {},
					allowed:   true,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, tc := range tc.cases {
				t.Run(tc.name, func(t *testing.T) {
					validator := createControlPlaneValidatorTestFixture()
					tc.configure(tc.smcp)
					response := validator.Handle(ctx, createCreateRequest(tc.smcp))
					if tc.allowed {
						defer func() {
							if t.Failed() {
								t.Logf("Unexpected validation Error: %s", response.Result.Message)
							}
						}()
						assert.True(response.Allowed, "Expected validator to accept ServiceMeshControlPlane", t)
					} else {
						assert.False(response.Allowed, "Expected validator to reject ServiceMeshControlPlane", t)
						t.Logf("Validation Error: %s", response.Result.Message)
					}
				})
			}
		})
	}
}

func createControlPlaneValidatorTestFixture(clientObjects ...runtime.Object) *ControlPlaneValidator {
	cl, tracker := test.CreateClient(clientObjects...)
	s := tracker.Scheme

	decoder, err := webhookadmission.NewDecoder(s)
	if err != nil {
		panic(fmt.Sprintf("Could not create decoder: %s", err))
	}
	validator := NewControlPlaneValidator("")

	err = validator.InjectClient(cl)
	if err != nil {
		panic(fmt.Sprintf("Could not inject client: %s", err))
	}

	err = validator.InjectDecoder(decoder)
	if err != nil {
		panic(fmt.Sprintf("Could not inject decoder: %s", err))
	}

	return validator
}

func newControlPlaneWithVersion(name, namespace, version string) *maistrav2.ServiceMeshControlPlane {
	controlPlane := &maistrav2.ServiceMeshControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: maistrav2.ControlPlaneSpec{},
	}
	controlPlane.Spec.Version = version
	return controlPlane
}
