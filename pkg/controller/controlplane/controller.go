package controlplane

import (
	"context"

	istiov1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/v1alpha3"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

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

var log = logf.Log.WithName("controller_controlplane")

const watchNamespace = "istio-system"

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ControlPlane Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileControlPlane{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("controlplane-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ControlPlane
	// XXX: hack: remove predicate once old installation mechanism is removed
	err = c.Watch(&source.Kind{Type: &istiov1alpha3.ControlPlane{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(evt event.CreateEvent) bool { return evt.Meta != nil && evt.Meta.GetNamespace() == watchNamespace },
		DeleteFunc: func(evt event.DeleteEvent) bool { return evt.Meta != nil && evt.Meta.GetNamespace() == watchNamespace },
		UpdateFunc: func(evt event.UpdateEvent) bool {
			return evt.MetaNew != nil && evt.MetaNew.GetNamespace() == watchNamespace
		},
		GenericFunc: func(evt event.GenericEvent) bool { return evt.Meta != nil && evt.Meta.GetNamespace() == watchNamespace },
	})
	if err != nil {
		return err
	}

	// XXX: consider adding watches on created resources.  This would need to be
	// done in the reconciler, although I suppose we could hard code known types
	// (ServiceAccount, Service, ClusterRole, ClusterRoleBinding, Deployment,
	// ConfigMap, ValidatingWebhook, MutatingWebhook, MeshPolicy, DestinationRule,
	// Gateway, PodDisruptionBudget, HorizontalPodAutoscaler, Ingress, Route).
	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner ControlPlane
	// err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
	// 	IsController: true,
	// 	OwnerType:    &istiov1alpha3.ControlPlane{},
	// })
	// if err != nil {
	// 	return err
	// }

	return nil
}

var _ reconcile.Reconciler = &ReconcileControlPlane{}

// ReconcileControlPlane reconciles a ControlPlane object
type ReconcileControlPlane struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

const (
	finalizer = "istio-operator-ControlPlane"
)

// Reconcile reads that state of the cluster for a ControlPlane object and makes changes based on the state read
// and what is in the ControlPlane.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileControlPlane) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Processing ControlPlane")

	// Fetch the ControlPlane instance
	instance := &istiov1alpha3.ControlPlane{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	if instance.GetGeneration() == instance.Status.ObservedGeneration &&
		instance.Status.GetCondition(istiov1alpha3.ConditionTypeInstalled).Status == istiov1alpha3.ConditionStatusTrue {
		reqLogger.Info("nothing to reconcile, generations match")
		return reconcile.Result{}, nil
	}

	reconciler := controlPlaneReconciler{
		ReconcileControlPlane: r,
		instance:              instance,
		log:                   reqLogger,
		status:                istiov1alpha3.NewControlPlaneStatus(),
	}

	deleted := instance.GetDeletionTimestamp() != nil
	finalizers := instance.GetFinalizers()
	finalizerIndex := indexOf(finalizers, finalizer)

	if deleted {
		if finalizerIndex < 0 {
			return reconcile.Result{}, nil
		}
		reqLogger.Info("Deleting ControlPlane")
		result, err := reconciler.Delete()
		// XXX: for now, nuke the resources, regardless of errors
		finalizers = append(finalizers[:finalizerIndex], finalizers[finalizerIndex+1:]...)
		instance.SetFinalizers(finalizers)
		_ = r.client.Update(context.TODO(), instance)
		return result, err
	} else if finalizerIndex < 0 {
		reqLogger.V(1).Info("Adding finalizer", "finalizer", finalizer)
		finalizers = append(finalizers, finalizer)
		instance.SetFinalizers(finalizers)
		err = r.client.Update(context.TODO(), instance)
		return reconcile.Result{}, err
	}

	reqLogger.Info("Reconciling ControlPlane")

	return reconciler.Reconcile()
}

func indexOf(l []string, s string) int {
	for i, elem := range l {
		if elem == s {
			return i
		}
	}
	return -1
}
