package webhookca

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

const controllerName = "webhookca-controller"

const (
	galleySecretName                 = "istio.istio-galley-service-account"
	galleyWebhookNamePrefix          = "istio-galley-"
	istiodSecretName                 = "istio-ca-secret"
	istiodCustomCertSecretName       = "cacerts"
	istiodWebhookNamePrefix          = "istiod-"
	istioValidatorWebhookNamePrefix  = "istio-validator-"
	sidecarInjectorSecretName        = "istio.istio-sidecar-injector-service-account"
	sidecarInjectorWebhookNamePrefix = "istio-sidecar-injector-"
	ServiceMeshControlPlaneCRDName   = "servicemeshcontrolplanes.maistra.io"
	ServiceMeshExtensionCRDName      = "servicemeshextensions.maistra.io"
)

// autoRegistrationMap maps webhook name prefixes to a list of secret names. This
// is used to auto register the webhook with the WebhookCABundleManager. Order of
// secretNames determines priority.
var autoRegistrationMap = map[string]CABundleSource{
	galleyWebhookNamePrefix: &SecretCABundleSource{
		SecretNames: []string{galleySecretName},
		Key:         common.IstioRootCertKey,
	},
	sidecarInjectorWebhookNamePrefix: &SecretCABundleSource{
		SecretNames: []string{sidecarInjectorSecretName},
		Key:         common.IstioRootCertKey,
	},
	istiodWebhookNamePrefix: &SecretCABundleSource{
		SecretNames: []string{istiodCustomCertSecretName, istiodSecretName},
		Key:         common.IstiodCertKey,
	},
	istioValidatorWebhookNamePrefix: &SecretCABundleSource{
		SecretNames: []string{istiodCustomCertSecretName, istiodSecretName},
		Key:         common.IstiodCertKey,
	},
}

// Add creates a new Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	if !common.Config.Controller.WebhookManagementEnabled {
		createLogger().Info("Not adding webhookca controller to Manager")
		return nil
	}

	return add(mgr, newReconciler(mgr.GetClient(), mgr.GetScheme(), WebhookCABundleManagerInstance))
}

func newReconciler(cl client.Client, scheme *runtime.Scheme, webhookCABundleManager WebhookCABundleManager) *reconciler {
	return &reconciler{
		ControllerResources: common.ControllerResources{
			Client: cl,
			Scheme: scheme,
		},
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
			return r.webhookCABundleManager.ReconcileRequestsFromSource(
				ObjectRef{
					Kind:      "Secret",
					Namespace: obj.Meta.GetNamespace(),
					Name:      obj.Meta.GetName(),
				})
		}),
	}, sourceWatchPredicates(r))
	if err != nil {
		return err
	}

	// Watch config map
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
			return r.webhookCABundleManager.ReconcileRequestsFromSource(
				ObjectRef{
					Kind:      "ConfigMap",
					Namespace: obj.Meta.GetNamespace(),
					Name:      obj.Meta.GetName(),
				})
		}),
	}, sourceWatchPredicates(r))
	if err != nil {
		return err
	}

	webhookEventHander := enqueueWebhookRequests(r.webhookCABundleManager)
	// Watch MutatingWebhookConfigurations
	err = c.Watch(
		&source.Kind{Type: &v1.MutatingWebhookConfiguration{}},
		webhookEventHander,
		webhookWatchPredicates(r.webhookCABundleManager))
	if err != nil {
		return err
	}

	// Watch ValidatingWebhookConfigurations
	err = c.Watch(
		&source.Kind{Type: &v1.ValidatingWebhookConfiguration{}},
		webhookEventHander,
		webhookWatchPredicates(r.webhookCABundleManager))
	if err != nil {
		return err
	}

	// Watch CustomResourceDefinition
	err = c.Watch(
		&source.Kind{Type: &apixv1.CustomResourceDefinition{}},
		webhookEventHander,
		webhookWatchPredicates(r.webhookCABundleManager))
	if err != nil {
		return err
	}
	return nil
}

func sourceWatchPredicates(r *reconciler) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return r.isWatchingSourceObject(event.Meta, event.Object)
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return r.isWatchingSourceObject(event.MetaNew, event.ObjectNew)
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			return r.isWatchingSourceObject(event.Meta, event.Object)
		},
		// generic events don't interest us
		GenericFunc: func(event event.GenericEvent) bool {
			return false
		},
	}
}

func (r *reconciler) isWatchingSourceObject(meta metav1.Object, object runtime.Object) bool {
	ref := ObjectRef{
		Namespace: meta.GetNamespace(),
		Name:      meta.GetName(),
	}
	kinds, _, err := r.Scheme.ObjectKinds(object)
	if err != nil {
		createLogger().Error(err, fmt.Sprintf("error retrieving object type for %s/%s", meta.GetNamespace(), meta.GetName()))
	}
	for _, kind := range kinds {
		ref.Kind = kind.Kind
		if r.webhookCABundleManager.IsManagingWebhooksForSource(ref) {
			return true
		}
	}
	return false
}

func enqueueWebhookRequests(webhookCABundleManager WebhookCABundleManager) handler.EventHandler {
	return &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
			return webhookCABundleManager.ReconcileRequestsFromWebhook(obj.Object)
		}),
	}
}

func webhookWatchPredicates(webhookCABundleManager WebhookCABundleManager) predicate.Predicate {
	return &predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) (ok bool) {
			objName := event.Meta.GetName()
			if _, isCRD := event.Object.(*apixv1.CustomResourceDefinition); !isCRD {
				for prefix, source := range autoRegistrationMap {
					if strings.HasPrefix(objName, prefix) {
						if err := webhookCABundleManager.ManageWebhookCABundle(event.Object, source.Copy()); err == nil {
							return true
						}
						// XXX: should we log an error here?
						return false
					}
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
			if webhookCABundleManager.IsManaged(event.Object) {
				for prefix := range autoRegistrationMap {
					if strings.HasPrefix(objName, prefix) {
						// remove sidecar injector webhook
						if err := webhookCABundleManager.UnmanageWebhookCABundle(event.Object); err != nil { // nolint:staticcheck
							// XXX: should we log an error here?
						}
						return false
					}
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
// from the respective Istio SA secret or CA Bundle config map
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := createLogger().WithValues("WebhookConfig", request.NamespacedName.String())
	logger.Info("reconciling WebhookConfiguration")
	ctx := common.NewReconcileContext(logger)
	return reconcile.Result{}, r.webhookCABundleManager.UpdateCABundle(ctx, r.Client, request.NamespacedName)
}

func (wm *webhookCABundleManager) UpdateCABundle(ctx context.Context, cl client.Client, webhookName types.NamespacedName) error {
	logger := common.LogFromContext(ctx)

	// get current webhook config
	currentConfig, err := wm.getWebhookWrapper(ctx, cl, webhookName)
	if err != nil {
		logger.Info("WebhookConfiguration does not exist yet. No action taken")
		return nil
	}
	caBundleSource, ok := wm.caBundleSourceForWebhook(webhookName)
	if !ok {
		logger.Error(nil, "webhook is not registered with the caBundle manager")
		return nil
	}

	caBundle, err := caBundleSource.GetCABundle(ctx, cl)
	if err != nil {
		logger.Info("could not get CA bundle", "caBundleSourceConfig", caBundleSource, "error", err)
		return nil
	}
	return currentConfig.UpdateCABundle(ctx, cl, caBundle)
}

// Don't use this function to obtain a logger. Get it by invoking
// common.LogFromContext(ctx) to ensure that the logger has the
// correct context info and logs it.
func createLogger() logr.Logger {
	return logf.Log.WithName(controllerName)
}
