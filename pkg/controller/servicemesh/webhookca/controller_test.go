package webhookca

import (
	"context"
	"testing"

	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

const (
	appNamespace               = "app-namespace"
	galleyWebhookName          = galleyWebhookNamePrefix + appNamespace
	sidecarInjectorWebhookName = sidecarInjectorWebhookNamePrefix + appNamespace
	istiodWebhookName          = istiodWebhookNamePrefix + "default-" + appNamespace
	istioOperatorWebhookName   = "istio-operator.servicemesh-resources.maistra.io"
	caBundleConfigMapName      = "maistra-operator-cabundle"
)

var (
	caBundleStringValue = "CABundle"
	caBundleValue       = []byte(caBundleStringValue)

	galleyRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: validatingNamespaceValue,
			Name:      galleyWebhookName,
		},
	}

	sidecarRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: mutatingNamespaceValue,
			Name:      sidecarInjectorWebhookName,
		},
	}

	istiodInjectorRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: mutatingNamespaceValue,
			Name:      istiodWebhookName,
		},
	}

	istiodValidatorRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: validatingNamespaceValue,
			Name:      istiodWebhookName,
		},
	}

	operatorValidatorRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: validatingNamespaceValue,
			Name:      istioOperatorWebhookName,
		},
	}

	conversionRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: conversionNamespaceValue,
			Name:      ServiceMeshControlPlaneCRDName,
		},
	}
)

type testCase struct {
	name                 string
	webhook              runtime.Object
	webhookName          string
	source               CABundleSource
	objectName           string
	dataKey              string
	kind                 string
	request              reconcile.Request
	getter               webhookGetter
	skipAutoRegistration bool
}

func cases() []testCase {
	return []testCase{
		{
			name:        "sidecar-injector-webhook",
			webhook:     newMutatingWebhookConfig(sidecarInjectorWebhookName, caBundleValue),
			webhookName: sidecarInjectorWebhookName,
			source:      autoRegistrationMap[sidecarInjectorWebhookNamePrefix],
			objectName:  sidecarInjectorSecretName,
			dataKey:     common.IstioRootCertKey,
			kind:        "Secret",
			request:     sidecarRequest,
			getter:      mutatingWebhook,
		},
		{
			name:        "galley-webhook",
			webhook:     newValidatingWebhookConfig(galleyWebhookName, caBundleValue),
			webhookName: galleyWebhookName,
			source:      autoRegistrationMap[galleyWebhookNamePrefix],
			objectName:  galleySecretName,
			dataKey:     common.IstioRootCertKey,
			kind:        "Secret",
			request:     galleyRequest,
			getter:      validatingWebhook,
		},
		{
			name:        "istiod-injector-webhook",
			webhook:     newMutatingWebhookConfig(istiodWebhookName, caBundleValue),
			webhookName: istiodWebhookName,
			source:      autoRegistrationMap[istiodWebhookNamePrefix],
			objectName:  istiodSecretName,
			dataKey:     common.IstiodCertKey,
			kind:        "Secret",
			request:     istiodInjectorRequest,
			getter:      mutatingWebhook,
		},
		{
			name:        "istiod-validating-webhook",
			webhook:     newValidatingWebhookConfig(istiodWebhookName, caBundleValue),
			webhookName: istiodWebhookName,
			source:      autoRegistrationMap[istiodWebhookNamePrefix],
			objectName:  istiodSecretName,
			dataKey:     common.IstiodCertKey,
			kind:        "Secret",
			request:     istiodValidatorRequest,
			getter:      validatingWebhook,
		},
		{
			name:        "istiod-validating-webhook-cert-manager",
			webhook:     newValidatingWebhookConfig(istiodWebhookName, caBundleValue),
			webhookName: istiodWebhookName,
			source:      autoRegistrationMap[istiodWebhookNamePrefix],
			objectName:  istiodCertManagerSecretName,
			dataKey:     common.IstiodCertManagerCertKey,
			kind:        "Secret",
			request:     istiodValidatorRequest,
			getter:      validatingWebhook,
		},
		{
			name:        "istio-operator-validating-webhook",
			webhook:     newValidatingWebhookConfig(istioOperatorWebhookName, caBundleValue),
			webhookName: istioOperatorWebhookName,
			source: &ConfigMapCABundleSource{
				ConfigMapName: caBundleConfigMapName,
				Key:           common.ServiceCABundleKey,
			},
			objectName:           caBundleConfigMapName,
			dataKey:              common.ServiceCABundleKey,
			kind:                 "ConfigMap",
			request:              operatorValidatorRequest,
			getter:               validatingWebhook,
			skipAutoRegistration: true,
		},
		{
			name:        "service-mesh-conversion",
			webhook:     newCustomResourceDefinition(ServiceMeshControlPlaneCRDName, caBundleValue),
			webhookName: ServiceMeshControlPlaneCRDName,
			source: &ConfigMapCABundleSource{
				ConfigMapName: caBundleConfigMapName,
				Key:           common.ServiceCABundleKey,
			},
			objectName:           caBundleConfigMapName,
			dataKey:              common.ServiceCABundleKey,
			kind:                 "ConfigMap",
			request:              conversionRequest,
			getter:               conversionWebhook,
			skipAutoRegistration: true,
		},
	}
}

func init() {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))
}

func TestReconcileDoesNothingWhenWebhookConfigMissing(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			object := newObject(tc.kind, tc.objectName, tc.dataKey, caBundleStringValue)
			_, tracker, r := createClientAndReconciler(object)
			r.webhookCABundleManager.ManageWebhookCABundle(tc.webhook, tc.source)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileDoesNothingWhenSecretMissing(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			_, tracker, r := createClientAndReconciler(tc.webhook)
			r.webhookCABundleManager.ManageWebhookCABundle(tc.webhook, tc.source)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileDoesNothingWhenSecretContainsNoCertificate(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			object := newEmptyObject(tc.kind, tc.objectName)
			_, tracker, r := createClientAndReconciler(tc.webhook, object)
			r.webhookCABundleManager.ManageWebhookCABundle(tc.webhook, tc.source)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileDoesNothingWhenCABundleMatches(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			object := newObject(tc.kind, tc.objectName, tc.dataKey, caBundleStringValue)
			_, tracker, r := createClientAndReconciler(tc.webhook, object)
			r.webhookCABundleManager.ManageWebhookCABundle(tc.webhook, tc.source)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileUpdatesCABundle(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			object := newObject(tc.kind, tc.objectName, tc.dataKey, "new-value")
			cl, tracker, r := createClientAndReconciler(tc.webhook, object)
			if err := r.webhookCABundleManager.ManageWebhookCABundle(tc.webhook, tc.source); err != nil {
				t.Fatal(err)
			}
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)

			wrapper, _ := tc.getter.Get(context.TODO(), cl, types.NamespacedName{Name: tc.webhookName})
			assert.DeepEquals(string(wrapper.ClientConfigs()[0].CABundle), "new-value", "Expected Reconcile() to update the CABundle in the webhook configuration", t)
		})
	}
}

func TestReconcileUnmanagedWebhookNotUpdated(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			object := newObject(tc.kind, tc.objectName, tc.dataKey, "random data")
			cl, tracker, r := createClientAndReconciler(tc.webhook, object)

			assertReconcileSucceeds(r, tc.request, t)

			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
			wrapper, _ := tc.getter.Get(context.TODO(), cl, types.NamespacedName{Name: tc.webhookName})
			assert.DeepEquals(wrapper.ClientConfigs()[0].CABundle, caBundleValue, "Expected Reconcile() to update the CABundle in the webhook configuration", t)
		})
	}
}

func TestReconcileAutomaticRegistration(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipAutoRegistration {
				t.SkipNow()
			}
			object := newObject(tc.kind, tc.objectName, tc.dataKey, "new-value")
			cl, tracker, r := createClientAndReconciler(tc.webhook, object)

			accessor, _ := meta.Accessor(tc.webhook)
			watchPredicates := webhookWatchPredicates(r.webhookCABundleManager)
			watchPredicates.Create(event.CreateEvent{Meta: accessor, Object: tc.webhook})

			assertReconcileSucceeds(r, tc.request, t)

			test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)
			wrapper, _ := tc.getter.Get(context.TODO(), cl, types.NamespacedName{Name: tc.webhookName})
			assert.DeepEquals(wrapper.ClientConfigs()[0].CABundle, []byte("new-value"), "Expected Reconcile() to update the CABundle in the webhook configuration", t)

			assert.True(r.webhookCABundleManager.IsManagingWebhooksForSource(ObjectRef{
				Kind:      tc.kind,
				Namespace: appNamespace,
				Name:      tc.objectName,
			}), "Expected source to trigger a webhook reconcile", t)

			watchPredicates.Delete(event.DeleteEvent{Meta: accessor, Object: tc.webhook})
			if r.webhookCABundleManager.IsManaged(tc.webhook) {
				t.Errorf("webhook should no longer be watched after deletion.")
			}
		})
	}
}

func TestReconcileHandlesWebhookConfigsWithoutWebhooks(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			switch wh := tc.webhook.(type) {
			case *v1.MutatingWebhookConfiguration:
				wh.Webhooks = nil
			case *v1.ValidatingWebhookConfiguration:
				wh.Webhooks = nil
			}
			object := newObject(tc.kind, tc.objectName, tc.dataKey, caBundleStringValue)
			_, tracker, r := createClientAndReconciler(tc.webhook, object)
			r.webhookCABundleManager.ManageWebhookCABundle(tc.webhook, tc.source)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileDoesNothingWithMultipleNamespacedServices(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			switch wh := tc.webhook.(type) {
			case *v1.MutatingWebhookConfiguration:
				wh.Webhooks = append(wh.Webhooks, v1.MutatingWebhook{
					Name: "bad-webhook",
					ClientConfig: v1.WebhookClientConfig{
						Service: &v1.ServiceReference{
							Name:      "bogus-service",
							Namespace: appNamespace + "-2",
						},
					},
				})
			case *v1.ValidatingWebhookConfiguration:
				wh.Webhooks = append(wh.Webhooks, v1.ValidatingWebhook{
					Name: "bad-webhook",
					ClientConfig: v1.WebhookClientConfig{
						Service: &v1.ServiceReference{
							Name:      "bogus-service",
							Namespace: appNamespace + "-2",
						},
					},
				})
			}
			object := newObject(tc.kind, tc.objectName, tc.dataKey, caBundleStringValue)
			_, tracker, r := createClientAndReconciler(tc.webhook, object)
			r.webhookCABundleManager.ManageWebhookCABundle(tc.webhook, tc.source)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileReturnsErrorWhenUpdateFails(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			object := newObject(tc.kind, tc.objectName, tc.dataKey, "random data")
			_, tracker, r := createClientAndReconciler(tc.webhook, object)
			r.webhookCABundleManager.ManageWebhookCABundle(tc.webhook, tc.source)
			tracker.AddReactor("update", "mutatingwebhookconfigurations", test.ClientFails())
			tracker.AddReactor("update", "validatingwebhookconfigurations", test.ClientFails())
			tracker.AddReactor("update", "customresourcedefinitions", test.ClientFails())
			assertReconcileFails(r, tc.request, t)
		})
	}
}

// TODO: add test to ensure reconcile() is never called for webhook configs that don't start with the correct prefix, as it would panic

func newMutatingWebhookConfig(name string, caBundleValue []byte) *v1.MutatingWebhookConfiguration {
	webhookConfig := &v1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Webhooks: []v1.MutatingWebhook{
			{
				Name: "webhhook",
				ClientConfig: v1.WebhookClientConfig{
					CABundle: caBundleValue,
					Service: &v1.ServiceReference{
						Name:      "webhook-service",
						Namespace: appNamespace,
					},
				},
			},
		},
	}
	return webhookConfig
}

func newValidatingWebhookConfig(name string, caBundleValue []byte) *v1.ValidatingWebhookConfiguration {
	webhookConfig := &v1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Webhooks: []v1.ValidatingWebhook{
			{
				Name: "webhhook",
				ClientConfig: v1.WebhookClientConfig{
					CABundle: caBundleValue,
					Service: &v1.ServiceReference{
						Name:      "webhook-service",
						Namespace: appNamespace,
					},
				},
			},
		},
	}
	return webhookConfig
}

func newCustomResourceDefinition(name string, caBundleValue []byte) *apixv1.CustomResourceDefinition {
	crd := &apixv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apixv1.CustomResourceDefinitionSpec{
			Conversion: &apixv1.CustomResourceConversion{
				Strategy: apixv1.WebhookConverter,
				Webhook: &apixv1.WebhookConversion{
					ClientConfig: &apixv1.WebhookClientConfig{
						CABundle: caBundleValue,
						Service: &apixv1.ServiceReference{
							Name:      "webhook-service",
							Namespace: appNamespace,
						},
					},
				},
			},
		},
	}
	return crd
}

func newObject(kind, objName, key, data string) runtime.Object {
	obj := newEmptyObject(kind, objName)
	if kind == "ConfigMap" {
		obj.(*corev1.ConfigMap).Data = map[string]string{
			key: data,
		}
	} else {
		obj.(*corev1.Secret).Data = map[string][]byte{
			key: []byte(data),
		}
	}
	return obj
}

func newEmptyObject(kind, objName string) runtime.Object {
	objMeta := metav1.ObjectMeta{
		Name:      objName,
		Namespace: appNamespace,
	}
	if kind == "ConfigMap" {
		return &corev1.ConfigMap{
			ObjectMeta: objMeta,
		}
	}
	return &corev1.Secret{
		ObjectMeta: objMeta,
	}
}

func createClientAndReconciler(clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, *reconciler) {
	cl, enhancedTracker := test.CreateClient(clientObjects...)
	r := newReconciler(cl, scheme.Scheme, newWebhookCABundleManager())
	return cl, enhancedTracker, r
}

func assertReconcileSucceeds(r *reconciler, request reconcile.Request, t *testing.T) {
	t.Helper()
	res, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if res.Requeue {
		t.Error("Reconcile requeued the request, but it shouldn't have")
	}
}

func assertReconcileFails(r *reconciler, request reconcile.Request, t *testing.T) {
	t.Helper()
	_, err := r.Reconcile(request)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}
}
