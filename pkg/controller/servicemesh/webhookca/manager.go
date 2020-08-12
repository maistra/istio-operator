package webhookca

import (
	"context"
	"fmt"
	"sync"

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
	webhook, err := toWebhookWrapper(obj)
	if err != nil {
		return err
	}
	info := secretInfo{NamespacedName: secret, keyName: keyName}
	info.Namespace = ""
	for _, clientConfig := range webhook.ClientConfigs() {
		if clientConfig.Service == nil {
			continue
		}
		if info.Namespace != "" && clientConfig.Service.Namespace != info.Namespace {
			return fmt.Errorf("webhook has multiple clients referencing services in multiple namespaces: %s", webhook.MetaObject().GetName())
		}
		info.Namespace = clientConfig.Service.Namespace
	}
	if info.Namespace == "" {
		return fmt.Errorf("no clients to configure in webhook: %s", webhook.MetaObject().GetName())
	}

	webhookName := webhook.NamespacedName()
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if _, ok := wm.webhooksToSecrets[webhookName]; ok {
		return fmt.Errorf("Already watching webhook %s", webhookName)
	}
	wm.webhooksToSecrets[webhookName] = info
	webhooks := wm.secretsToWebhooks[info.NamespacedName]
	if webhooks == nil {
		webhooks = make(map[types.NamespacedName]struct{})
		wm.secretsToWebhooks[info.NamespacedName] = webhooks
	}
	webhooks[webhookName] = struct{}{}
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
	if name, err := wm.namespacedNameForWebhook(obj); err == nil {
		wm.mu.RLock()
		defer wm.mu.RUnlock()
		_, ok := wm.webhooksToSecrets[name]
		return ok
	}
	return false
}

func (wm *webhookCABundleManager) IsManagingWebhooksForSecret(secret types.NamespacedName) bool {
	return len(wm.webhooksForSecret(secret)) > 0
}

func (wm *webhookCABundleManager) ReconcileRequestsFromWebhook(webhook runtime.Object) []reconcile.Request {
	webhookName, err := wm.namespacedNameForWebhook(webhook)
	if err != nil {
		return nil
	}
	return []reconcile.Request{{NamespacedName: webhookName}}
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

func (wm *webhookCABundleManager) secretForWebhook(webhookName types.NamespacedName) (secretInfo, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	secret, ok := wm.webhooksToSecrets[webhookName]
	return secret, ok
}

func (wm *webhookCABundleManager) initSecretForWebhook(webhook webhookWrapper) (secretInfo, error) {
	info := secretInfo{}
	for _, clientConfig := range webhook.ClientConfigs() {
		if clientConfig.Service == nil {
			continue
		}
		if info.Namespace != "" && clientConfig.Service.Namespace != info.Namespace {
			return info, fmt.Errorf("webhook has multiple clients referencing services in multiple namespaces: %s", webhook.MetaObject().GetName())
		}
		info.Namespace = clientConfig.Service.Namespace
	}
	if info.Namespace == "" {
		return info, fmt.Errorf("no clients to configure in webhook: %s", webhook.MetaObject().GetName())
	}
	return info, nil
}

const (
	validatingNamespaceValue = "validating"
	mutatingNamespaceValue   = "mutating"
	conversionNamespaceValue = "conversion"
)

// namespacedNameForWebhook returns a types.NamespacedName used to identify the
// webhook within the manager.  The key is composed of type and name in the
// form, <type>/<name>
func (wm *webhookCABundleManager) namespacedNameForWebhook(obj runtime.Object) (types.NamespacedName, error) {
	wh, err := toWebhookWrapper(obj)
	if err == nil {
		return wh.NamespacedName(), nil
	}
	return types.NamespacedName{}, err
}

// getWebhookGetter returns a factory for the object.  Note, the type of webhook
// is passed through the Namespace field.
func (wm *webhookCABundleManager) getWebhookWrapper(ctx context.Context, cl client.Client, webhook types.NamespacedName) (wrapper webhookWrapper, err error) {
	switch webhook.Namespace {
	case validatingNamespaceValue:
		return validatingWebhook.Get(ctx, cl, types.NamespacedName{Name: webhook.Name})
	case mutatingNamespaceValue:
		return mutatingWebhook.Get(ctx, cl, types.NamespacedName{Name: webhook.Name})
	case conversionNamespaceValue:
		return conversionWebhook.Get(ctx, cl, types.NamespacedName{Name: webhook.Name})
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
