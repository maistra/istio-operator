package controlplane

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

var log = logf.Log.WithName("controller_servicemeshcontrolplane")

const (
	controllerName = "servicemeshcontrolplane-controller"
)

// Add creates a new ControlPlane Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	operatorNamespace := common.GetOperatorNamespace()
	if err := common.InitCNIStatus(mgr); err != nil {
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
		Scheme:        mgr.GetScheme(),
		EventRecorder: mgr.GetRecorder(controllerName),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	var c controller.Controller
	var err error
	if c, err = controller.New(controllerName, mgr, controller.Options{Reconciler: r}); err != nil {
		return err
	}

	// Watch for changes to primary resource ServiceMeshControlPlane
	if err = c.Watch(&source.Kind{Type: &v1.ServiceMeshControlPlane{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// watch created resources for use in synchronizing ready status
	if err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v1.ServiceMeshControlPlane{},
		},
		ownedResourcePredicates); err != nil {
		return err
	}
	if err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v1.ServiceMeshControlPlane{},
		},
		ownedResourcePredicates); err != nil {
		return err
	}
	if err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v1.ServiceMeshControlPlane{},
		},
		ownedResourcePredicates); err != nil {
		return err
	}

	// add watch for cni daemon set
	operatorNamespace := common.GetOperatorNamespace()
	if err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(obj handler.MapObject) []reconcile.Request {
				if obj.Meta.GetNamespace() != operatorNamespace {
					return nil
				}
				smcpList := &v1.ServiceMeshControlPlaneList{}
				if err := mgr.GetClient().List(context.TODO(), nil, smcpList); err != nil {
					log.Error(err, "error listing ServiceMeshControlPlane objects in CNI DaemonSet watcher")
					return nil
				}
				requests := make([]reconcile.Request, 0, len(smcpList.Items))
				for _, smcp := range smcpList.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      smcp.Name,
							Namespace: smcp.Namespace,
						},
					})
				}
				return requests
			}),
		},
		ownedResourcePredicates); err != nil {
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
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a ServiceMeshControlPlane object and makes changes based on the state read
// and what is in the ServiceMeshControlPlane.Spec
func (r *ReconcileControlPlane) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("ServiceMeshControlPlane", request)
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
	finalizers := sets.NewString(instance.Finalizers...)

	if deleted {
		if !finalizers.Has(common.FinalizerName) {
			reqLogger.Info("Deletion of ServiceMeshControlPlane complete")
			return reconcile.Result{}, nil
		}

		err := reconciler.Delete()
		return reconcile.Result{}, err
	} else if !finalizers.Has(common.FinalizerName) {
		reqLogger.V(1).Info("Adding finalizer", "finalizer", common.FinalizerName)
		finalizers.Insert(common.FinalizerName)
		instance.SetFinalizers(finalizers.List())
		err = r.Client.Update(context.TODO(), instance)
		return reconcile.Result{}, err
	}

	if common.ReconciledVersion(instance.GetGeneration()) == instance.Status.GetReconciledVersion() &&
		instance.Status.GetCondition(v1.ConditionTypeReconciled).Status == v1.ConditionStatusTrue {
		// sync readiness state
		err := reconciler.UpdateReadiness()
		return reconcile.Result{}, err
	}

	reqLogger.Info(fmt.Sprintf("Reconciling ServiceMeshControlPlane: %v", instance.Status.StatusType))

	return reconciler.Reconcile()
}

var reconcilers = map[string]*ControlPlaneReconciler{}

func reconcilersMapKey(instance *v1.ServiceMeshControlPlane) string {
	return fmt.Sprintf("%s/%s", instance.GetNamespace(), instance.GetName())
}

func (r *ReconcileControlPlane) getOrCreateReconciler(newInstance *v1.ServiceMeshControlPlane) *ControlPlaneReconciler {
	key := reconcilersMapKey(newInstance)
	if existing, ok := reconcilers[key]; ok {
		oldInstance := existing.Instance
		existing.Instance = newInstance
		if existing.Instance.GetGeneration() != oldInstance.GetGeneration() {
			// we need to regenerate the renderings
			existing.renderings = nil
			existing.lastComponent = ""
			// reset reconcile status
			existing.Status.SetCondition(v1.Condition{Type: v1.ConditionTypeReconciled, Status: v1.ConditionStatusUnknown})
		}
		return existing
	}
	newReconciler := &ControlPlaneReconciler{
		ReconcileControlPlane: r,
		Instance:              newInstance,
		Status:                newInstance.Status.DeepCopy(),
	}
	reconcilers[key] = newReconciler
	return newReconciler
}

func (r *ReconcileControlPlane) deleteReconciler(reconciler *ControlPlaneReconciler) {
	if reconciler == nil {
		return
	}
	if reconciler.Status.GetCondition(v1.ConditionTypeReconciled).Status == v1.ConditionStatusTrue {
		delete(reconcilers, reconcilersMapKey(reconciler.Instance))
	}
}
