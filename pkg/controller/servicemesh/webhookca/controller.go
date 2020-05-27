package webhookca

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/pkg/errors"
	"k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const controllerName = "webhookca-controller"

const (
	galleySecretName                 = "istio.istio-galley-service-account"
	galleyWebhookNamePrefix          = "istio-galley-"
	sidecarInjectorSecretName        = "istio.istio-sidecar-injector-service-account"
	sidecarInjectorWebhookNamePrefix = "istio-sidecar-injector-"
)

// autoRegistrationMap maps webhook name prefixes to a secret name.  This is
// is used to auto register the webhook with the WebhookCABundleManager.
var autoRegistrationMap = map[string]string{
	galleyWebhookNamePrefix:          galleySecretName,
	sidecarInjectorWebhookNamePrefix: sidecarInjectorSecretName,
}

// Add creates a new Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr.GetClient(), mgr.GetScheme(), WebhookCABundleManagerInstance))
}

func newReconciler(cl client.Client, scheme *runtime.Scheme, webhookCABundleManager WebhookCABundleManager) *reconciler {
	return &reconciler{ControllerResources: common.ControllerResources{
		Client: cl,
		Scheme: scheme},
		webhookCABundleManager: webhookCABundleManager,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch secret
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
			return r.webhookCABundleManager.ReconcileRequestsFromSecret(
				types.NamespacedName{
					Namespace: obj.Meta.GetNamespace(),
					Name:      obj.Meta.GetName(),
				})
		})}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return r.webhookCABundleManager.IsManagingWebhooksForSecret(
				types.NamespacedName{
					Namespace: event.Meta.GetNamespace(),
					Name:      event.Meta.GetName(),
				})
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return r.webhookCABundleManager.IsManagingWebhooksForSecret(
				types.NamespacedName{
					Namespace: event.MetaNew.GetNamespace(),
					Name:      event.MetaNew.GetName(),
				})
		},
		// deletion and generic events don't interest us
		DeleteFunc: func(event event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return false
		},
	})
	if err != nil {
		return err
	}

	webhookEventHander := enqueueWebhookRequests(r.webhookCABundleManager)
	// Watch MutatingWebhookConfigurations
	err = c.Watch(
		&source.Kind{Type: &v1beta1.MutatingWebhookConfiguration{}},
		webhookEventHander,
		webhookWatchPredicates(r.webhookCABundleManager))
	if err != nil {
		return err
	}

	// Watch ValidatingWebhookConfigurations
	err = c.Watch(
		&source.Kind{Type: &v1beta1.ValidatingWebhookConfiguration{}},
		webhookEventHander,
		webhookWatchPredicates(r.webhookCABundleManager))
	if err != nil {
		return err
	}

	return nil
}

func enqueueWebhookRequests(webhookCABundleManager WebhookCABundleManager) handler.EventHandler {
	return &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
			return webhookCABundleManager.ReconcileRequestsFromWebhook(obj.Object)
		})}
}

func webhookWatchPredicates(webhookCABundleManager WebhookCABundleManager) predicate.Predicate {
	return &predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) (ok bool) {
			objName := event.Meta.GetName()
			for prefix, secret := range autoRegistrationMap {
				if strings.HasPrefix(objName, prefix) {
					namespace := objName[len(prefix):]
					if err := webhookCABundleManager.ManageWebhookCABundle(
						event.Object,
						types.NamespacedName{
							Namespace: namespace,
							Name:      secret,
						},
						common.IstioRootCertKey); err == nil {
						return true
					}
					// XXX: should we log an error here?
					return false
				}
			}
			return webhookCABundleManager.IsManaged(event.Object)
		},
		UpdateFunc: func(event event.UpdateEvent) (ok bool) {
			return webhookCABundleManager.IsManaged(event.ObjectNew)
		},
		// deletion and generic events don't interest us
		DeleteFunc: func(event event.DeleteEvent) bool {
			objName := event.Meta.GetName()
			for prefix := range autoRegistrationMap {
				if strings.HasPrefix(objName, prefix) {
					// remove sidecar injector webhook
					if err := webhookCABundleManager.UnmanageWebhookCABundle(event.Object); err != nil {
						// XXX: should we log an error here?
					}
					return false
				}
			}
			return false
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return false
		},
	}
}

// reconciles webhook configurations
type reconciler struct {
	common.ControllerResources
	webhookCABundleManager WebhookCABundleManager
}

// Reconcile updates ClientConfigs of MutatingWebhookConfigurations to contain the CABundle
// from the respective Istio SA secret
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := createLogger().WithValues("WebhookConfig", request.NamespacedName.String())
	logger.Info("reconciling WebhookConfiguration")
	ctx := common.NewReconcileContext(logger)
	return reconcile.Result{}, r.webhookCABundleManager.UpdateCABundle(ctx, r.Client, request.NamespacedName)
}

func (wm *webhookCABundleManager) UpdateCABundle(ctx context.Context, cl client.Client, webhook types.NamespacedName) error {
	logger := common.LogFromContext(ctx)

	// get current webhook config
	currentConfig, err := wm.getWebhookWrapper(ctx, cl, webhook)
	if err != nil {
		logger.Info("WebhookConfiguration does not exist yet. No action taken")
		return nil
	}
	if !wm.IsManaged(currentConfig.Object()) {
		logger.Error(nil, "webhook is not registered with the caBundle manager")
		return nil
	}

	secret := wm.secretForWebhook(webhook)
	caRoot, err := common.GetRootCertFromSecret(ctx, cl, secret.Namespace, secret.Name, secret.keyName)
	if err != nil {
		logger.Info("could not get secret: " + err.Error())
		return nil
	}
	// update caBundle if it doesn't match what's in the secret
	updated := false
	newConfig := currentConfig.Copy().(webhookWrapper)
	for _, clientConfig := range newConfig.ClientConfigs() {
		updated = common.InjectCABundle(clientConfig, caRoot) || updated
	}

	if updated {
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

// Don't use this function to obtain a logger. Get it by invoking
// common.LogFromContext(ctx) to ensure that the logger has the
// correct context info and logs it.
func createLogger() logr.Logger {
	return logf.Log.WithName(controllerName)
}
