package webhooks

import (
	"context"
	"fmt"
	"sync/atomic"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/webhookca"
)

const (
	webhookSecretName    = "maistra-operator-serving-cert"
	webhookConfigMapName = "maistra-operator-cabundle"
	webhookServiceName   = "maistra-admission-controller"

	injectCABundleKey = "service.beta.openshift.io/inject-cabundle"
)

// Returns a Runnable that will remove the resources that previous versions of
// the operator created for webhooks, so that they don't conflict with the
// new resources that are deployed by OLM.
func NewCleanupRunnable(cl client.Client) *WebhookCleanupRunnable {
	return &WebhookCleanupRunnable{client: cl}
}

type WebhookCleanupRunnable struct {
	client client.Client
	done   atomic.Bool
}

var (
	_ manager.Runnable               = &WebhookCleanupRunnable{}
	_ manager.LeaderElectionRunnable = &WebhookCleanupRunnable{}
)

func (r *WebhookCleanupRunnable) Start(_ <-chan struct{}) error {
	ctx := common.NewContextWithLog(common.NewContext(), logf.Log.WithName("webhook-cleanup"))

	if err := removeObsoleteWebhookResources(ctx, r.client); err != nil {
		return err
	}
	if err := disableCRDBundleInjection(ctx, r.client); err != nil {
		return err
	}

	r.done.Store(true)
	return nil
}

// NeedLeaderElection returns false because it needs to run before the operator
// becomes the leader, since the operator also starts serving webhooks before
// it becomes the leader. If the webhooks are cleaned up as soon as the operator
// starts, then the webhook server will log TLS certificate errors because the
// old certificate is still being used.
// With NeedLeaderElection being false, if multiple instances of the operator
// start at the same time, they will all try to delete these resources, but this
// shouldn't be an issue because the code that deletes them handles conflicts.
func (r *WebhookCleanupRunnable) NeedLeaderElection() bool {
	return false
}

func (r *WebhookCleanupRunnable) Done() bool {
	return r.done.Load()
}

// removeObsoleteWebhookResources removes the Validating/MutatingWebhookConfiguration,
// and the associated Service, Secret, and ConfigMap that were created by previous
// versions of the operator.
func removeObsoleteWebhookResources(ctx context.Context, cl client.Client) error {
	operatorNamespace := common.GetOperatorNamespace()

	type webhookResource struct {
		obj       runtime.Object
		name      string
		namespace string
	}
	webhookResources := []webhookResource{
		{obj: &corev1.Service{}, name: webhookServiceName, namespace: operatorNamespace},
		{obj: &corev1.Secret{}, name: webhookSecretName, namespace: operatorNamespace},
		{obj: &corev1.ConfigMap{}, name: webhookConfigMapName, namespace: operatorNamespace},
		{obj: &admissionv1.ValidatingWebhookConfiguration{}, name: validatingWebhookConfigName(operatorNamespace)},
		{obj: &admissionv1.MutatingWebhookConfiguration{}, name: mutatingWebhookConfigName(operatorNamespace)},
	}
	for _, res := range webhookResources {
		if err := deleteObject(ctx, cl, res.obj, res.name, res.namespace); err != nil {
			return err
		}
	}
	return nil
}

func deleteObject(ctx context.Context, cl client.Client, obj runtime.Object, name, ns string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := cl.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, obj); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		log := common.LogFromContext(ctx)
		if err := cl.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Deleted old webhook object", "kind", obj.GetObjectKind().GroupVersionKind(), "name", name, "namespace", ns)
		return nil
	})
}

// disableCRDBundleInjection removes the annotation that causes the CA bundle to
// be injected into the servicemeshcontrolplanes CRD. This must be done so that
// OLM and the service-ca operator don't fight over the CA bundle, causing the
// CRD to be updated continuously.
// Note: the CRD definition bundled with the operator already has this annotation
// set to false, but since it's critical for this annotation to be false to prevent
// the issue described above, we set it to false here as well.
func disableCRDBundleInjection(ctx context.Context, cl client.Client) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := cl.Get(ctx, client.ObjectKey{Name: webhookca.ServiceMeshControlPlaneCRDName}, crd); err != nil {
			return err
		}

		if crd.Annotations != nil && crd.Annotations[injectCABundleKey] == "true" {
			crd.Annotations[injectCABundleKey] = "false"
			if err := cl.Update(ctx, crd); err != nil {
				return err
			}
			log.Info("Updated CRD annotation value from true to false", "annotation", injectCABundleKey, "crd", webhookca.ServiceMeshControlPlaneCRDName)
		}
		return nil
	})
}

func validatingWebhookConfigName(ns string) string {
	return fmt.Sprintf("%s.servicemesh-resources.maistra.io", ns)
}

func mutatingWebhookConfigName(namespace string) string {
	return fmt.Sprintf("%s.servicemesh-resources.maistra.io", namespace)
}
