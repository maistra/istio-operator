package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/maistra/istio-operator/pkg/apis/external"
	jaegerv1 "github.com/maistra/istio-operator/pkg/apis/external/jaeger/v1"
	kialiv1alpha1 "github.com/maistra/istio-operator/pkg/apis/external/kiali/v1alpha1"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	. "github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

var featureEnabled = maistrav2.Enablement{
	Enabled: ptrTrue,
}

var featureDisabled = maistrav2.Enablement{
	Enabled: ptrFalse,
}

type installAddonsTestCase struct {
	name                string
	smcp                *maistrav2.ServiceMeshControlPlane
	resources           []runtime.Object
	create              IntegrationTestValidation
	delete              IntegrationTestValidation
	unsupportedVersions []string
}

func TestAddonsInstall(t *testing.T) {
	const (
		smcpName                     = "test"
		domain                       = "test.com"
		kialiName                    = "kiali"
		kialiExistingName            = "kiali-existing"
		jaegerName                   = "jaeger"
		jaegerExistingName           = "jaeger-existing"
		prometheusPasswordExpected   = true
		prometheusPasswordUnexpected = false
	)

	if testing.Verbose() {
		logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(os.Stderr), zap.Level(zapcore.Level(-5))))
	}

	jaegerRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jaegerExistingName,
			Namespace: controlPlaneNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/instance":  jaegerExistingName,
				"app.kubernetes.io/component": "query-route",
			},
		},
		Spec: routev1.RouteSpec{
			Host: fmt.Sprintf("%s.%s", jaegerExistingName, domain),
		},
	}
	jaegerExisting := &jaegerv1.Jaeger{
		Base: external.Base{
			ObjectMeta: metav1.ObjectMeta{Name: jaegerExistingName, Namespace: controlPlaneNamespace},
		},
	}

	testCases := []installAddonsTestCase{
		{
			name: "kiali.install.jaeger.install",
			smcp: NewSMCPForKialiJaegerTests(smcpName, kialiName, ""),
			create: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsSeen(),
					Assert("create").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsSeen(),
				},
			},
			delete: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("delete").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsSeen(),
					Assert("delete").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsSeen(),
				},
			},
		},
		{
			name: "kiali.install.jaeger.existing",
			smcp: NewSMCPForKialiJaegerTests(smcpName, "", jaegerExistingName),
			resources: []runtime.Object{
				jaegerExisting,
				jaegerRoute,
			},
			create: IntegrationTestValidation{
				Verifier: Verify("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).
					Passes(ExpectedKialiCreate(jaegerExistingName, domain, prometheusPasswordExpected)),
				Assertions: ActionAssertions{
					Assert("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsSeen(),
					Assert("create").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("jaegers").Named(jaegerExistingName).In(controlPlaneNamespace).IsNotSeen(),
				},
			},
			delete: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("delete").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsSeen(),
					Assert("delete").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("jaegers").Named(jaegerExistingName).In(controlPlaneNamespace).IsNotSeen(),
				},
			},
		},
		{
			name: "kiali.existing.jaeger.install",
			smcp: NewSMCPForKialiJaegerTests(smcpName, kialiName, ""),
			resources: []runtime.Object{
				&kialiv1alpha1.Kiali{Base: external.Base{
					ObjectMeta: metav1.ObjectMeta{Name: kialiName, Namespace: controlPlaneNamespace},
				}},
			},
			create: IntegrationTestValidation{
				Verifier: Verify("patch").On("kialis").Named(kialiName).In(controlPlaneNamespace).
					Passes(ExpectedKialiPatch(jaegerName, domain, true)),
				Assertions: ActionAssertions{
					Assert("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsSeen(),
				},
			},
			delete: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("delete").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsSeen(),
				},
			},
		},
		{
			name: "kiali.existing.jaeger.existing",
			smcp: NewSMCPForKialiJaegerTests(smcpName, kialiExistingName, jaegerExistingName),
			resources: []runtime.Object{
				&kialiv1alpha1.Kiali{Base: external.Base{
					ObjectMeta: metav1.ObjectMeta{Name: kialiExistingName, Namespace: controlPlaneNamespace},
				}},
				jaegerExisting,
				jaegerRoute,
			},
			create: IntegrationTestValidation{
				Verifier: Verify("patch").On("kialis").Named(kialiExistingName).In(controlPlaneNamespace).
					Passes(ExpectedKialiPatch(jaegerExistingName, domain, prometheusPasswordExpected)),
				Assertions: ActionAssertions{
					Assert("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("kialis").Named(kialiExistingName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("jaegers").Named(jaegerExistingName).In(controlPlaneNamespace).IsNotSeen(),
				},
			},
			delete: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("delete").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("kialis").Named(kialiExistingName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("jaegers").Named(jaegerExistingName).In(controlPlaneNamespace).IsNotSeen(),
				},
			},
		},
		{
			name: "kiali.existing.jaeger.existing.prometheus.disabled",
			smcp: newSMCPForKialiJaegerPrometheusDisabledTest(smcpName, kialiName, jaegerExistingName),
			resources: []runtime.Object{
				&kialiv1alpha1.Kiali{Base: external.Base{
					ObjectMeta: metav1.ObjectMeta{Name: kialiName, Namespace: controlPlaneNamespace},
				}},
			},
			create: IntegrationTestValidation{
				Verifier: Verify("patch").On("kialis").Named(kialiName).In(controlPlaneNamespace).
					Passes(ExpectedKialiPatch(jaegerExistingName, domain, prometheusPasswordUnexpected)),
				Assertions: ActionAssertions{
					Assert("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsNotSeen(),
				},
			},
			delete: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("delete").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsNotSeen(),
				},
			},
			unsupportedVersions: []string{versions.V2_0.String(), versions.V2_1.String(), versions.V2_2.String(), versions.V2_3.String()},
		},
		{
			name: "addons.ingress.hosts",
			smcp: NewSMCPForPrometheusGrafanaTests(
				controlPlaneName,
				[]string{
					"example1.com",
					"example2.com",
				},
				[]string{
					"example3.com",
					"example4.com",
				},
			),
			create: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("create").On("routes").Named("prometheus").In(controlPlaneNamespace).IsSeen(),
					Assert("create").On("routes").Named("prometheus-1").In(controlPlaneNamespace).IsSeen(),
					Assert("create").On("routes").Named("grafana").In(controlPlaneNamespace).IsSeen(),
					Assert("create").On("routes").Named("grafana-1").In(controlPlaneNamespace).IsSeen(),
				},
			},
			delete: IntegrationTestValidation{
				Assertions: ActionAssertions{
					Assert("delete").On("routes").Named("prometheus").In(controlPlaneNamespace).IsSeen(),
					Assert("delete").On("routes").Named("prometheus-1").In(controlPlaneNamespace).IsSeen(),
					Assert("delete").On("routes").Named("grafana").In(controlPlaneNamespace).IsSeen(),
					Assert("delete").On("routes").Named("grafana-1").In(controlPlaneNamespace).IsSeen(),
				},
			},
			unsupportedVersions: []string{versions.V2_0.String(), versions.V2_1.String(), versions.V2_2.String()},
		},
	}

	for _, tc := range testCases {
		supportedVersions := sets.NewString(allV2Versions()...).Delete(tc.unsupportedVersions...)
		for _, v := range supportedVersions.List() {
			tc.smcp.Spec.Version = v
			RunSimpleInstallTest(t, IntegrationTestCase{
				name:      fmt.Sprintf("%s.%s", tc.name, v),
				smcp:      tc.smcp,
				resources: tc.resources,
				create:    tc.create,
				delete:    tc.delete,
			})
		}
	}
}

func allV2Versions() []string {
	var result []string
	for _, v := range versions.TestedVersions {
		result = append(result, v.String())
	}
	return result
}

func NewSMCPForKialiJaegerTests(smcpName, kialiName, jaegerName string) *maistrav2.ServiceMeshControlPlane {
	return NewV2SMCPResource(smcpName, controlPlaneNamespace, &maistrav2.ControlPlaneSpec{
		Tracing: &maistrav2.TracingConfig{
			Type: maistrav2.TracerTypeJaeger,
		},
		Addons: &maistrav2.AddonsConfig{
			Grafana: &maistrav2.GrafanaAddonConfig{
				Enablement: featureEnabled,
			},
			Kiali: &maistrav2.KialiAddonConfig{
				Enablement: featureEnabled,
				Name:       kialiName,
			},
			Jaeger: &maistrav2.JaegerAddonConfig{
				Name: jaegerName,
			},
			Prometheus: &maistrav2.PrometheusAddonConfig{
				Enablement: featureEnabled,
			},
		},
	})
}

func newSMCPForKialiJaegerPrometheusDisabledTest(smcpName, kialiName, jaegerName string) *maistrav2.ServiceMeshControlPlane {
	smcp := NewSMCPForKialiJaegerTests(smcpName, kialiName, jaegerName)
	smcp.Spec.Addons.Prometheus = nil
	return smcp
}

func NewSMCPForPrometheusGrafanaTests(smcpName string, grafanaHosts, prometheusHosts []string) *maistrav2.ServiceMeshControlPlane {
	return NewV2SMCPResource(smcpName, controlPlaneNamespace, &maistrav2.ControlPlaneSpec{
		Addons: &maistrav2.AddonsConfig{
			Kiali: &maistrav2.KialiAddonConfig{
				Enablement: featureDisabled,
			},
			Grafana: &maistrav2.GrafanaAddonConfig{
				Enablement: featureEnabled,
				Install: &maistrav2.GrafanaInstallConfig{
					Service: &maistrav2.ComponentServiceConfig{
						Ingress: &maistrav2.ComponentIngressConfig{
							Enablement: featureEnabled,
							Hosts:      grafanaHosts,
						},
					},
				},
			},
			Prometheus: &maistrav2.PrometheusAddonConfig{
				Enablement: featureEnabled,
				Install: &maistrav2.PrometheusInstallConfig{
					Service: &maistrav2.ComponentServiceConfig{
						Ingress: &maistrav2.ComponentIngressConfig{
							Enablement: featureEnabled,
							Hosts:      prometheusHosts,
						},
					},
				},
			},
		},
	})
}

func ExpectedKialiCreate(jaegerName, domain string, foundPrometheusPassword bool) VerifierTestFunc {
	return func(action clienttesting.Action) error {
		createAction := action.(clienttesting.CreateAction)
		obj := createAction.GetObject()
		kiali := obj.(*unstructured.Unstructured)
		if err := VerifyKialiUpdate(jaegerName, domain, foundPrometheusPassword, maistrav1.NewHelmValues(kiali.Object)); err != nil {
			fmt.Printf("kiali:\n%v\n", kiali)
			return err
		}
		return nil
	}
}

func ExpectedKialiPatch(jaegerName, domain string, foundPrometheusPassword bool) VerifierTestFunc {
	return func(action clienttesting.Action) error {
		patchAction := action.(clienttesting.PatchAction)
		if patchAction.GetPatchType() != types.MergePatchType {
			return fmt.Errorf("unexpected patch type: %s", patchAction.GetPatchType())
		}
		patch := map[string]interface{}{}
		if err := json.Unmarshal(patchAction.GetPatch(), &patch); err != nil {
			return err
		}
		patchValues := maistrav1.NewHelmValues(patch)
		if err := VerifyKialiUpdate(jaegerName, domain, foundPrometheusPassword, patchValues); err != nil {
			fmt.Printf("patch:\n%s\n", string(patchAction.GetPatch()))
			return err
		}
		return nil
	}
}

func VerifyKialiUpdate(jaegerName, domain string, foundPrometheusPassword bool, values *maistrav1.HelmValues) error {
	var allErrors []error
	expectedGrafanaURL := "https://grafana." + domain
	if url, _, _ := values.GetString("spec.external_services.grafana.url"); url != expectedGrafanaURL {
		allErrors = append(allErrors, fmt.Errorf("unexpected grafana URL, expected %s, got '%s'", expectedGrafanaURL, url))
	}
	if enabled, _, _ := values.GetBool("spec.external_services.grafana.enabled"); !enabled {
		allErrors = append(allErrors, fmt.Errorf("expected grafana to be enabled"))
	}
	if _, ok, _ := values.GetString("spec.external_services.grafana.auth.password"); !ok {
		allErrors = append(allErrors, fmt.Errorf("expected grafana password to be set"))
	}
	expectedJaegerURL := fmt.Sprintf("http://%s.%s", jaegerName, domain)
	if url, _, _ := values.GetString("spec.external_services.tracing.url"); url != expectedJaegerURL {
		allErrors = append(allErrors, fmt.Errorf("unexpected jaeger URL, expected %s, got %s", expectedJaegerURL, url))
	}
	if enabled, _, _ := values.GetBool("spec.external_services.tracing.enabled"); !enabled {
		allErrors = append(allErrors, fmt.Errorf("expected jaeger to be enabled"))
	}
	if _, ok, _ := values.GetString("spec.external_services.tracing.auth.password"); !ok {
		allErrors = append(allErrors, fmt.Errorf("expected jaeger password to be set"))
	}
	if _, ok, _ := values.GetString("spec.external_services.prometheus.auth.password"); ok != foundPrometheusPassword {
		if foundPrometheusPassword {
			allErrors = append(allErrors, fmt.Errorf("expected prometheus password to be set"))
		} else {
			allErrors = append(allErrors, fmt.Errorf("expected prometheus password to not be set"))
		}
	}
	if len(allErrors) > 0 {
		return errors.NewAggregate(allErrors)
	}
	return nil
}

func TestPatchAddonsResult(t *testing.T) {
	requeueWithTimeout := reconcile.Result{RequeueAfter: patchKialiRequeueInterval}
	kiali := newKiali()
	htpasswd := newHtpasswd()
	grafanaRoute := newGrafanaRoute("grafana.istio-system.svc.cluster.local")
	jaegerRoute := newJaegerRoute("jaeger-query.istio-system.svc.cluster.local")

	testCases := []struct {
		name                         string
		kialiEnabled                 bool
		grafanaEnabled               bool
		jaegerEnabled                bool
		objects                      []runtime.Object
		expectedReconciliationResult reconcile.Result
	}{
		{
			name:                         "reconciliation should succeed when Kiali is disabled",
			kialiEnabled:                 false,
			grafanaEnabled:               true,
			jaegerEnabled:                true,
			objects:                      []runtime.Object{},
			expectedReconciliationResult: reconcile.Result{},
		},
		{
			name:           "reconciliation should succeed when jaeger and grafana are enabled and their routes exist",
			kialiEnabled:   true,
			grafanaEnabled: true,
			jaegerEnabled:  true,
			objects: []runtime.Object{
				kiali,
				htpasswd,
				grafanaRoute,
				jaegerRoute,
			},
			expectedReconciliationResult: reconcile.Result{},
		},
		{
			name:           "reconciliation should succeed when jaeger is disabled and its route does not exist",
			kialiEnabled:   true,
			grafanaEnabled: true,
			jaegerEnabled:  false,
			objects: []runtime.Object{
				kiali,
				htpasswd,
				grafanaRoute,
			},
			expectedReconciliationResult: reconcile.Result{},
		},
		{
			name:           "reconciliation should succeed when grafana is disabled and its route does not exist",
			kialiEnabled:   true,
			grafanaEnabled: false,
			jaegerEnabled:  true,
			objects: []runtime.Object{
				kiali,
				htpasswd,
				jaegerRoute,
			},
			expectedReconciliationResult: reconcile.Result{},
		},
		{
			name:           "should requeue reconciliation with timeout when jaeger and grafana are enabled, but their routes do not exist",
			kialiEnabled:   true,
			grafanaEnabled: true,
			jaegerEnabled:  true,
			objects: []runtime.Object{
				kiali,
				htpasswd,
			},
			expectedReconciliationResult: requeueWithTimeout,
		},
		{
			name:           "should requeue reconciliation with timeout when jaeger and grafana are enabled, but jaeger route does not exist",
			kialiEnabled:   true,
			grafanaEnabled: true,
			jaegerEnabled:  true,
			objects: []runtime.Object{
				kiali,
				htpasswd,
				grafanaRoute,
			},
			expectedReconciliationResult: requeueWithTimeout,
		},
		{
			name:           "should requeue reconciliation with timeout when jaeger and grafana are enabled, but grafana route does not exist",
			kialiEnabled:   true,
			grafanaEnabled: true,
			jaegerEnabled:  true,
			objects: []runtime.Object{
				kiali,
				htpasswd,
				jaegerRoute,
			},
			expectedReconciliationResult: requeueWithTimeout,
		},
		{
			name:                         "should requeue reconciliation with timeout when Kiali is enabled, but does not exist",
			kialiEnabled:                 true,
			grafanaEnabled:               false,
			jaegerEnabled:                false,
			objects:                      []runtime.Object{},
			expectedReconciliationResult: requeueWithTimeout,
		},
	}

	for _, v := range allV2Versions() {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s.%s", tc.name, v), func(t *testing.T) {
				smcpSpec := newSmcpSpec(tc.kialiEnabled, tc.grafanaEnabled, tc.jaegerEnabled)
				smcp := NewV2xSMCPResource("basic", "istio-system", smcpSpec, v)
				smcp.Status = maistrav2.ControlPlaneStatus{AppliedSpec: *smcpSpec}

				s := scheme.Scheme
				configureKialiAPI(s)
				configureRouteAPI(s)

				c, tracker := CreateClientWithScheme(s, tc.objects...)
				dc := fake.FakeDiscovery{Fake: &tracker.Fake, FakedServerVersion: DefaultKubeVersion}
				r := newReconciler(c, s, &record.FakeRecorder{}, "istio-operator", cni.Config{Enabled: true}, &dc)
				r.instanceReconcilerFactory = NewControlPlaneInstanceReconciler

				_, smcpReconciler := r.getOrCreateReconciler(smcp)
				res, err := smcpReconciler.PatchAddons(context.TODO(), &smcp.Spec)

				assert.Nil(err, "unexpected error occurred", t)
				assert.Equals(res, tc.expectedReconciliationResult, "unexpected reconciliation result", t)
			})
		}
	}
}

func newSmcpSpec(kialiEnabled, grafanaEnabled, jaegerEnabled bool) *maistrav2.ControlPlaneSpec {
	spec := &maistrav2.ControlPlaneSpec{
		Addons: &maistrav2.AddonsConfig{},
	}

	if kialiEnabled {
		spec.Addons.Kiali = &maistrav2.KialiAddonConfig{
			Enablement: featureEnabled,
		}
	}
	if grafanaEnabled {
		spec.Addons.Grafana = &maistrav2.GrafanaAddonConfig{
			Enablement: featureEnabled,
		}
	}
	if jaegerEnabled {
		spec.Tracing = &maistrav2.TracingConfig{
			Type: maistrav2.TracerTypeJaeger,
		}
	}

	return spec
}

func newHtpasswd() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "htpasswd",
			Namespace: "istio-system",
		},
		Data: map[string][]byte{
			"rawPasswd": []byte("123"),
		},
	}
}

func newKiali() *kialiv1alpha1.Kiali {
	return &kialiv1alpha1.Kiali{
		Base: external.Base{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kiali",
				Namespace: "istio-system",
			},
		},
	}
}

func newGrafanaRoute(hostname string) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "grafana",
			Namespace: "istio-system",
		},
		Spec: routev1.RouteSpec{
			Host: hostname,
		},
	}
}

func newJaegerRoute(hostname string) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jaeger",
			Namespace: "istio-system",
			Labels: map[string]string{
				"app.kubernetes.io/instance":  "jaeger",
				"app.kubernetes.io/component": "query-route",
			},
		},
		Spec: routev1.RouteSpec{
			Host: hostname,
		},
	}
}

func configureKialiAPI(s *runtime.Scheme) {
	kialiGroupVersion := schema.GroupVersion{
		Group:   "kiali.io",
		Version: "v1alpha1",
	}
	s.AddKnownTypes(kialiGroupVersion, &kialiv1alpha1.Kiali{})
}

func configureRouteAPI(s *runtime.Scheme) {
	routeGroupVersion := schema.GroupVersion{
		Group:   "route.openshift.io",
		Version: "v1",
	}
	s.AddKnownTypes(routeGroupVersion, &routev1.Route{}, &routev1.RouteList{})
}
