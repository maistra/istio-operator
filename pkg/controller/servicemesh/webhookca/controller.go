package webhookca

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

	"github.com/maistra/istio-operator/pkg/controller/common"
)

const controllerName = "webhookca-controller"

const (
	galleySecretName                 = "istio.istio-galley-service-account"
	galleyWebhookNamePrefix          = "istio-galley-"
	istiodSecretName                 = "istio-ca-secret"
	istiodWebhookNamePrefix          = "istiod-"
	sidecarInjectorSecretName        = "istio.istio-sidecar-injector-service-account"
	sidecarInjectorWebhookNamePrefix = "istio-sidecar-injector-"
	ServiceMeshControlPlaneCRDName   = "servicemeshcontrolplanes.maistra.io"
)

// autoRegistrationMap maps webhook name prefixes to a secret name. This
// is used to auto register the webhook with the WebhookCABundleManager.
var autoRegistrationMap = map[string]registrationMapEntry{
	galleyWebhookNamePrefix: {
		secretName: galleySecretName,
		caFileName: common.IstioRootCertKey,
	},
	sidecarInjectorWebhookNamePrefix: {
		secretName: sidecarInjectorSecretName,
		caFileName: common.IstioRootCertKey,
	},
	istiodWebhookNamePrefix: {
		secretName: istiodSecretName,
		caFileName: common.IstiodCertKey,
	},
}

type registrationMapEntry struct {
	secretName string
	caFileName string
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
			return r.webhookCABundleManager.ReconcileRequestsFromSource(
				CABundleSource{
					Kind:           CABundleSourceKindSecret,
					NamespacedName: common.ToNamespacedName(obj.Meta)})
		})}, sourceWatchPredicates(r, CABundleSourceKindSecret))
	if err != nil {
		return err
	}

	// Watch config map
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
			return r.webhookCABundleManager.ReconcileRequestsFromSource(
				CABundleSource{
					Kind:           CABundleSourceKindConfigMap,
					NamespacedName: common.ToNamespacedName(obj.Meta)})
		})}, sourceWatchPredicates(r, CABundleSourceKindConfigMap))
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

func sourceWatchPredicates(r *reconciler, sourceKind CABundleSourceKind) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return r.webhookCABundleManager.IsManagingWebhooksForSource(
				CABundleSource{
					Kind:           sourceKind,
					NamespacedName: common.ToNamespacedName(event.Meta)})
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return r.webhookCABundleManager.IsManagingWebhooksForSource(
				CABundleSource{
					Kind:           sourceKind,
					NamespacedName: common.ToNamespacedName(event.MetaNew)})
		},
		// deletion and generic events don't interest us
		DeleteFunc: func(event event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return false
		},
	}
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
			if _, isCRD := event.Object.(*apixv1.CustomResourceDefinition); !isCRD {
				for prefix, registration := range autoRegistrationMap {
					if strings.HasPrefix(objName, prefix) {
						if err := webhookCABundleManager.ManageWebhookCABundle(
							event.Object,
							CABundleSource{
								Kind: CABundleSourceKindSecret,
								NamespacedName: types.NamespacedName{
									Namespace: "",
									Name:      registration.secretName,
								}}, registration.caFileName); err == nil {
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
						if err := webhookCABundleManager.UnmanageWebhookCABundle(event.Object); err != nil {
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
	caBundleSourceConfig, ok := wm.caBundleSourceForWebhook(webhookName)
	if !ok {
		logger.Error(nil, "webhook is not registered with the caBundle manager")
		return nil
	}

	caBundle, err := wm.getCABundleFromSource(ctx, cl, caBundleSourceConfig)
	if err != nil {
		logger.Info("could not get CA bundle", "caBundleSourceConfig", caBundleSourceConfig, "error", err)
		return nil
	}
	return currentConfig.UpdateCABundle(ctx, cl, caBundle)
}

// GetCABundleFromSource retrieves the CA bundle from a secret or config map
func (wm *webhookCABundleManager) getCABundleFromSource(ctx context.Context, cl client.Client, config caBundleSourceConfig) ([]byte, error) {
	switch config.Kind {
	case CABundleSourceKindSecret:
		return common.GetCABundleFromSecret(ctx, cl, config.NamespacedName, config.keyName)
	case CABundleSourceKindConfigMap:
		return common.GetCABundleFromConfigMap(ctx, cl, config.NamespacedName, config.keyName)
	default:
		panic("Invalid source type " + config.Kind)
	}
}

// Don't use this function to obtain a logger. Get it by invoking
// common.LogFromContext(ctx) to ensure that the logger has the
// correct context info and logs it.
func createLogger() logr.Logger {
	return logf.Log.WithName(controllerName)
}
