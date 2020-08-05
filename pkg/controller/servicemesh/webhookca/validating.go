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

var validatingWebhook webhookGetter = &validatingWebhookGetter{}

type validatingWebhookGetter struct{}

var _ webhookGetter = (*validatingWebhookGetter)(nil)

func (vwg *validatingWebhookGetter) Get(ctx context.Context, cl client.Client, name types.NamespacedName) (webhookWrapper, error) {
	obj := &v1beta1.ValidatingWebhookConfiguration{}
	if err := cl.Get(ctx, name, obj); err != nil {
		return nil, err
	}
	return &validatingWebhookWrapper{ValidatingWebhookConfiguration: obj}, nil
}

type validatingWebhookWrapper struct {
	*v1beta1.ValidatingWebhookConfiguration
}

var _ webhookWrapper = (*validatingWebhookWrapper)(nil)

func (vw *validatingWebhookWrapper) Object() runtime.Object {
	return vw.ValidatingWebhookConfiguration
}

func (vw *validatingWebhookWrapper) MetaObject() metav1.Object {
	return vw.ValidatingWebhookConfiguration
}

func (vw *validatingWebhookWrapper) Copy() webhookWrapper {
	return &validatingWebhookWrapper{ValidatingWebhookConfiguration: vw.ValidatingWebhookConfiguration.DeepCopyObject().(*v1beta1.ValidatingWebhookConfiguration)}
}

func (vw *validatingWebhookWrapper) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: validatingNamespaceValue, Name: vw.GetName()}
}

func (vw *validatingWebhookWrapper) ClientConfigs() []*v1beta1.WebhookClientConfig {
	clientConfigs := make([]*v1beta1.WebhookClientConfig, len(vw.Webhooks))
	for index := range vw.Webhooks {
		clientConfigs[index] = &vw.Webhooks[index].ClientConfig
	}
	return clientConfigs
}
