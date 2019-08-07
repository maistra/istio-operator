package controlplane

import (
	"context"
	"fmt"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/bootstrap"
	"github.com/maistra/istio-operator/pkg/controller/common"

	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_servicemeshcontrolplane")

const (
	finalizer      = "istio-operator-ControlPlane"
	controllerName = "servicemeshcontrolplane-controller"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ControlPlane Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	operatorNamespace, err := common.GetOperatorNamespace()
	if err != nil {
		return err
	}

	err = common.InitCNIStatus(mgr)
	if err != nil {
		return err
	}

	return add(mgr, newReconciler(mgr, operatorNamespace))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, operatorNamespace string) reconcile.Reconciler {
	return &ReconcileControlPlane{
		ResourceManager: common.ResourceManager{
			Client:            mgr.GetClient(),
			PatchFactory:      common.NewPatchFactory(mgr.GetClient()),
			Log:               log,
			OperatorNamespace: operatorNamespace,
		},
		Scheme:  mgr.GetScheme(),
		Manager: mgr,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ServiceMeshControlPlane
	err = c.Watch(&source.Kind{Type: &v1.ServiceMeshControlPlane{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// watch created resources for use in synchronizing ready status
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v1.ServiceMeshControlPlane{},
		},
		ownedResourcePredicates)
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v1.ServiceMeshControlPlane{},
		},
		ownedResourcePredicates)
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v1.ServiceMeshControlPlane{},
		},
		ownedResourcePredicates)
	if err != nil {
		return err
	}

	return nil
}

var ownedResourcePredicates = predicate.Funcs{
	CreateFunc: func(_ event.CreateEvent) bool {
		// we don't need to update status on create events
		return false
	},
	GenericFunc: func(_ event.GenericEvent) bool {
		// we don't need to update status on generic events
		return false
	},
}

var _ reconcile.Reconciler = &ReconcileControlPlane{}

// ReconcileControlPlane reconciles a ServiceMeshControlPlane object
type ReconcileControlPlane struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	common.ResourceManager
	Scheme  *runtime.Scheme
	Manager manager.Manager
}

// Reconcile reads that state of the cluster for a ServiceMeshControlPlane object and makes changes based on the state read
// and what is in the ServiceMeshControlPlane.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileControlPlane) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Processing ServiceMeshControlPlane")
	defer func() {
		reqLogger.Info("Completed ServiceMeshControlPlane processing")
	}()

	// Fetch the ServiceMeshControlPlane instance
	instance := &v1.ServiceMeshControlPlane{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("ServiceMeshControlPlane deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	reconciler := r.getOrCreateReconciler(instance)
	defer r.deleteReconciler(reconciler)
	deleted := instance.GetDeletionTimestamp() != nil
	finalizers := instance.GetFinalizers()
	finalizerIndex := common.IndexOf(finalizers, finalizer)

	if deleted {
		if finalizerIndex < 0 {
			reqLogger.Info("Deletion of ServiceMeshControlPlane complete")
			return reconcile.Result{}, nil
		}
		reqLogger.Info("Deleting ServiceMeshControlPlane")
		result, err := reconciler.Delete()
		// XXX: for now, nuke the resources, regardless of errors
		finalizers = append(finalizers[:finalizerIndex], finalizers[finalizerIndex+1:]...)
		instance.SetFinalizers(finalizers)
		finalizerError := r.Client.Update(context.TODO(), instance)
		for retryCount := 0; errors.IsConflict(finalizerError) && retryCount < 5; retryCount++ {
			reqLogger.Info("conflict during finalizer removal, retrying")
			err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
			if err != nil {
				reqLogger.Error(err, "Could not get ServiceMeshControlPlane")
				continue
			}
			finalizers = instance.GetFinalizers()
			finalizerIndex = common.IndexOf(finalizers, finalizer)
			finalizers = append(finalizers[:finalizerIndex], finalizers[finalizerIndex+1:]...)
			instance.SetFinalizers(finalizers)
			finalizerError = r.Client.Update(context.TODO(), instance)
		}
		if finalizerError != nil {
			reqLogger.Error(finalizerError, "error removing finalizer")
			r.Manager.GetRecorder(controllerName).Event(instance, "Warning", "ServiceMeshDeleted", fmt.Sprintf("Error occurred removing finalizer from service mesh: %s", finalizerError))
		}
		return result, err
	} else if finalizerIndex < 0 {
		reqLogger.V(1).Info("Adding finalizer", "finalizer", finalizer)
		finalizers = append(finalizers, finalizer)
		instance.SetFinalizers(finalizers)
		err = r.Client.Update(context.TODO(), instance)
		return reconcile.Result{}, err
	}

	if instance.GetGeneration() == instance.Status.ObservedGeneration &&
		instance.Status.GetCondition(v1.ConditionTypeReconciled).Status == v1.ConditionStatusTrue {
		// sync readiness state
		return reconciler.UpdateReadiness()
	}

	reqLogger.Info("Reconciling ServiceMeshControlPlane")

	if instance.Status.GetCondition(v1.ConditionTypeReconciled).Status != v1.ConditionStatusFalse {
		var readyMessage string
		if instance.Status.ObservedGeneration == 0 {
			readyMessage = fmt.Sprintf("Installing mesh generation %d", instance.GetGeneration())
			r.Manager.GetRecorder(controllerName).Event(instance, "Normal", "CreatingServiceMesh", readyMessage)
		} else {
			readyMessage = fmt.Sprintf("Updating mesh from generation %d to generation %d", instance.Status.ObservedGeneration, instance.GetGeneration())
			r.Manager.GetRecorder(controllerName).Event(instance, "Normal", "UpdatingServiceMesh", readyMessage)
		}
		instance.Status.SetCondition(v1.Condition{
			Type:    v1.ConditionTypeReconciled,
			Status:  v1.ConditionStatusFalse,
			Message: readyMessage,
		})
		instance.Status.SetCondition(v1.Condition{
			Type:    v1.ConditionTypeReady,
			Status:  v1.ConditionStatusFalse,
			Message: readyMessage,
		})
		err = reconciler.PostStatus()
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Ensure Istio CNI is installed
	if common.IsCNIEnabled {
		err = bootstrap.InstallCNI(reconciler.Manager)
		if err != nil {
			reqLogger.Error(err, "Failed to install/update Istio CNI")
			return reconcile.Result{}, err
		}
	}

	// Ensure CRDs are installed
	err = bootstrap.InstallCRDs(reconciler.Manager)
	if err != nil {
		reqLogger.Error(err, "Failed to install/update Istio CRDs")
		return reconcile.Result{}, err
	}

	return reconciler.Reconcile()
}

var reconcilers = map[string]*ControlPlaneReconciler{}

func reconcilersMapKey(instance *v1.ServiceMeshControlPlane) string {
	return fmt.Sprintf("%s/%s", instance.GetNamespace(), instance.GetName())
}

func (r *ReconcileControlPlane) getOrCreateReconciler(instance *v1.ServiceMeshControlPlane) *ControlPlaneReconciler {
	key := reconcilersMapKey(instance)
	if existing, ok := reconcilers[key]; ok {
		if existing.Instance.GetGeneration() != instance.GetGeneration() {
			// we need to regenerate the renderings
			existing.renderings = nil
			var readyMessage string
			if instance.Status.ObservedGeneration == 0 {
				readyMessage = fmt.Sprintf("Installing mesh generation %d", instance.GetGeneration())
				r.Manager.GetRecorder(controllerName).Event(instance, "Normal", "CreatingServiceMesh", readyMessage)
			} else {
				readyMessage = fmt.Sprintf("Updating mesh from generation %d to generation %d", instance.Status.ObservedGeneration, instance.GetGeneration())
				r.Manager.GetRecorder(controllerName).Event(instance, "Normal", "UpdatingServiceMesh", readyMessage)
			}
			instance.Status.SetCondition(v1.Condition{
				Type:    v1.ConditionTypeReady,
				Status:  v1.ConditionStatusFalse,
				Message: readyMessage,
			})
			// ignore error.  instance already has ready status. it will just have a stale message
			_ = existing.PostStatus()
		}
		existing.Instance = instance
		return existing
	}
	newReconciler := &ControlPlaneReconciler{
		ReconcileControlPlane: r,
		Instance:              instance,
		Status:                v1.NewControlPlaneStatus(),
	}

	reconcilers[key] = newReconciler
	return newReconciler
}

func (r *ReconcileControlPlane) getReconciler(instance *v1.ServiceMeshControlPlane) *ControlPlaneReconciler {
	if existing, ok := reconcilers[reconcilersMapKey(instance)]; ok {
		return existing
	}
	return nil
}

func (r *ReconcileControlPlane) deleteReconciler(reconciler *ControlPlaneReconciler) {
	if reconciler == nil {
		return
	}
	if reconciler.Instance.Status.GetCondition(v1.ConditionTypeReconciled).Status == v1.ConditionStatusTrue {
		delete(reconcilers, reconcilersMapKey(reconciler.Instance))
	}
}
