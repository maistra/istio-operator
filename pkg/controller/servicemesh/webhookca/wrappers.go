package webhookca

import (
	"context"
	"fmt"

	"k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type webhookGetter interface {
	Get(ctx context.Context, cl client.Client, name types.NamespacedName) (webhookWrapper, error)
}

type webhookWrapper interface {
	MetaObject() metav1.Object
	Object() runtime.Object
	ClientConfigs() []*v1beta1.WebhookClientConfig
	Copy() webhookWrapper
	NamespacedName() types.NamespacedName
}

func toWebhookWrapper(obj runtime.Object) (webhookWrapper, error) {
	switch wh := obj.(type) {
	case *v1beta1.ValidatingWebhookConfiguration:
		return &validatingWebhookWrapper{ValidatingWebhookConfiguration: wh}, nil
	case *v1beta1.MutatingWebhookConfiguration:
		return &mutatingWebhookWrapper{MutatingWebhookConfiguration: wh}, nil
	}
	return nil, fmt.Errorf("Object is not a MutatingWebhookConfiguration or ValidatingWebhookConfiguration")
}
