package webhookca

import (
	"context"

	"k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type webhookGetter interface {
	Get(ctx context.Context, cl client.Client, name types.NamespacedName) (webhookWrapper, error)
}

type webhookWrapper interface {
	runtime.Object
	Object() runtime.Object
	ClientConfigs() []*v1beta1.WebhookClientConfig
	Copy() webhookWrapper
}
