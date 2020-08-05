package webhookca

import (
	"context"

	"k8s.io/api/admissionregistration/v1beta1"
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
	obj := &v1beta1.MutatingWebhookConfiguration{}
	if err := cl.Get(ctx, name, obj); err != nil {
		return nil, err
	}
	return &mutatingWebhookWrapper{MutatingWebhookConfiguration: obj}, nil
}

type mutatingWebhookWrapper struct {
	*v1beta1.MutatingWebhookConfiguration
}

var _ webhookWrapper = (*mutatingWebhookWrapper)(nil)

func (mw *mutatingWebhookWrapper) Object() runtime.Object {
	return mw.MutatingWebhookConfiguration
}

func (mw *mutatingWebhookWrapper) MetaObject() metav1.Object {
	return mw.MutatingWebhookConfiguration
}

func (mw *mutatingWebhookWrapper) Copy() webhookWrapper {
	return &mutatingWebhookWrapper{MutatingWebhookConfiguration: mw.MutatingWebhookConfiguration.DeepCopyObject().(*v1beta1.MutatingWebhookConfiguration)}
}

func (mw *mutatingWebhookWrapper) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: mutatingNamespaceValue, Name: mw.GetName()}
}

func (mw *mutatingWebhookWrapper) ClientConfigs() []*v1beta1.WebhookClientConfig {
	clientConfigs := make([]*v1beta1.WebhookClientConfig, len(mw.Webhooks))
	for index := range mw.Webhooks {
		clientConfigs[index] = &mw.Webhooks[index].ClientConfig
	}
	return clientConfigs
}
