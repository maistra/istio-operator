package webhookca

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// WebhookCABundleManager is the public interface for managing webhook caBundle.
type WebhookCABundleManager interface {
	// ManageWebhookCABundle adds a webhook to the manager.
	ManageWebhookCABundle(obj runtime.Object, secret types.NamespacedName, keyName string) error
	// UnmanageWebhookCABundle removes a webhook from the manager.
	UnmanageWebhookCABundle(obj runtime.Object) error
	// IsManaged returns true if the webhook is being managed.
	IsManaged(obj runtime.Object) bool
	// IsManagingWebhooksForSecret returns true if any webhooks being managed are using the secret.
	IsManagingWebhooksForSecret(secret types.NamespacedName) bool
	// ReconcileRequestsFromSecret returns a slice of reconcile.Request objects for the specified secret
	ReconcileRequestsFromSecret(secret types.NamespacedName) []reconcile.Request
	// ReconcileRequestsFromWebhook returns a slice of reconcile.Request objects for the specified webhook
	ReconcileRequestsFromWebhook(webhook runtime.Object) []reconcile.Request
	// UpdateCABundle updates the caBundle for the webhook.  The webhook namespace identifies the type of webhook (validating or mutating).
	UpdateCABundle(ctx context.Context, cl client.Client, webhook types.NamespacedName) error
}

// WebhookCABundleManagerInstance is a singleton used to manage the caBundle on a
// webhook's clientConfig.
var WebhookCABundleManagerInstance = newWebhookCABundleManager()

type webhookCABundleManager struct {
	mu                sync.RWMutex
	webhooksToSecrets map[types.NamespacedName]secretInfo
	secretsToWebhooks map[types.NamespacedName]map[types.NamespacedName]struct{}
}

var _ WebhookCABundleManager = (*webhookCABundleManager)(nil)

// ManageWebhookCABundle registers the webhook to be managed with the secret that
// should be used to populate its caBundle field.
func (wm *webhookCABundleManager) ManageWebhookCABundle(obj runtime.Object, secret types.NamespacedName, keyName string) error {
	webhook, err := wm.namespacedNameForWebhook(obj)
	if err != nil {
		return err
	}
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if _, ok := wm.webhooksToSecrets[webhook]; ok {
		return fmt.Errorf("Already watching webhook %s", webhook)
	}
	wm.webhooksToSecrets[webhook] = secretInfo{NamespacedName: secret, keyName: keyName}
	webhooks := wm.secretsToWebhooks[secret]
	if webhooks == nil {
		webhooks = make(map[types.NamespacedName]struct{})
		wm.secretsToWebhooks[secret] = webhooks
	}
	webhooks[webhook] = struct{}{}
	return nil
}

// UnmanageWebhookCABundle removes the webhook from being managed
func (wm *webhookCABundleManager) UnmanageWebhookCABundle(obj runtime.Object) error {
	key, err := wm.namespacedNameForWebhook(obj)
	if err != nil {
		return err
	}
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if secret, ok := wm.webhooksToSecrets[key]; ok {
		delete(wm.webhooksToSecrets, key)
		webhooks := wm.secretsToWebhooks[secret.NamespacedName]
		if webhooks != nil {
			delete(webhooks, key)
		}
	} else {
		return fmt.Errorf("Not managing webhook %s", key)
	}
	return nil
}

func (wm *webhookCABundleManager) IsManaged(obj runtime.Object) bool {
	key, err := wm.namespacedNameForWebhook(obj)
	if err != nil {
		return false
	}
	return len(wm.secretForWebhook(key).Name) > 0
}

func (wm *webhookCABundleManager) IsManagingWebhooksForSecret(secret types.NamespacedName) bool {
	return len(wm.webhooksForSecret(secret)) > 0
}

func (wm *webhookCABundleManager) ReconcileRequestsFromWebhook(webhook runtime.Object) []reconcile.Request {
	webhookName, err := wm.namespacedNameForWebhook(webhook)
	if err != nil {
		return nil
	}
	return []reconcile.Request{reconcile.Request{NamespacedName: webhookName}}
}

func (wm *webhookCABundleManager) ReconcileRequestsFromSecret(secret types.NamespacedName) []reconcile.Request {
	webhooks := wm.webhooksForSecret(secret)
	requests := make([]reconcile.Request, len(webhooks))
	for index, webhook := range webhooks {
		requests[index] = reconcile.Request{
			NamespacedName: webhook,
		}
	}
	return requests
}

func (wm *webhookCABundleManager) webhooksForSecret(secret types.NamespacedName) []types.NamespacedName {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	webhooksMap := wm.secretsToWebhooks[secret]
	if webhooksMap == nil {
		return []types.NamespacedName{}
	}
	webhooks := make([]types.NamespacedName, 0, len(webhooksMap))
	for webhook := range webhooksMap {
		webhooks = append(webhooks, webhook)
	}
	return webhooks
}

func (wm *webhookCABundleManager) secretForWebhook(webhookName types.NamespacedName) secretInfo {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.webhooksToSecrets[webhookName]
}

const (
	validatingNamespaceValue = "validating"
	mutatingNamespaceValue   = "mutating"
)

// namespacedNameForWebhook returns a types.NamespacedName used to identify the
// webhook within the manager.  The key is composed of type and name in the
// form, <type>/<name>
func (wm *webhookCABundleManager) namespacedNameForWebhook(obj runtime.Object) (types.NamespacedName, error) {
	switch wh := obj.(type) {
	case *v1beta1.ValidatingWebhookConfiguration:
		return types.NamespacedName{Namespace: validatingNamespaceValue, Name: wh.GetName()}, nil
	case *v1beta1.MutatingWebhookConfiguration:
		return types.NamespacedName{Namespace: mutatingNamespaceValue, Name: wh.GetName()}, nil
	}
	return types.NamespacedName{}, fmt.Errorf("Object is not a MutatingWebhookConfiguration or ValidatingWebhookConfiguration")
}

// getWebhookGetter returns a factory for the object.  Note, the type of webhook
// is passed through the Namespace field.
func (wm *webhookCABundleManager) getWebhookWrapper(ctx context.Context, cl client.Client, webhook types.NamespacedName) (wrapper webhookWrapper, err error) {
	switch webhook.Namespace {
	case validatingNamespaceValue:
		return validatingWebhook.Get(ctx, cl, types.NamespacedName{Name: webhook.Name})
	case mutatingNamespaceValue:
		return mutatingWebhook.Get(ctx, cl, types.NamespacedName{Name: webhook.Name})
	}
	return nil, fmt.Errorf("unsupported webhook type: %s", webhook.String())
}

func newWebhookCABundleManager() WebhookCABundleManager {
	return &webhookCABundleManager{
		webhooksToSecrets: make(map[types.NamespacedName]secretInfo),
		secretsToWebhooks: make(map[types.NamespacedName]map[types.NamespacedName]struct{}),
	}
}

type secretInfo struct {
	types.NamespacedName
	keyName string
}
