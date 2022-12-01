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

var mutatingWebhook webhookGetter = &mutatingWebhookGetter{}

type mutatingWebhookGetter struct{}

var _ webhookGetter = (*mutatingWebhookGetter)(nil)

func (mwg *mutatingWebhookGetter) Get(ctx context.Context, cl client.Client, name types.NamespacedName) (webhookWrapper, error) {
	obj := &v1.MutatingWebhookConfiguration{}
	if err := cl.Get(ctx, name, obj); err != nil {
		return nil, err
	}
	return &mutatingWebhookWrapper{MutatingWebhookConfiguration: obj}, nil
}

type mutatingWebhookWrapper struct {
	*v1.MutatingWebhookConfiguration
}

var _ webhookWrapper = (*mutatingWebhookWrapper)(nil)

func (mw *mutatingWebhookWrapper) Object() runtime.Object {
	return mw.MutatingWebhookConfiguration
}

func (mw *mutatingWebhookWrapper) MetaObject() metav1.Object {
	return mw.MutatingWebhookConfiguration
}

func (mw *mutatingWebhookWrapper) Copy() webhookWrapper {
	return &mutatingWebhookWrapper{MutatingWebhookConfiguration: mw.MutatingWebhookConfiguration.DeepCopy()}
}

func (mw *mutatingWebhookWrapper) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: mutatingNamespaceValue, Name: mw.GetName()}
}

func (mw *mutatingWebhookWrapper) ClientConfigs() []*v1.WebhookClientConfig {
	clientConfigs := make([]*v1.WebhookClientConfig, len(mw.Webhooks))
	for index := range mw.Webhooks {
		clientConfigs[index] = &mw.Webhooks[index].ClientConfig
	}
	return clientConfigs
}

func (mw *mutatingWebhookWrapper) UpdateCABundle(ctx context.Context, cl client.Client, caBundle []byte) error {
	return updateAdmissionWebHookCABundles(ctx, cl, mw, caBundle)
}
