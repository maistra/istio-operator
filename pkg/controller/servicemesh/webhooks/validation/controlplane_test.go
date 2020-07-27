package validation

import (
	"fmt"
	"os"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	webhookadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"

	"github.com/maistra/istio-operator/pkg/apis/istio/simple"
	configv1alpha2 "github.com/maistra/istio-operator/pkg/apis/istio/simple/config/v1alpha2"
	networkingv1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/simple/networking/v1alpha3"
	securityv1beta1 "github.com/maistra/istio-operator/pkg/apis/istio/simple/security/v1beta1"
	"github.com/maistra/istio-operator/pkg/apis/maistra"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
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

func TestControlPlaneNotAllowedInOperatorNamespace(t *testing.T) {
	test.PanicOnError(os.Setenv("POD_NAMESPACE", "openshift-operators")) // TODO: make it easier to set the namespace in tests
	controlPlane := newControlPlane("my-smcp", "openshift-operators")
	validator, _, _ := createControlPlaneValidatorTestFixture()
	response := validator.Handle(ctx, createCreateRequest(controlPlane))
	assert.False(response.Response.Allowed, "Expected validator to reject ServiceMeshControlPlane in operator's namespace", t)
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
		controlPlane *maistrav1.ServiceMeshControlPlane
		resources    []runtime.Object
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
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
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
			resources: []runtime.Object{
				&apiextensionsv1beta1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "jaegers.jaegertracing.io",
					},
				},
			},
			valid: false,
		},
		{
			name: "jaeger-external-uri-wrong-namespace",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
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
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
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
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
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
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
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
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
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
			resources: []runtime.Object{
				&apiextensionsv1beta1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "jaegers.jaegertracing.io",
					},
				},
			},
			valid: false,
		},
		{
			name: "zipkin-address-but-no-jaegerInClusterURL-v1.0",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
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
			resources: []runtime.Object{
				&apiextensionsv1beta1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kialis.kiali.io",
					},
				},
			},
			valid: true,
		},
		{
			name: "zipkin-address-but-no-jaegerInClusterURL-v1.1",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
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
			resources: []runtime.Object{
				&apiextensionsv1beta1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kialis.kiali.io",
					},
				},
			},
			valid: false,
		},
		{
			name: "zipkin-address-with-jaegerInClusterURL-v1.0",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
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
			resources: []runtime.Object{
				&apiextensionsv1beta1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kialis.kiali.io",
					},
				},
			},
			valid: true,
		},
		{
			name: "zipkin-address-with-jaegerInClusterURL-v1.1",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
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
			resources: []runtime.Object{
				&apiextensionsv1beta1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kialis.kiali.io",
					},
				},
			},
			valid: true,
		},
		{
			name: "tracing-with-jaeger",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"tracing": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			},
			resources: []runtime.Object{
				&apiextensionsv1beta1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "jaegers.jaegertracing.io",
					},
				},
			},
			valid: true,
		},
		{
			name: "tracing-with-no-jaeger",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"tracing": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "kiali-with-kiali",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"kiali": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			},
			resources: []runtime.Object{
				&apiextensionsv1beta1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kialis.kiali.io",
					},
				},
			},
			valid: true,
		},
		{
			name: "kiali-with-no-kiali",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"kiali": map[string]interface{}{
							"enabled": true,
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "gateway-outside-mesh",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"gateways": map[string]interface{}{
							"istio-ingressgateway": map[string]interface{}{
								"namespace": "outside",
							},
							"istio-egressgateway": map[string]interface{}{
								"namespace": "inside",
							},
						},
					},
				},
			},
			resources: []runtime.Object{
				&maistrav1.ServiceMeshMemberRoll{
					ObjectMeta: meta.ObjectMeta{
						Name:      "default",
						Namespace: "istio-system",
					},
					Spec: maistrav1.ServiceMeshMemberRollSpec{
						Members: []string{
							"inside",
						},
					},
					Status: maistrav1.ServiceMeshMemberRollStatus{
						ConfiguredMembers: []string{
							"inside",
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "gateway-inside-mesh",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"gateways": map[string]interface{}{
							"istio-ingressgateway": map[string]interface{}{
								"namespace": "inside",
							},
							"istio-egressgateway": map[string]interface{}{
								"namespace": "inside",
							},
						},
					},
				},
			},
			resources: []runtime.Object{
				&maistrav1.ServiceMeshMemberRoll{
					ObjectMeta: meta.ObjectMeta{
						Name:      "default",
						Namespace: "istio-system",
					},
					Spec: maistrav1.ServiceMeshMemberRollSpec{
						Members: []string{
							"inside",
						},
					},
					Status: maistrav1.ServiceMeshMemberRollStatus{
						ConfiguredMembers: []string{
							"inside",
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "cipher-suite-missing-http2",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tls": map[string]interface{}{
								"cipherSuite": "TLS_DHE_RSA_WITH_AES_128_CBC_SHA, TLS_DHE_RSA_WITH_AES_256_CBC_SHA",
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "cipher-suite-including-http2",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tls": map[string]interface{}{
								"cipherSuite": "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, TLS_DHE_RSA_WITH_AES_128_CBC_SHA, TLS_DHE_RSA_WITH_AES_256_CBC_SHA",
							},
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "cipher-suite-unrecognised",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tls": map[string]interface{}{
								"cipherSuite": "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, TLS_UNRECOGNISED",
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "cipher-suite-good-after-bad",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tls": map[string]interface{}{
								"cipherSuite": "TLS_DHE_RSA_WITH_AES_128_CBC_SHA, TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "ecdh-curves",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tls": map[string]interface{}{
								"ecdhCurves": "CurveP256, CurveP384, CurveP521, X25519",
							},
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "ecdh-curves-unrecognised",
			controlPlane: &maistrav1.ServiceMeshControlPlane{
				ObjectMeta: meta.ObjectMeta{
					Name:      "some-smcp",
					Namespace: "istio-system",
				},
				Spec: maistrav1.ControlPlaneSpec{
					Version: maistra.V1_1.String(),
					Istio: map[string]interface{}{
						"global": map[string]interface{}{
							"tls": map[string]interface{}{
								"ecdhCurves": "CurveP256, CurveP384, CurveP521, X25519, UNRECOGNISED",
							},
						},
					},
				},
			},
			valid: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validator, _, _ := createControlPlaneValidatorTestFixture(tc.resources...)
			response := validator.Handle(ctx, createCreateRequest(tc.controlPlane))
			if tc.valid {
				var reason string
				if response.Response.Result != nil {
					reason = response.Response.Result.Message
				}
				assert.True(response.Response.Allowed, "Expected validator to accept valid ServiceMeshControlPlane, but rejected: "+reason, t)
			} else {
				assert.False(response.Response.Allowed, "Expected validator to reject invalid ServiceMeshControlPlane", t)
				t.Logf("Validation Error: %s", response.Response.Result.Message)
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
		smcp      *maistrav1.ServiceMeshControlPlane
		configure func(smcp *maistrav1.ServiceMeshControlPlane)
		allowed   bool
	}

	cases := []struct {
		name  string
		cases []subcase
	}{
		{
			name: "v1.0",
			// all these tests should be allowed, as we only perform 1.0
			// validation when downgrading
			cases: []subcase{
				{
					name:      "valid",
					smcp:      newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {},
					allowed:   true,
				},
				{
					name: "global.proxy.alwaysInjectSelector=false",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
						setNestedField(smcp.Spec.Istio, "global.proxy.alwaysInjectSelector", false)
					},
					allowed: true,
				},
				{
					name: "global.proxy.alwaysInjectSelector=true",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
						setNestedField(smcp.Spec.Istio, "global.proxy.alwaysInjectSelector", true)
					},
					allowed: true,
				},
				{
					name: "global.proxy.neverInjectSelector=false",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
						setNestedField(smcp.Spec.Istio, "global.proxy.neverInjectSelector", false)
					},
					allowed: true,
				},
				{
					name: "global.proxy.neverInjectSelector=true",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
						setNestedField(smcp.Spec.Istio, "global.proxy.neverInjectSelector", true)
					},
					allowed: true,
				},
				{
					name: "global.proxy.envoyAccessLogService.enabled=false",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
						setNestedField(smcp.Spec.Istio, "global.proxy.envoyAccessLogService.enabled", false)
					},
					allowed: true,
				},
				{
					name: "global.proxy.envoyAccessLogService.enabled=true",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
						setNestedField(smcp.Spec.Istio, "global.proxy.envoyAccessLogService.enabled", true)
					},
					allowed: true,
				},
				{
					name: "telemetry.enabled=false, telemetry.v2.enabled=false",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
						setNestedField(smcp.Spec.Istio, "telemetry.enabled", false)
						setNestedField(smcp.Spec.Istio, "telemetry.v2.enabled", false)
					},
					allowed: true,
				},
				{
					name: "telemetry.enabled=false, telemetry.v2.enabled=true",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
						setNestedField(smcp.Spec.Istio, "telemetry.enabled", false)
						setNestedField(smcp.Spec.Istio, "telemetry.v2.enabled", true)
					},
					allowed: true,
				},
				{
					name: "telemetry.enabled=true, telemetry.v2.enabled=true",
					smcp: newControlPlaneWithVersion("my-smcp", "istio-system", "v1.0"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
						setNestedField(smcp.Spec.Istio, "telemetry.enabled", true)
						setNestedField(smcp.Spec.Istio, "telemetry.v2.enabled", true)
					},
					allowed: true,
				},
			},
		},
		{
			name: "v1.1",
			cases: []subcase{
				{
					name:      "valid",
					smcp:      newControlPlaneWithVersion("my-smcp", "istio-system", "v1.1"),
					configure: func(smcp *maistrav1.ServiceMeshControlPlane) {},
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
						defer func() {
							if t.Failed() {
								t.Logf("Unexpected validation Error: %s", response.Response.Result.Message)
							}
						}()
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
				&configv1alpha2.Stdio{
					Base: simple.Base{
						TypeMeta: metav1.TypeMeta{
							APIVersion: configv1alpha2.SchemeGroupVersion.String(),
							Kind:       "stdio",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-stdio",
							Namespace: "other-namespace",
						},
					},
				},
			},
		},
		{
			name:    "unsupported-resource-controller-owned",
			allowed: true,
			resources: []runtime.Object{
				&configv1alpha2.Stdio{
					Base: simple.Base{
						TypeMeta: metav1.TypeMeta{
							APIVersion: configv1alpha2.SchemeGroupVersion.String(),
							Kind:       "stdio",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-stdio",
							Namespace: "istio-system",
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(v1_0ControlPlane, maistrav1.SchemeGroupVersion.WithKind("ServiceMeshControlPlane")),
							},
						},
					},
				},
			},
		},
		{
			name:    "unsupported-resource",
			allowed: false,
			resources: []runtime.Object{
				&configv1alpha2.Stdio{
					Base: simple.Base{
						TypeMeta: metav1.TypeMeta{
							APIVersion: configv1alpha2.SchemeGroupVersion.String(),
							Kind:       "stdio",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-stdio",
							Namespace: "app-namespace",
						},
					},
				},
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
								Name: "http",
								Port: 81,
							},
							corev1.ServicePort{
								Name: "http2-test",
								Port: 82,
							},
							corev1.ServicePort{
								Name: "http2",
								Port: 84,
							},
						},
					},
				},
			},
		},
		{
			name:    "service-with-secure-http-prefixed-port",
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
								Name: "http",
								Port: 443,
							},
						},
					},
				},
			},
		},
		{
			name:    "service-with-secure-http2-prefixed-port",
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
								Name: "http2",
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
				defer func() {
					if t.Failed() {
						t.Logf("Unexpected validation Error: %s", response.Response.Result.Message)
					}
				}()
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
		configure       func(smcp *maistrav1.ServiceMeshControlPlane)
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
				&securityv1beta1.AuthorizationPolicy{
					Base: simple.Base{
						TypeMeta: metav1.TypeMeta{
							APIVersion: securityv1beta1.SchemeGroupVersion.String(),
							Kind:       "AuthorizationPolicy",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-auth-policy",
							Namespace: "other-namespace",
						},
					},
				},
			},
		},
		{
			name:    "unsupported-resource-controller-owned",
			allowed: true,
			resources: []runtime.Object{
				&securityv1beta1.AuthorizationPolicy{
					Base: simple.Base{
						TypeMeta: metav1.TypeMeta{
							APIVersion: securityv1beta1.SchemeGroupVersion.String(),
							Kind:       "AuthorizationPolicy",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-auth-policy",
							Namespace: "istio-system",
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(v1_1ControlPlane, maistrav1.SchemeGroupVersion.WithKind("ServiceMeshControlPlane")),
							},
						},
					},
				},
			},
		},
		{
			name:    "unsupported-resource",
			allowed: false,
			resources: []runtime.Object{
				&securityv1beta1.AuthorizationPolicy{
					Base: simple.Base{
						TypeMeta: metav1.TypeMeta{
							APIVersion: securityv1beta1.SchemeGroupVersion.String(),
							Kind:       "AuthorizationPolicy",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-auth-policy",
							Namespace: "app-namespace",
						},
					},
				},
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
				&networkingv1alpha3.VirtualService{
					Base: simple.Base{
						TypeMeta: metav1.TypeMeta{
							APIVersion: networkingv1alpha3.SchemeGroupVersion.String(),
							Kind:       "VirtualService",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-virtual-service",
							Namespace: "app-namespace",
						},
						Spec: map[string]interface{}{
							"http": []interface{}{
								map[string]interface{}{
									"name": "some-http",
								},
								map[string]interface{}{
									"name": "some-other-http",
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "VirtualService-with-mirrorPercent-controller-owned",
			allowed: true,
			resources: []runtime.Object{
				&networkingv1alpha3.VirtualService{
					Base: simple.Base{
						TypeMeta: metav1.TypeMeta{
							APIVersion: networkingv1alpha3.SchemeGroupVersion.String(),
							Kind:       "VirtualService",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-virtual-service",
							Namespace: "app-namespace",
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(v1_1ControlPlane, maistrav1.SchemeGroupVersion.WithKind("ServiceMeshControlPlane")),
							},
						},
						Spec: map[string]interface{}{
							"http": []interface{}{
								map[string]interface{}{
									"name": "some-http",
								},
								map[string]interface{}{
									"name":          "some-other-http",
									"mirrorPercent": "50%",
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "VirtualService-with-mirrorPercent",
			allowed: false,
			resources: []runtime.Object{
				&networkingv1alpha3.VirtualService{
					Base: simple.Base{
						TypeMeta: metav1.TypeMeta{
							APIVersion: networkingv1alpha3.SchemeGroupVersion.String(),
							Kind:       "VirtualService",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "dummy-virtual-service",
							Namespace: "app-namespace",
						},
						Spec: map[string]interface{}{
							"http": []interface{}{
								map[string]interface{}{
									"name": "some-http",
								},
								map[string]interface{}{
									"name":          "some-other-http",
									"mirrorPercent": "50%",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "global.proxy.alwaysInjectSelector=false",
			configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
				setNestedField(smcp.Spec.Istio, "global.proxy.alwaysInjectSelector", false)
			},
			allowed: true,
		},
		{
			name: "global.proxy.alwaysInjectSelector=true",
			configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
				setNestedField(smcp.Spec.Istio, "global.proxy.alwaysInjectSelector", true)
			},
			allowed: false,
		},
		{
			name: "global.proxy.neverInjectSelector=false",
			configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
				setNestedField(smcp.Spec.Istio, "global.proxy.neverInjectSelector", false)
			},
			allowed: true,
		},
		{
			name: "global.proxy.neverInjectSelector=true",
			configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
				setNestedField(smcp.Spec.Istio, "global.proxy.neverInjectSelector", true)
			},
			allowed: false,
		},
		{
			name: "global.proxy.envoyAccessLogService.enabled=false",
			configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
				setNestedField(smcp.Spec.Istio, "global.proxy.envoyAccessLogService.enabled", false)
			},
			allowed: true,
		},
		{
			name: "global.proxy.envoyAccessLogService.enabled=true",
			configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
				setNestedField(smcp.Spec.Istio, "global.proxy.envoyAccessLogService.enabled", true)
			},
			allowed: false,
		},
		{
			name: "telemetry.enabled=false, telemetry.v2.enabled=false",
			configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
				setNestedField(smcp.Spec.Istio, "telemetry.enabled", false)
				setNestedField(smcp.Spec.Istio, "telemetry.v2.enabled", false)
			},
			allowed: true,
		},
		{
			name: "telemetry.enabled=false, telemetry.v2.enabled=true",
			configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
				setNestedField(smcp.Spec.Istio, "telemetry.enabled", false)
				setNestedField(smcp.Spec.Istio, "telemetry.v2.enabled", true)
			},
			allowed: true,
		},
		{
			name: "telemetry.enabled=true, telemetry.v2.enabled=true",
			configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
				setNestedField(smcp.Spec.Istio, "telemetry.enabled", true)
				setNestedField(smcp.Spec.Istio, "telemetry.v2.enabled", true)
			},
			allowed: false,
		},
		{
			name: "zipkin-address-with-jaegerInClusterURL-v1.1",
			configure: func(smcp *maistrav1.ServiceMeshControlPlane) {
				setNestedField(smcp.Spec.Istio, "global.tracer.zipkin.address", "jaeger-query.istio-system")
				setNestedField(smcp.Spec.Istio, "kiali.enabled", true)
				setNestedField(smcp.Spec.Istio, "kiali.jaegerInClusterURL", "jaeger-collector.istio-system")
			},
			allowed: false,
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
			newsmcp := v1_0ControlPlane.DeepCopy()
			if tc.configure != nil {
				tc.configure(newsmcp)
			}
			response := validator.Handle(ctx, createUpdateRequest(v1_1ControlPlane, newsmcp))
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

func newControlPlaneWithVersion(name, namespace, version string) *maistrav1.ServiceMeshControlPlane {
	controlPlane := newControlPlane(name, namespace)
	controlPlane.Spec.Version = version
	controlPlane.Spec.Istio = make(map[string]interface{})
	return controlPlane
}

func newControlPlane(name, namespace string) *maistrav1.ServiceMeshControlPlane {
	return &maistrav1.ServiceMeshControlPlane{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: maistrav1.ControlPlaneSpec{},
	}
}

func setNestedField(obj map[string]interface{}, path string, value interface{}) {
	unstructured.SetNestedField(obj, value, strings.Split(path, ".")...)
}
