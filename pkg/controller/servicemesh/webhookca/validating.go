package webhookca

import (
	"context"

	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// this file contains implementations specific to validating webhooks

var validatingWebhook webhookGetter = &validatingWebhookGetter{}

type validatingWebhookGetter struct{}

var _ webhookGetter = (*validatingWebhookGetter)(nil)

func (vwg *validatingWebhookGetter) Get(ctx context.Context, cl client.Client, name types.NamespacedName) (webhookWrapper, error) {
	obj := &v1.ValidatingWebhookConfiguration{}
	if err := cl.Get(ctx, name, obj); err != nil {
		return nil, err
	}
	return &validatingWebhookWrapper{ValidatingWebhookConfiguration: obj}, nil
}

type validatingWebhookWrapper struct {
	*v1.ValidatingWebhookConfiguration
}

var _ webhookWrapper = (*validatingWebhookWrapper)(nil)

func (w *validatingWebhookWrapper) Object() runtime.Object {
	return w.ValidatingWebhookConfiguration
}

func (w *validatingWebhookWrapper) MetaObject() metav1.Object {
	return w.ValidatingWebhookConfiguration
}

func (w *validatingWebhookWrapper) Copy() webhookWrapper {
	return &validatingWebhookWrapper{ValidatingWebhookConfiguration: w.ValidatingWebhookConfiguration.DeepCopy()}
}

func (w *validatingWebhookWrapper) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: validatingNamespaceValue, Name: w.GetName()}
}

func (w *validatingWebhookWrapper) ClientConfigs() []*v1.WebhookClientConfig {
	clientConfigs := make([]*v1.WebhookClientConfig, len(w.Webhooks))
	for index := range w.Webhooks {
		clientConfigs[index] = &w.Webhooks[index].ClientConfig
	}
	return clientConfigs
}

func (w *validatingWebhookWrapper) UpdateCABundle(ctx context.Context, cl client.Client, caBundle []byte) error {
	return updateAdmissionWebHookCABundles(ctx, cl, w, caBundle)
}
