package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/maistra/istio-operator/pkg/apis/external"
	jaegerv1 "github.com/maistra/istio-operator/pkg/apis/external/jaeger/v1"
	kialiv1alpha1 "github.com/maistra/istio-operator/pkg/apis/external/kiali/v1alpha1"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	. "github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/versions"
	routev1 "github.com/openshift/api/route/v1"
)

func TestAddonsInstall(t *testing.T) {
	const (
		operatorNamespace     = "istio-operator"
		controlPlaneNamespace = "test"
		smcpName              = "test"
		cniDaemonSetName      = "istio-node"
		domain                = "test.com"
		kialiName             = "kiali"
		kialiExistingName     = "kiali-existing"
		jaegerName            = "jaeger"
		jaegerExistingName    = "jaeger-existing"
	)

	enable := true

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

	type validation struct {
		Assertions []ActionAssertion
		Verifier   ActionVerifier
	}

	testCases := []struct {
		name       string
		jaegerName string
		kialiName  string
		resources  []runtime.Object
		create     validation
		delete     validation
	}{
		{
			name:      "kiali.install.jaeger.install",
			kialiName: kialiName,
			create: validation{
				Assertions: ActionAssertions{
					Assert("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsSeen(),
					Assert("create").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsSeen(),
				},
			},
			delete: validation{
				Assertions: ActionAssertions{
					Assert("delete").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsSeen(),
					Assert("delete").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsSeen(),
				},
			},
		},
		{
			name: "kiali.install.jaeger.existing",
			resources: []runtime.Object{
				jaegerExisting,
				jaegerRoute,
			},
			jaegerName: jaegerExistingName,
			create: validation{
				Verifier: ActionVerifier(
					Verify("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).Passes(ExpectedKialiCreate(jaegerExistingName, domain)),
				),
				Assertions: ActionAssertions{
					Assert("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsSeen(),
					Assert("create").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("jaegers").Named(jaegerExistingName).In(controlPlaneNamespace).IsNotSeen(),
				},
			},
			delete: validation{
				Assertions: ActionAssertions{
					Assert("delete").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsSeen(),
					Assert("delete").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("jaegers").Named(jaegerExistingName).In(controlPlaneNamespace).IsNotSeen(),
				},
			},
		},
		{
			name:      "kiali.existing.jaeger.install",
			kialiName: kialiName,
			resources: []runtime.Object{
				&kialiv1alpha1.Kiali{Base: external.Base{
					ObjectMeta: metav1.ObjectMeta{Name: kialiName, Namespace: controlPlaneNamespace},
				}},
			},
			create: validation{
				Verifier: ActionVerifier(
					Verify("patch").On("kialis").Named(kialiName).In(controlPlaneNamespace).Passes(ExpectedKialiPatch(jaegerName, domain)),
				),
				Assertions: ActionAssertions{
					Assert("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsSeen(),
				},
			},
			delete: validation{
				Assertions: ActionAssertions{
					Assert("delete").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsSeen(),
				},
			},
		},
		{
			name:      "kiali.existing.jaeger.existing",
			kialiName: kialiExistingName,
			resources: []runtime.Object{
				&kialiv1alpha1.Kiali{Base: external.Base{
					ObjectMeta: metav1.ObjectMeta{Name: kialiExistingName, Namespace: controlPlaneNamespace},
				}},
				jaegerExisting,
				jaegerRoute,
			},
			jaegerName: jaegerExistingName,
			create: validation{
				Verifier: ActionVerifier(
					Verify("patch").On("kialis").Named(kialiExistingName).In(controlPlaneNamespace).Passes(ExpectedKialiPatch(jaegerExistingName, domain)),
				),
				Assertions: ActionAssertions{
					Assert("create").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("kialis").Named(kialiExistingName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("create").On("jaegers").Named(jaegerExistingName).In(controlPlaneNamespace).IsNotSeen(),
				},
			},
			delete: validation{
				Assertions: ActionAssertions{
					Assert("delete").On("kialis").Named(kialiName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("kialis").Named(kialiExistingName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("jaegers").Named(jaegerName).In(controlPlaneNamespace).IsNotSeen(),
					Assert("delete").On("jaegers").Named(jaegerExistingName).In(controlPlaneNamespace).IsNotSeen(),
				},
			},
		},
	}

	for _, tc := range testCases {
		ctc := ControllerTestCase{
			Name:             tc.name,
			ConfigureGlobals: InitializeGlobals(operatorNamespace),
			AddControllers:   []AddControllerFunc{Add},
			Resources: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNamespace}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operatorNamespace}},
			},
			GroupResources: []*restmapper.APIGroupResources{
				CNIGroupResources,
				//MaistraGroupResources,
			},
			StorageVersions: []schema.GroupVersion{maistrav2.SchemeGroupVersion},
			Events: []ControllerTestEvent{
				{
					Name: "create-smcp",
					Execute: func(mgr *FakeManager, _ *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), &maistrav2.ServiceMeshControlPlane{
							ObjectMeta: metav1.ObjectMeta{Name: smcpName, Namespace: controlPlaneNamespace},
							Spec: maistrav2.ControlPlaneSpec{
								Version:  versions.V2_0.String(),
								Profiles: []string{"maistra"},
								Tracing: &maistrav2.TracingConfig{
									Type: maistrav2.TracerTypeJaeger,
								},
								Addons: &maistrav2.AddonsConfig{
									Kiali: &maistrav2.KialiAddonConfig{
										Enablement: maistrav2.Enablement{
											Enabled: &enable,
										},
										Name: tc.kialiName,
									},
									Jaeger: &maistrav2.JaegerAddonConfig{
										Name: tc.jaegerName,
									},
								},
							},
						})
					},
					Verifier:   tc.create.Verifier,
					Assertions: tc.create.Assertions,
					Reactors: []clienttesting.Reactor{
						// make sure deployments come back as ready
						ReactTo("create").On("deployments").In(controlPlaneNamespace).With(SetDeploymentReady),
						// create reasonable default Host value
						ReactTo("create").On("routes").In(controlPlaneNamespace).With(SetRouteHostName(domain)),
						// create jaeger routes and services
						ReactTo("create").On("jaegers").In(controlPlaneNamespace).With(SimulateJaegerInstall(domain, nil)),
					},
					Timeout: 10 * time.Second,
				},
				{
					Name: "delete-smcp",
					Execute: func(mgr *FakeManager, _ *EnhancedTracker) error {
						return mgr.GetClient().Delete(context.TODO(), &maistrav2.ServiceMeshControlPlane{
							ObjectMeta: metav1.ObjectMeta{Name: smcpName, Namespace: controlPlaneNamespace},
						})
					},
					Verifier:   tc.delete.Verifier,
					Assertions: tc.delete.Assertions,
					Timeout:    10 * time.Second,
				},
			},
		}
		if tc.resources != nil {
			ctc.Resources = append(ctc.Resources, tc.resources...)
		}
		t.Run(tc.name, func(t *testing.T) {
			RunControllerTestCase(t, ctc)
		})
	}
}

func ExpectedKialiCreate(jaegerName, domain string) VerifierTestFunc {
	return func(action clienttesting.Action) error {
		createAction := action.(clienttesting.CreateAction)
		obj := createAction.GetObject()
		kiali := obj.(*unstructured.Unstructured)
		if err := VerifyKialiUpdate(jaegerName, domain, maistrav1.NewHelmValues(kiali.Object)); err != nil {
			fmt.Println(fmt.Sprintf("kiali:\n%v", kiali))
			return err
		}
		return nil
	}
}

func ExpectedKialiPatch(jaegerName, domain string) VerifierTestFunc {
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
		if err := VerifyKialiUpdate(jaegerName, domain, patchValues); err != nil {
			fmt.Println(fmt.Sprintf("patch:\n%s", string(patchAction.GetPatch())))
			return err
		}
		return nil
	}
}

func VerifyKialiUpdate(jaegerName, domain string, values *maistrav1.HelmValues) error {
	var allErrors []error
	expectedGrafanaURL := "https://grafana." + domain
	if url, _, _ := values.GetString("spec.external_services.grafana.url"); url != expectedGrafanaURL {
		allErrors = append(allErrors, fmt.Errorf("unexpected grafana URL, expected %s, got %s", expectedGrafanaURL, url))
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
	if _, ok, _ := values.GetString("spec.external_services.prometheus.auth.password"); !ok {
		allErrors = append(allErrors, fmt.Errorf("expected prometheus password to be set"))
	}
	if len(allErrors) > 0 {
		return errors.NewAggregate(allErrors)
	}
	return nil
}
