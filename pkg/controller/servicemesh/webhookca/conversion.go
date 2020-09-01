package webhookca

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	v1 "k8s.io/api/admissionregistration/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

// this file contains implementations specific to validating webhooks

var conversionWebhook webhookGetter = &conversionWebhookGetter{}

type conversionWebhookGetter struct{}

var _ webhookGetter = (*mutatingWebhookGetter)(nil)

func (mwg *conversionWebhookGetter) Get(ctx context.Context, cl client.Client, name types.NamespacedName) (webhookWrapper, error) {
	obj := &apixv1.CustomResourceDefinition{}
	if err := cl.Get(ctx, name, obj); err != nil {
		return nil, err
	}
	return &conversionWebhookWrapper{CustomResourceDefinition: obj}, nil
}

type conversionWebhookWrapper struct {
	*apixv1.CustomResourceDefinition
}

var _ webhookWrapper = (*conversionWebhookWrapper)(nil)

func (mw *conversionWebhookWrapper) Object() runtime.Object {
	return mw.CustomResourceDefinition
}

func (mw *conversionWebhookWrapper) MetaObject() metav1.Object {
	return mw.CustomResourceDefinition
}

func (mw *conversionWebhookWrapper) Copy() webhookWrapper {
	return &conversionWebhookWrapper{CustomResourceDefinition: mw.CustomResourceDefinition.DeepCopy()}
}

func (mw *conversionWebhookWrapper) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: conversionNamespaceValue, Name: mw.GetName()}
}

func (mw *conversionWebhookWrapper) ClientConfigs() []*v1.WebhookClientConfig {
	// This helps with testing
	if mw.Spec.Conversion == nil || mw.Spec.Conversion.Strategy != apixv1.WebhookConverter || mw.Spec.Conversion.Webhook == nil || mw.Spec.Conversion.Webhook.ClientConfig == nil {
		return nil
	}
	return []*v1.WebhookClientConfig{
		{
			Service: &v1.ServiceReference{
				Name:      mw.Spec.Conversion.Webhook.ClientConfig.Service.Name,
				Namespace: mw.Spec.Conversion.Webhook.ClientConfig.Service.Namespace,
				Path:      mw.Spec.Conversion.Webhook.ClientConfig.Service.Path,
			},
			CABundle: mw.Spec.Conversion.Webhook.ClientConfig.CABundle,
		},
	}
}

func (mw *conversionWebhookWrapper) UpdateCABundle(ctx context.Context, cl client.Client, caBundle []byte) error {
	logger := common.LogFromContext(ctx)
	if mw.Spec.Conversion == nil || mw.Spec.Conversion.Strategy != apixv1.WebhookConverter || mw.Spec.Conversion.Webhook == nil || mw.Spec.Conversion.Webhook.ClientConfig == nil {
		logger.Info("Skipping CABundle update, no webhook client.")
		return nil
	}
	if bytes.Equal(mw.Spec.Conversion.Webhook.ClientConfig.CABundle, caBundle) {
		logger.Info("Correct CABundle already present. Ignoring")
		return nil
	}
	logger.Info("Updating CABundle")
	patched := mw.CustomResourceDefinition.DeepCopy()
	patched.Spec.Conversion.Webhook.ClientConfig.CABundle = caBundle
	if err := cl.Update(ctx, patched); err != nil {
		return errors.Wrap(err, "failed to update CABundle")
	}
	logger.Info("CABundle updated")
	return nil
}
