package webhookca

import (
	"context"
	"testing"

	"k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
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

	invalidRequest = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: sidecarInjectorWebhookNamePrefix + appNamespace,
		},
	}
)

func cases() []struct {
	name        string
	webhook     runtime.Object
	webhookName string
	secret      *corev1.Secret
	request     reconcile.Request
	getter      webhookGetter
} {
	return []struct {
		name        string
		webhook     runtime.Object
		webhookName string
		secret      *corev1.Secret
		request     reconcile.Request
		getter      webhookGetter
	}{
		{
			name:        "sidecar-injector-webhook",
			webhook:     newMutatingWebhookConfig(caBundleValue),
			webhookName: sidecarInjectorWebhookName,
			secret:      newSecret(sidecarInjectorSecretName, caBundleValue),
			request:     sidecarRequest,
			getter:      mutatingWebhook,
		},
		{
			name:        "galley-webhook",
			webhook:     newValidatingWebhookConfig(caBundleValue),
			webhookName: galleyWebhookName,
			secret:      newSecret(galleySecretName, caBundleValue),
			request:     galleyRequest,
			getter:      validatingWebhook,
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
				types.NamespacedName{Namespace: tc.secret.GetNamespace(), Name: tc.secret.GetName()}, common.IstioRootCertKey)
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
				types.NamespacedName{Namespace: tc.secret.GetNamespace(), Name: tc.secret.GetName()}, common.IstioRootCertKey)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileDoesNothingWhenSecretContainsNoCertificate(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			delete(tc.secret.Data, common.IstioRootCertKey)
			_, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				types.NamespacedName{Namespace: tc.secret.GetNamespace(), Name: tc.secret.GetName()}, common.IstioRootCertKey)
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
				types.NamespacedName{Namespace: tc.secret.GetNamespace(), Name: tc.secret.GetName()}, common.IstioRootCertKey)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}

}

func TestReconcileUpdatesCABundle(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			tc.secret.Data = map[string][]byte{
				common.IstioRootCertKey: []byte("new-value"),
			}
			cl, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				types.NamespacedName{Namespace: tc.secret.GetNamespace(), Name: tc.secret.GetName()}, common.IstioRootCertKey)
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
				common.IstioRootCertKey: []byte("new-value"),
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
			tc.secret.Data = map[string][]byte{
				common.IstioRootCertKey: []byte("new-value"),
			}
			cl, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)

			accessor, _ := meta.Accessor(tc.webhook)
			watchPredicates := webhookWatchPredicates(r.webhookCABundleManager)
			watchPredicates.Create(event.CreateEvent{Meta: accessor, Object: tc.webhook})

			assertReconcileSucceeds(r, tc.request, t)

			test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)
			wrapper, _ := tc.getter.Get(context.TODO(), cl, types.NamespacedName{Name: tc.webhookName})
			assert.DeepEquals(wrapper.ClientConfigs()[0].CABundle, []byte("new-value"), "Expected Reconcile() to update the CABundle in the webhook configuration", t)

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
			case *v1beta1.MutatingWebhookConfiguration:
				wh.Webhooks = nil
			case *v1beta1.ValidatingWebhookConfiguration:
				wh.Webhooks = nil
			}
			_, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				types.NamespacedName{Namespace: tc.secret.GetNamespace(), Name: tc.secret.GetName()}, common.IstioRootCertKey)
			assertReconcileSucceeds(r, tc.request, t)
			test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
		})
	}
}

func TestReconcileReturnsErrorWhenUpdateFails(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc.name, func(t *testing.T) {
			tc.secret.Data = map[string][]byte{
				common.IstioRootCertKey: []byte("new-value"),
			}
			_, tracker, r := createClientAndReconciler(t, tc.webhook, tc.secret)
			r.webhookCABundleManager.ManageWebhookCABundle(
				tc.webhook,
				types.NamespacedName{Namespace: tc.secret.GetNamespace(), Name: tc.secret.GetName()}, common.IstioRootCertKey)
			tracker.AddReactor("update", "mutatingwebhookconfigurations", test.ClientFails())
			tracker.AddReactor("update", "validatingwebhookconfigurations", test.ClientFails())
			assertReconcileFails(r, tc.request, t)
		})
	}
}

// TODO: add test to ensure reconcile() is never called for webhook configs that don't start with the correct prefix, as it would panic

func newMutatingWebhookConfig(caBundleValue []byte) *v1beta1.MutatingWebhookConfiguration {
	webhookConfig := &v1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: sidecarInjectorWebhookName,
		},
		Webhooks: []v1beta1.MutatingWebhook{
			{
				Name: "webhhook",
				ClientConfig: v1beta1.WebhookClientConfig{
					CABundle: caBundleValue,
				},
			},
		},
	}
	return webhookConfig
}

func newValidatingWebhookConfig(caBundleValue []byte) *v1beta1.ValidatingWebhookConfiguration {
	webhookConfig := &v1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: galleyWebhookName,
		},
		Webhooks: []v1beta1.ValidatingWebhook{
			{
				Name: "webhhook",
				ClientConfig: v1beta1.WebhookClientConfig{
					CABundle: caBundleValue,
				},
			},
		},
	}
	return webhookConfig
}

func newSecret(secretName string, caBundleValue []byte) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: appNamespace,
		},
		Data: map[string][]byte{
			common.IstioRootCertKey: caBundleValue,
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
