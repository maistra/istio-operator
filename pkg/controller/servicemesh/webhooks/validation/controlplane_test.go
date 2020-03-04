package validation

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	webhookadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"

	maistra "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func TestDeletedControlPlaneIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "istio-system")
	controlPlane.DeletionTimestamp = now()

	validator, _, _ := createControlPlaneValidatorTestFixture()
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.True(response.Response.Allowed, "Expected validator to allow deleted ServiceMeshControlPlane", t)
}

func TestControlPlaneOutsideWatchedNamespaceIsAlwaysAllowed(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "not-watched")
	validator, _, _ := createControlPlaneValidatorTestFixture()
	validator.namespaceFilter = "watched-namespace"
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.True(response.Response.Allowed, "Expected validator to allow ServiceMeshControlPlane whose namespace isn't watched", t)
}

func TestControlPlaneWithIncorrectVersionIsRejected(t *testing.T) {
	controlPlane := newControlPlane("my-smcp", "not-watched")
	controlPlane.Spec.Version = "0.0"
	validator, _, _ := createControlPlaneValidatorTestFixture()
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshControlPlane with bad version", t)
}

func TestOnlyOneControlPlaneIsAllowedPerNamespace(t *testing.T) {
	controlPlane1 := newControlPlane("my-smcp", "istio-system")
	validator, _, _ := createControlPlaneValidatorTestFixture(controlPlane1)
	controlPlane2 := newControlPlane("my-smcp2", "istio-system")
	response := validator.Handle(ctx, createCreateRequest(controlPlane2))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshControlPlane with bad version", t)
}

func TestControlPlaneValidation(t *testing.T) {
	cases := []struct {
		name         string
		controlPlane *maistra.ServiceMeshControlPlane
		valid        bool
	}{
		{
			name:         "blank-version",
			controlPlane: newControlPlane("my-smcp", "istio-system"),
			valid:        true,
		},
		{
			name:         "version-1.0",
			controlPlane: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
			valid:        true,
		},
		{
			name:         "version-1.1",
			controlPlane: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.1"),
			valid:        true,
		},
		{
			name: "jaeger-enabled-despite-external-uri",
			controlPlane: &maistra.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistra.ControlPlaneSpec{
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tracer": map[string]interface{}{
								"zipkin": map[string]interface{}{
									"address": "someservice",
								},
							},
						},
						"tracing": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "jaeger-external-uri-wrong-namespace",
			controlPlane: &maistra.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistra.ControlPlaneSpec{
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tracer": map[string]interface{}{
								"zipkin": map[string]interface{}{
									"address": "jaeger-query.othernamespace",
								},
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "jaeger-external-uri-correct-namespace",
			controlPlane: &maistra.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistra.ControlPlaneSpec{
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tracer": map[string]interface{}{
								"zipkin": map[string]interface{}{
									"address": "jaeger-query.istio-system.svc.cluster.local",
								},
							},
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "jaeger-external-uri-no-namespace",
			controlPlane: &maistra.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistra.ControlPlaneSpec{
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tracer": map[string]interface{}{
								"zipkin": map[string]interface{}{
									"address": "jaeger-query",
								},
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "zipkin-address-wrong-tracer",
			controlPlane: &maistra.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistra.ControlPlaneSpec{
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"proxy": map[string]interface{}{
								"tracer": "lightstep",
							},
							"tracer": map[string]interface{}{
								"zipkin": map[string]interface{}{
									"address": "jaeger-query.istio-system",
								},
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "zipkin-address-but-tracing-enabled",
			controlPlane: &maistra.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistra.ControlPlaneSpec{
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tracer": map[string]interface{}{
								"zipkin": map[string]interface{}{
									"address": "jaeger-query.istio-system",
								},
							},
						},
						"tracing": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "zipkin-address-but-no-jaegerInClusterURL",
			controlPlane: &maistra.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistra.ControlPlaneSpec{
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tracer": map[string]interface{}{
								"zipkin": map[string]interface{}{
									"address": "jaeger-query.istio-system",
								},
							},
						},
						"kiali": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "zipkin-address-with-jaegerInClusterURL",
			controlPlane: &maistra.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistra.ControlPlaneSpec{
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tracer": map[string]interface{}{
								"zipkin": map[string]interface{}{
									"address": "jaeger-query.istio-system",
								},
							},
						},
						"kiali": map[string]interface{}{
							"enabled":            true,
							"jaegerInClusterURL": "jaeger-collector.istio-system",
						},
					},
				},
			},
			valid: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validator, _, _ := createControlPlaneValidatorTestFixture()
			response := validator.Handle(ctx, createCreateRequest(tc.controlPlane))
			if tc.valid {
				var reason string
				if response.Response.Result != nil {
					reason = response.Response.Result.Message
				}
				assert.True(response.Response.Allowed, "Expected validator to accept valid ServiceMeshControlPlane, but rejected: "+reason, t)
			} else {
				assert.False(response.Response.Allowed, "Expected validator to reject invalid ServiceMeshControlPlane", t)
			}
		})
	}
}

func TestUpdateOfValidControlPlane(t *testing.T) {
	oldControlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0")
	validator, _, _ := createControlPlaneValidatorTestFixture(oldControlPlane)

	controlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "v1.1")
	response := validator.Handle(ctx, createUpdateRequest(oldControlPlane, controlPlane))
	assert.True(response.Response.Allowed, "Expected validator to accept update of valid ServiceMeshControlPlane", t)
}

func TestInvalidVersion(t *testing.T) {
	validControlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0")
	invalidControlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "InvalidVersion")
	createValidator, _, _ := createControlPlaneValidatorTestFixture()
	updateValidator, _, _ := createControlPlaneValidatorTestFixture(validControlPlane)
	cases := []struct {
		name      string
		request   atypes.Request
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
			assert.False(response.Response.Allowed, "Expected validator to reject invalid ServiceMeshControlPlane", t)
		})
	}
}

func TestVersionValidation(t *testing.T) {
	type subcase struct {
		name      string
		smcp      *maistra.ServiceMeshControlPlane
		configure func(smcp *maistra.ServiceMeshControlPlane)
		allowed   bool
	}

	cases := []struct {
		name  string
		cases []subcase
	}{
		{
			name: "v1.0",
			cases: []subcase{
				{
					name:      "valid",
					smcp:      newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {},
					allowed:   true,
				},
				{
					name: "global.proxy.alwaysInjectSelector=false",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {
						unstructured.SetNestedField(smcp.Spec.Istio, false, strings.Split("global.proxy.alwaysInjectSelector", ".")...)
					},
					allowed: true,
				},
				{
					name: "global.proxy.alwaysInjectSelector=true",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {
						unstructured.SetNestedField(smcp.Spec.Istio, true, strings.Split("global.proxy.alwaysInjectSelector", ".")...)
					},
					allowed: false,
				},
				{
					name: "global.proxy.neverInjectSelector=false",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {
						unstructured.SetNestedField(smcp.Spec.Istio, false, strings.Split("global.proxy.neverInjectSelector", ".")...)
					},
					allowed: true,
				},
				{
					name: "global.proxy.neverInjectSelector=true",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {
						unstructured.SetNestedField(smcp.Spec.Istio, true, strings.Split("global.proxy.neverInjectSelector", ".")...)
					},
					allowed: false,
				},
				{
					name: "global.proxy.envoyAccessLogService.enabled=false",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {
						unstructured.SetNestedField(smcp.Spec.Istio, false, strings.Split("global.proxy.envoyAccessLogService.enabled", ".")...)
					},
					allowed: true,
				},
				{
					name: "global.proxy.envoyAccessLogService.enabled=true",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {
						unstructured.SetNestedField(smcp.Spec.Istio, true, strings.Split("global.proxy.envoyAccessLogService.enabled", ".")...)
					},
					allowed: false,
				},
				{
					name: "telemetry.enabled=false, telemetry.v2.enabled=false",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {
						unstructured.SetNestedField(smcp.Spec.Istio, false, strings.Split("telemetry.enabled", ".")...)
						unstructured.SetNestedField(smcp.Spec.Istio, false, strings.Split("telemetry.v2.enabled", ".")...)
					},
					allowed: true,
				},
				{
					name: "telemetry.enabled=false, telemetry.v2.enabled=true",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {
						unstructured.SetNestedField(smcp.Spec.Istio, false, strings.Split("telemetry.enabled", ".")...)
						unstructured.SetNestedField(smcp.Spec.Istio, true, strings.Split("telemetry.v2.enabled", ".")...)
					},
					allowed: true,
				},
				{
					name: "telemetry.enabled=true, telemetry.v2.enabled=true",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {
						unstructured.SetNestedField(smcp.Spec.Istio, true, strings.Split("telemetry.enabled", ".")...)
						unstructured.SetNestedField(smcp.Spec.Istio, true, strings.Split("telemetry.v2.enabled", ".")...)
					},
					allowed: false,
				},
			},
		},
		{
			name: "v1.1",
			cases: []subcase{
				{
					name:      "valid",
					smcp:      newControlPlaneWithVersion("my-smcp", "istio-system", "v1.1"),
					configure: func(smcp *maistra.ServiceMeshControlPlane) {},
					allowed:   true,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, tc := range tc.cases {
				t.Run(tc.name, func(t *testing.T) {
					validator, _, _ := createControlPlaneValidatorTestFixture()
					tc.configure(tc.smcp)
					response := validator.Handle(ctx, createCreateRequest(tc.smcp))
					if tc.allowed {
						assert.True(response.Response.Allowed, "Expected validator to accept ServiceMeshControlPlane", t)
					} else {
						assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshControlPlane", t)
						t.Logf("Validation Error: %s", response.Response.Result.Message)
					}
				})
			}
		})
	}
}

func TestVersionUpgrade1_0To1_1(t *testing.T) {
	v1_0ControlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0")
	v1_1ControlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "v1.1")
	v1_0ControlPlane.SetUID("random-uid")
	v1_1ControlPlane.SetUID("random-uid")
	cases := []struct {
		name      string
		allowed   bool
		resources []runtime.Object
	}{
		{
			name:    "valid",
			allowed: true,
		},
		{
			name:    "unsupported-resource-other-namespace",
			allowed: true,
			resources: []runtime.Object{
				newDummyResource("dummy-stdio", "other-namespace",
					schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "stdio"},
					nil,
					nil),
			},
		},
		{
			name:    "unsupported-resource-controller-owned",
			allowed: true,
			resources: []runtime.Object{
				newDummyResource("dummy-stdio", "istio-system",
					schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "stdio"},
					nil,
					v1_0ControlPlane),
			},
		},
		{
			name:    "unsupported-resource",
			allowed: false,
			resources: []runtime.Object{
				newDummyResource("dummy-stdio", "app-namespace",
					schema.GroupVersionKind{Group: "config.istio.io", Version: "v1alpha2", Kind: "stdio"},
					nil,
					nil),
			},
		},
		{
			name:    "service-with-http-ports",
			allowed: true,
			resources: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-service",
						Namespace: "app-namespace",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Name: "http-test",
								Port: 80,
							},
							corev1.ServicePort{
								Name: "http2-test",
								Port: 82,
							},
						},
					},
				},
			},
		},
		{
			name:    "service-with-secure-http-port",
			allowed: false,
			resources: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-service",
						Namespace: "app-namespace",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Name: "http-test",
								Port: 443,
							},
						},
					},
				},
			},
		},
		{
			name:    "service-with-secure-http2-port",
			allowed: false,
			resources: []runtime.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-service",
						Namespace: "app-namespace",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							corev1.ServicePort{
								Name: "http2-test",
								Port: 443,
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			memberRoll := newMemberRoll("default", "istio-system", "app-namespace")
			memberRoll.Status.ConfiguredMembers = append([]string{}, memberRoll.Spec.Members...)
			resources := append(tc.resources,
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "istio-system",
						Labels: map[string]string{
							common.MemberOfKey: "istio-system",
						},
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-namespace",
						Labels: map[string]string{
							common.MemberOfKey: "istio-system",
						},
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "other-namespace",
					},
				},
				memberRoll)
			validator, _, _ := createControlPlaneValidatorTestFixture(resources...)
			response := validator.Handle(ctx, createUpdateRequest(v1_0ControlPlane, v1_1ControlPlane))
			if tc.allowed {
				assert.True(response.Response.Allowed, "Expected validator to accept ServiceMeshControlPlane", t)
			} else {
				assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshControlPlane", t)
				t.Logf("Validation Error: %s", response.Response.Result.Message)
			}
		})
	}
}

func TestVersionDowngrade1_1To1_0(t *testing.T) {
	v1_0ControlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0")
	v1_1ControlPlane := newControlPlaneWithVersion("my-smcp", "istio-system", "v1.1")
	v1_0ControlPlane.SetUID("random-uid")
	v1_1ControlPlane.SetUID("random-uid")
	cases := []struct {
		name            string
		allowed         bool
		namespaceLabels map[string]string
		resources       []runtime.Object
	}{
		{
			name:    "valid",
			allowed: true,
		},
		{
			name:    "unsupported-resource-other-namespace",
			allowed: true,
			resources: []runtime.Object{
				newDummyResource("dummy-auth-policy", "other-namespace",
					schema.GroupVersionKind{Group: "security.istio.io", Version: "v1beta1", Kind: "AuthorizationPolicy"},
					nil,
					nil),
			},
		},
		{
			name:    "unsupported-resource-controller-owned",
			allowed: true,
			resources: []runtime.Object{
				newDummyResource("dummy-auth-policy", "istio-system",
					schema.GroupVersionKind{Group: "security.istio.io", Version: "v1beta1", Kind: "AuthorizationPolicy"},
					nil,
					v1_1ControlPlane),
			},
		},
		{
			name:    "unsupported-resource",
			allowed: false,
			resources: []runtime.Object{
				newDummyResource("dummy-auth-policy", "app-namespace",
					schema.GroupVersionKind{Group: "security.istio.io", Version: "v1beta1", Kind: "AuthorizationPolicy"},
					nil,
					nil),
			},
		},
		{
			name:    "ca.istio.io/env-other-namespace",
			allowed: true,
			resources: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "other-namespace",
						Labels: map[string]string{
							"ca.istio.io/env": "dummy",
						},
					},
				},
			},
		},
		{
			name:    "ca.istio.io/env-app-namespace",
			allowed: false,
			namespaceLabels: map[string]string{
				"ca.istio.io/env": "dummy",
			},
		},
		{
			name:    "ca.istio.io/override-other-namespace",
			allowed: true,
			resources: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "other-namespace",
						Labels: map[string]string{
							"ca.istio.io/override": "dummy",
						},
					},
				},
			},
		},
		{
			name:    "ca.istio.io/override-app-namespace",
			allowed: false,
			namespaceLabels: map[string]string{
				"ca.istio.io/override": "dummy",
			},
		},
		{
			name:    "VirtualService-without-mirrorPercent",
			allowed: true,
			resources: []runtime.Object{
				newDummyResource("dummy-virtual-service", "app-namespace",
					schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1alpha3", Kind: "VirtualService"},
					map[string]interface{}{
						"spec.http": []interface{}{
							map[string]interface{}{
								"name": "some-http",
							},
							map[string]interface{}{
								"name": "some-other-http",
							},
						},
					},
					nil),
			},
		},
		{
			name:    "VirtualService-with-mirrorPercent-controller-owned",
			allowed: true,
			resources: []runtime.Object{
				newDummyResource("dummy-virtual-service", "app-namespace",
					schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1alpha3", Kind: "VirtualService"},
					map[string]interface{}{
						"spec.http": []interface{}{
							map[string]interface{}{
								"name": "some-http",
							},
							map[string]interface{}{
								"name":          "some-other-http",
								"mirrorPercent": "50%",
							},
						},
					},
					v1_1ControlPlane),
			},
		},
		{
			name:    "VirtualService-with-mirrorPercent",
			allowed: false,
			resources: []runtime.Object{
				newDummyResource("dummy-virtual-service", "app-namespace",
					schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1alpha3", Kind: "VirtualService"},
					map[string]interface{}{
						"spec.http": []interface{}{
							map[string]interface{}{
								"name": "some-http",
							},
							map[string]interface{}{
								"name":          "some-other-http",
								"mirrorPercent": "50%",
							},
						},
					},
					nil),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			memberRoll := newMemberRoll("default", "istio-system", "app-namespace")
			memberRoll.Status.ConfiguredMembers = append([]string{}, memberRoll.Spec.Members...)
			memberNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-namespace",
					Labels: map[string]string{
						common.MemberOfKey: "istio-system",
					},
				},
			}
			for key, value := range tc.namespaceLabels {
				memberNamespace.Labels[key] = value
			}
			resources := append(tc.resources,
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "istio-system",
						Labels: map[string]string{
							common.MemberOfKey: "istio-system",
						},
					},
				},
				memberNamespace,
				memberRoll)
			validator, _, _ := createControlPlaneValidatorTestFixture(resources...)
			response := validator.Handle(ctx, createUpdateRequest(v1_1ControlPlane, v1_0ControlPlane))
			if tc.allowed {
				assert.True(response.Response.Allowed, "Expected validator to accept ServiceMeshControlPlane", t)
			} else {
				assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshControlPlane", t)
				t.Logf("Validation Error: %s", response.Response.Result.Message)
			}
		})
	}
}

func createControlPlaneValidatorTestFixture(clientObjects ...runtime.Object) (*ControlPlaneValidator, client.Client, *test.EnhancedTracker) {
	cl, tracker := test.CreateClient(clientObjects...)
	s := tracker.Scheme
	for _, gvk := range unsupportedNewResourcesV1_0 {
		s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		gvk.Kind = gvk.Kind + "List"
		s.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
	}
	for _, gvk := range unsupportedOldResourcesV1_1 {
		s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		gvk.Kind = gvk.Kind + "List"
		s.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
	}
	// Used with v1.1 downgrade check
	gvk := schema.GroupVersionKind{Group: "networking.istio.io", Version: "v1alpha3", Kind: "VirtualService"}
	s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	gvk.Kind = gvk.Kind + "List"
	s.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})

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

	return validator, cl, tracker
}

func newControlPlaneWithVersion(name, namespace, version string) *maistra.ServiceMeshControlPlane {
	controlPlane := newControlPlane(name, namespace)
	controlPlane.Spec.Version = version
	controlPlane.Spec.Istio = make(map[string]interface{})
	return controlPlane
}

func newControlPlane(name, namespace string) *maistra.ServiceMeshControlPlane {
	return &maistra.ServiceMeshControlPlane{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: maistra.ControlPlaneSpec{},
	}
}

func newDummyResource(name string, namespace string, gvk schema.GroupVersionKind, values map[string]interface{}, owner *maistra.ServiceMeshControlPlane) runtime.Object {
	obj := &unstructured.Unstructured{}
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetGroupVersionKind(gvk)
	for path, value := range values {
		unstructured.SetNestedField(obj.UnstructuredContent(), value, strings.Split(path, ".")...)
	}
	if owner != nil {
		ownerRef := metav1.NewControllerRef(owner, maistra.SchemeGroupVersion.WithKind("ServiceMeshControlPlane"))
		obj.SetOwnerReferences([]metav1.OwnerReference{
			*ownerRef,
		})
	}
	return obj
}
