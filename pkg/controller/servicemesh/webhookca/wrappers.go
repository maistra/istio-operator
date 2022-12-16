package webhookca

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/api/admissionregistration/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

type webhookGetter interface {
	Get(ctx context.Context, cl client.Client, name types.NamespacedName) (webhookWrapper, error)
}

type webhookWrapper interface {
	MetaObject() metav1.Object
	Object() client.Object
	ClientConfigs() []*v1.WebhookClientConfig
	Copy() webhookWrapper
	NamespacedName() types.NamespacedName
	UpdateCABundle(ctx context.Context, cl client.Client, caBundle []byte) error
}

func toWebhookWrapper(obj runtime.Object) (webhookWrapper, error) {
	switch wh := obj.(type) {
	case *v1.ValidatingWebhookConfiguration:
		return &validatingWebhookWrapper{ValidatingWebhookConfiguration: wh}, nil
	case *v1.MutatingWebhookConfiguration:
		return &mutatingWebhookWrapper{MutatingWebhookConfiguration: wh}, nil
	case *apixv1.CustomResourceDefinition:
		return &conversionWebhookWrapper{CustomResourceDefinition: wh}, nil
	}
	return nil, fmt.Errorf("object is not a [MutatingWebhookConfiguration, ValidatingWebhookConfiguration, CustomResourceDefinition]: %T", obj)
}

func updateAdmissionWebHookCABundles(ctx context.Context, cl client.Client, currentConfig webhookWrapper, caBundle []byte) error {
	logger := common.LogFromContext(ctx)
	updated := false
	newConfig := currentConfig.Copy()
	for _, clientConfig := range newConfig.ClientConfigs() {
		updated = common.InjectCABundle(clientConfig, caBundle) || updated
	}

	if updated {
		logger.Info("Updating CABundle")
		err := cl.Update(ctx, newConfig.Object())
		if err != nil {
			return errors.Wrap(err, "failed to update CABundle")
		}
		logger.Info("CABundle updated")
		return nil
	}

	logger.Info("Correct CABundle already present. Ignoring")
	return nil
}
