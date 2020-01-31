package validatingwebhook

import (
	"context"
	"strings"

	"k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/pkg/errors"
)

const controllerName = "validatingwebhook-controller"

var (
	serviceAccountSecretName = "istio.istio-galley-service-account"

	webhookConfigNamePrefix = "istio-galley-"
)

// Add creates a new Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr.GetClient(), mgr.GetScheme()))
}

func newReconciler(cl client.Client, scheme *runtime.Scheme) *reconciler {
	return &reconciler{ControllerResources: common.ControllerResources{
		Client: cl,
		Scheme: scheme,
		Log:    logf.Log.WithName(controllerName)}}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch secret
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
			requests := []reconcile.Request{}
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      webhookConfigNamePrefix + obj.Meta.GetNamespace(),
					Namespace: "",
				},
			})
			return requests
		})}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return event.Meta.GetName() == serviceAccountSecretName
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return event.MetaNew.GetName() == serviceAccountSecretName
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

	// Watch ValidatingWebhookConfigurations
	err = c.Watch(&source.Kind{Type: &v1beta1.ValidatingWebhookConfiguration{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return strings.HasPrefix(event.Meta.GetName(), webhookConfigNamePrefix)
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return strings.HasPrefix(event.MetaNew.GetName(), webhookConfigNamePrefix)
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

	return nil
}

// reconciles webhook configurations
type reconciler struct {
	common.ControllerResources
}

// Reconcile updates ClientConfigs of ValidatingWebhookConfigurations to contain the CABundle
// from the respective Istio SA secret
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := r.Log.WithValues("WebhookConfig", request.Name)
	logger.Info("reconciling ValidatingWebhookConfiguration")
	// get current webhook config
	currentConfig := &v1beta1.ValidatingWebhookConfiguration{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, currentConfig)
	if err != nil {
		r.Log.Info("ValidatingWebhookConfiguration does not exist yet. No action taken")
		return reconcile.Result{}, nil
	}
	namespace := request.Name[len(webhookConfigNamePrefix):]
	caRoot, err := common.GetRootCertFromSecret(r.Client, namespace, serviceAccountSecretName)
	if err != nil {
		logger.Info("could not get secret: " + err.Error())
		return reconcile.Result{}, nil
	}
	// update caBundle if it doesn't match what's in the secret
	updated := false
	newConfig := currentConfig.DeepCopyObject().(*v1beta1.ValidatingWebhookConfiguration)
	for i := range newConfig.Webhooks {
		updated = common.InjectCABundle(&newConfig.Webhooks[i].ClientConfig, caRoot) || updated
	}

	if updated {
		err := r.Client.Update(context.TODO(), newConfig)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "failed to update CABundle")
		}
		logger.Info("CABundle updated")
		return reconcile.Result{}, nil
	}

	logger.Info("Correct CABundle already present. Ignoring")
	return reconcile.Result{}, nil
}
