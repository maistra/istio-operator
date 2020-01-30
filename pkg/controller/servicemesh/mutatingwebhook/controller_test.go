package mutatingwebhook

import (
	"testing"

	"k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

const appNamespace = "app-namespace"

var (
	caBundleValue = []byte("CABundle")

	request = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: webhookConfigNamePrefix + appNamespace,
		},
	}
)

func init() {
	logf.SetLogger(logf.ZapLogger(true))
}

func TestReconcileDoesNothingWhenWebhookConfigMissing(t *testing.T) {
	secret := newSecret(caBundleValue)
	_, tracker, r := createClientAndReconciler(t, secret)
	assertReconcileSucceeds(r, request, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingWhenSecretMissing(t *testing.T) {
	webhookConfig := newMutatingWebhookConfig(caBundleValue)
	_, tracker, r := createClientAndReconciler(t, webhookConfig)
	assertReconcileSucceeds(r, request, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingWhenSecretContainsNoCertificate(t *testing.T) {
	secret := newSecret(caBundleValue)
	delete(secret.Data, common.RootCertKey)
	webhookConfig := newMutatingWebhookConfig(caBundleValue)
	_, tracker, r := createClientAndReconciler(t, webhookConfig, secret)
	assertReconcileSucceeds(r, request, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileDoesNothingWhenCABundleMatches(t *testing.T) {
	webhookConfig := newMutatingWebhookConfig(caBundleValue)
	secret := newSecret(caBundleValue)

	_, tracker, r := createClientAndReconciler(t, webhookConfig, secret)
	assertReconcileSucceeds(r, request, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileUpdatesCABundle(t *testing.T) {
	webhookConfig := newMutatingWebhookConfig([]byte("old-value"))
	secret := newSecret([]byte("new-value"))

	cl, tracker, r := createClientAndReconciler(t, webhookConfig, secret)
	assertReconcileSucceeds(r, request, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)

	updatedConfig := &v1beta1.MutatingWebhookConfiguration{}
	test.GetUpdatedObject(cl, webhookConfig.ObjectMeta, updatedConfig)
	assert.DeepEquals(updatedConfig.Webhooks[0].ClientConfig.CABundle, []byte("new-value"), "Expected Reconcile() to update the CABundle in the webhook configuration", t)
}

func TestReconcileHandlesWebhookConfigsWithoutWebhooks(t *testing.T) {
	webhookConfig := newMutatingWebhookConfig(caBundleValue)
	webhookConfig.Webhooks = nil
	secret := newSecret(caBundleValue)

	_, tracker, r := createClientAndReconciler(t, webhookConfig, secret)
	assertReconcileSucceeds(r, request, t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestReconcileReturnsErrorWhenUpdateFails(t *testing.T) {
	webhookConfig := newMutatingWebhookConfig([]byte("old-value"))
	secret := newSecret([]byte("new-value"))

	_, tracker, r := createClientAndReconciler(t, webhookConfig, secret)
	tracker.AddReactor(test.ClientFailsOn("update", "mutatingwebhookconfigurations"))
	assertReconcileFails(r, t)
}

// TODO: add test to ensure reconcile() is never called for webhook configs that don't start with the correct prefix, as it would panic

func newMutatingWebhookConfig(caBundleValue []byte) *v1beta1.MutatingWebhookConfiguration {
	webhookConfig := &v1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookConfigNamePrefix + appNamespace,
		},
		Webhooks: []v1beta1.Webhook{
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

func newSecret(caBundleValue []byte) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountSecretName,
			Namespace: appNamespace,
		},
		Data: map[string][]byte{
			common.RootCertKey: caBundleValue,
		},
		StringData: nil,
		Type:       "",
	}
	return secret
}

func createClientAndReconciler(t *testing.T, clientObjects ...runtime.Object) (client.Client, *test.EnhancedTracker, *reconciler) {
	cl, enhancedTracker := test.CreateClient(clientObjects...)
	r := newReconciler(cl, scheme.Scheme)
	return cl, enhancedTracker, r
}

func assertReconcileSucceeds(r *reconciler, request reconcile.Request, t *testing.T) {
	res, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if res.Requeue {
		t.Error("Reconcile requeued the request, but it shouldn't have")
	}
}

func assertReconcileFails(r *reconciler, t *testing.T) {
	_, err := r.Reconcile(request)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it didn't")
	}
}
