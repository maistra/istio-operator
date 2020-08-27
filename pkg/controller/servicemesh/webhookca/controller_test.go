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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

var ctx = common.NewContextWithLog(context.Background(), logf.Log)

const (
	appNamespace               = "app-namespace"
	galleyWebhookName          = galleyWebhookNamePrefix + appNamespace
	sidecarInjectorWebhookName = sidecarInjectorWebhookNamePrefix + appNamespace
	istiodWebhookName          = istiodWebhookNamePrefix + "default-" + appNamespace
)

var (
	caBundleValue = []byte("CABundle")

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

	conversionRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: conversionNamespaceValue,
			Name:      ServiceMeshControlPlaneCRDName,
		},
	}

	invalidRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: sidecarInjectorWebhookNamePrefix + appNamespace,
		},
	}
)

func cases() []struct {
	name                 string
	webhook              runtime.Object
	webhookName          string
	secret               *corev1.Secret
	caFileName           string
	request              reconcile.Request
	getter               webhookGetter
	skipAutoRegistration bool
} {
	return []struct {
		name                 string
		webhook              runtime.Object
		webhookName          string
		secret               *corev1.Secret
		caFileName           string
		request              reconcile.Request
		getter               webhookGetter
		skipAutoRegistration bool
	}{
		{
			name:        "sidecar-injector-webhook",
			webhook:     newMutatingWebhookConfig(sidecarInjectorWebhookName, caBundleValue),
			webhookName: sidecarInjectorWebhookName,
			secret:      newSecret(sidecarInjectorSecretName, common.IstioRootCertKey, caBundleValue),
			caFileName:  common.IstioRootCertKey,
			request:     sidecarRequest,
			getter:      mutatingWebhook,
		},
		{
			name:        "galley-webhook",
			webhook:     newValidatingWebhookConfig(galleyWebhookName, caBundleValue),
			webhookName: galleyWebhookName,
			secret:      newSecret(galleySecretName, common.IstioRootCertKey, caBundleValue),
			caFileName:  common.IstioRootCertKey,
			request:     galleyRequest,
			getter:      validatingWebhook,
		},
		{
			name:        "istiod-injector-webhook",
			webhook:     newMutatingWebhookConfig(istiodWebhookName, caBundleValue),
			webhookName: istiodWebhookName,
			secret:      newSecret(istiodSecretName, common.IstiodCertKey, caBundleValue),
			caFileName:  common.IstiodCertKey,
			request:     istiodInjectorRequest,
			getter:      mutatingWebhook,
		},
		{
			name:        "istiod-validating-webhook",
			webhook:     newValidatingWebhookConfig(istiodWebhookName, caBundleValue),
			webhookName: istiodWebhookName,
			secret:      newSecret(istiodSecretName, common.IstiodCertKey, caBundleValue),
			caFileName:  common.IstiodCertKey,
			request:     istiodValidatorRequest,
			getter:      validatingWebhook,
		},
		{
			name:                 "service-mesh-conversion",
			webhook:              newCustomResourceDefinition(ServiceMeshControlPlaneCRDName, caBundleValue),
			webhookName:          ServiceMeshControlPlaneCRDName,
			secret:               newSecret(istiodSecretName, common.IstiodCertKey, caBundleValue),
			caFileName:           common.ServiceCARootCertKey,
			request:              conversionRequest,
			getter:               conversionWebhook,
			skipAutoRegistration: true,
		},
	}
}

func init() {
	logf.SetLogger(logf.ZapLogger(true))
}

func TestReconcileDoesNothingWhenWebhookConfigMissing(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			_, tracker, r := createClientAndReconciler(t, tc.secret)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				common.ToNamespacedName(tc.secret), tc.caFileName)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileDoesNothingWhenSecretMissing(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			_, tracker, r := createClientAndReconciler(t, tc.webhook)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				common.ToNamespacedName(tc.secret), tc.caFileName)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileDoesNothingWhenSecretContainsNoCertificate(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			delete(tc.secret.Data, tc.caFileName)
			_, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				common.ToNamespacedName(tc.secret), tc.caFileName)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileDoesNothingWhenCABundleMatches(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			_, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				common.ToNamespacedName(tc.secret), tc.caFileName)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}

}

func TestReconcileUpdatesCABundle(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			tc.secret.Data = map[string][]byte{
				tc.caFileName: []byte("new-value"),
			}
			cl, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)
			if err := r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				common.ToNamespacedName(tc.secret), tc.caFileName); err != nil {
				t.Fatal(err)
			}
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)

			wrapper, _ := tc.getter.Get(context.TODO(), cl, types.NamespacedName{Name: tc.webhookName})
			assert.DeepEquals(wrapper.ClientConfigs()[0].CABundle, []byte("new-value"), "Expected Reconcile() to update the CABundle in the webhook configuration", t)
		})
	}
}

func TestReconcileUnmanagedWebhookNotUpdated(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			tc.secret.Data = map[string][]byte{
				tc.caFileName: []byte("new-value"),
			}
			cl, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)

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
			tc.secret.Data = map[string][]byte{
				tc.caFileName: []byte("new-value"),
			}
			cl, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)

			accessor, _ := meta.Accessor(tc.webhook)
			watchPredicates := webhookWatchPredicates(r.webhookCABundleManager)
			watchPredicates.Create(event.CreateEvent{Meta: accessor, Object: tc.webhook})

			assertReconcileSucceeds(r, tc.request, t)

			test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)
			wrapper, _ := tc.getter.Get(context.TODO(), cl, types.NamespacedName{Name: tc.webhookName})
			assert.DeepEquals(wrapper.ClientConfigs()[0].CABundle, []byte("new-value"), "Expected Reconcile() to update the CABundle in the webhook configuration", t)

			assert.True(r.webhookCABundleManager.IsManagingWebhooksForSecret(common.ToNamespacedName(tc.secret)), "Expected secret to trigger a webhook reconcile", t)

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
			_, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				common.ToNamespacedName(tc.secret), tc.caFileName)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileDoesNothingWithMultipleNamesacedServices(t *testing.T) {
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
			_, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				common.ToNamespacedName(tc.secret), tc.caFileName)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileReturnsErrorWhenUpdateFails(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			tc.secret.Data = map[string][]byte{
				tc.caFileName: []byte("new-value"),
			}
			_, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				common.ToNamespacedName(tc.secret), tc.caFileName)
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

func newSecret(secretName string, caName string, caBundleValue []byte) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: appNamespace,
		},
		Data: map[string][]byte{
			caName: caBundleValue,
		},
		StringData: nil,
		Type:       "",
	}
	return secret
}

func createClientAndReconciler(t *testing.T, clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, *reconciler) {
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
