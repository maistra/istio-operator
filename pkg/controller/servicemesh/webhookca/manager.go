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
	ManageWebhookCABundle(obj runtime.Object, source CABundleSource, keyName string) error
	// UnmanageWebhookCABundle removes a webhook from the manager.
	UnmanageWebhookCABundle(obj runtime.Object) error
	// IsManaged returns true if the webhook is being managed.
	IsManaged(obj runtime.Object) bool
	// IsManagingWebhooksForSource returns true if any webhooks being managed are using the secret or config map.
	IsManagingWebhooksForSource(source CABundleSource) bool
	// ReconcileRequestsFromSource returns a slice of reconcile.Request objects for the specified secret or config map
	ReconcileRequestsFromSource(source CABundleSource) []reconcile.Request
	// ReconcileRequestsFromWebhook returns a slice of reconcile.Request objects for the specified webhook
	ReconcileRequestsFromWebhook(webhook runtime.Object) []reconcile.Request
	// UpdateCABundle updates the caBundle for the webhook.  The webhook namespace identifies the type of webhook (validating or mutating).
	UpdateCABundle(ctx context.Context, cl client.Client, webhook types.NamespacedName) error
}

// WebhookCABundleManagerInstance is a singleton used to manage the caBundle on a
// webhook's clientConfig.
var WebhookCABundleManagerInstance = newWebhookCABundleManager()

type webhookCABundleManager struct {
	mu                     sync.RWMutex
	webhooksToBundleSource map[types.NamespacedName]caBundleSourceConfig
	sourcesToWebhooks      map[CABundleSource]map[types.NamespacedName]struct{}
}

var _ WebhookCABundleManager = (*webhookCABundleManager)(nil)

// ManageWebhookCABundle registers the webhook to be managed with the Secret/ConfigMap that
// should be used to populate its caBundle field.
func (wm *webhookCABundleManager) ManageWebhookCABundle(obj runtime.Object, source CABundleSource, keyName string) error {
	webhook, err := toWebhookWrapper(obj)
	if err != nil {
		return err
	}
	source.Namespace = ""
	for _, clientConfig := range webhook.ClientConfigs() {
		if clientConfig.Service == nil {
			continue
		}
		if source.Namespace != "" && clientConfig.Service.Namespace != source.Namespace {
			return fmt.Errorf("webhook has multiple clients referencing services in multiple namespaces: %s", webhook.MetaObject().GetName())
		}
		source.Namespace = clientConfig.Service.Namespace
	}
	if source.Namespace == "" {
		return fmt.Errorf("no clients to configure in webhook: %s", webhook.MetaObject().GetName())
	}

	webhookName := webhook.NamespacedName()
	wm.mu.Lock()
	defer wm.mu.Unlock()
	if _, ok := wm.webhooksToBundleSource[webhookName]; ok {
		return fmt.Errorf("Already watching webhook %s", webhookName)
	}
	wm.webhooksToBundleSource[webhookName] = caBundleSourceConfig{CABundleSource: source, keyName: keyName}

	webhooks := wm.sourcesToWebhooks[source]
	if webhooks == nil {
		webhooks = make(map[types.NamespacedName]struct{})
		wm.sourcesToWebhooks[source] = webhooks
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
	if caBundleSourceConfig, ok := wm.webhooksToBundleSource[key]; ok {
		delete(wm.webhooksToBundleSource, key)
		webhooks := wm.sourcesToWebhooks[caBundleSourceConfig.CABundleSource]
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
		_, ok := wm.webhooksToBundleSource[name]
		return ok
	}
	return false
}

func (wm *webhookCABundleManager) IsManagingWebhooksForSource(source CABundleSource) bool {
	return len(wm.webhooksForSource(source)) > 0
}

func (wm *webhookCABundleManager) ReconcileRequestsFromWebhook(webhook runtime.Object) []reconcile.Request {
	webhookName, err := wm.namespacedNameForWebhook(webhook)
	if err != nil {
		return nil
	}
	return []reconcile.Request{{NamespacedName: webhookName}}
}

func (wm *webhookCABundleManager) ReconcileRequestsFromSource(source CABundleSource) []reconcile.Request {
	webhooks := wm.webhooksForSource(source)
	requests := make([]reconcile.Request, len(webhooks))
	for index, webhook := range webhooks {
		requests[index] = reconcile.Request{
			NamespacedName: webhook,
		}
	}
	return requests
}

func (wm *webhookCABundleManager) webhooksForSource(source CABundleSource) []types.NamespacedName {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	webhooksMap := wm.sourcesToWebhooks[source]
	if webhooksMap == nil {
		return []types.NamespacedName{}
	}
	webhooks := make([]types.NamespacedName, 0, len(webhooksMap))
	for webhook := range webhooksMap {
		webhooks = append(webhooks, webhook)
	}
	return webhooks
}

func (wm *webhookCABundleManager) caBundleSourceForWebhook(webhookName types.NamespacedName) (caBundleSourceConfig, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	source, ok := wm.webhooksToBundleSource[webhookName]
	return source, ok
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
		webhooksToBundleSource: make(map[types.NamespacedName]caBundleSourceConfig),
		sourcesToWebhooks:      make(map[CABundleSource]map[types.NamespacedName]struct{}),
	}
}

type CABundleSourceKind string

const CABundleSourceKindConfigMap = "ConfigMap"
const CABundleSourceKindSecret = "Secret"

type CABundleSource struct {
	Kind CABundleSourceKind
	types.NamespacedName
}

type caBundleSourceConfig struct {
	CABundleSource
	keyName string
}
