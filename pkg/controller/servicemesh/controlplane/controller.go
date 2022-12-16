package controlplane

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/hacks"
)

const (
	controllerName = "servicemeshcontrolplane-controller"
)

// Add creates a new ControlPlane Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	operatorNamespace := common.GetOperatorNamespace()
	cniConfig, err := cni.InitConfig(mgr)
	if err != nil {
		return err
	}

	dcProvider, ok := mgr.(common.DiscoveryClientProvider)
	if !ok {
		return fmt.Errorf("expected mgr to be a DiscoveryClientProvider")
	}
	dc, err := dcProvider.GetDiscoveryClient()
	if err != nil {
		return err
	}

	reconciler := newReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetEventRecorderFor(controllerName), operatorNamespace, cniConfig, dc)
	return add(mgr, reconciler)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(cl client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder, operatorNamespace string,
	cniConfig cni.Config, dc discovery.DiscoveryInterface,
) *ControlPlaneReconciler {
	reconciler := &ControlPlaneReconciler{
		ControllerResources: common.ControllerResources{
			Client:            cl,
			Scheme:            scheme,
			EventRecorder:     eventRecorder,
			OperatorNamespace: operatorNamespace,
			DiscoveryClient:   dc,
		},
		cniConfig:                   cniConfig,
		earliestReconciliationTimes: map[types.NamespacedName]time.Time{},
		reconcilers:                 map[types.NamespacedName]ControlPlaneInstanceReconciler{},
	}
	reconciler.instanceReconcilerFactory = NewControlPlaneInstanceReconciler
	return reconciler
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ControlPlaneReconciler) error {
	log := createLogger()
	ctx := common.NewContextWithLog(common.NewContext(), log)

	wrappedReconciler := common.NewConflictHandlingReconciler(r)
	// Create a new controller
	var c controller.Controller
	var err error
	if c, err = controller.New(controllerName, mgr,
		controller.Options{
			MaxConcurrentReconciles: common.Config.Controller.ControlPlaneReconcilers,
			Reconciler:              wrappedReconciler,
		}); err != nil {
		return err
	}

	// Watch for changes to primary resource ServiceMeshControlPlane
	if err = c.Watch(&source.Kind{Type: &v2.ServiceMeshControlPlane{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// watch created resources for use in synchronizing ready status
	if err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, enqueueRequestForSMCP, ownedResourcePredicates); err != nil {
		return err
	}
	if err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, enqueueRequestForSMCP, ownedResourcePredicates); err != nil {
		return err
	}
	if err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}}, enqueueRequestForSMCP, ownedResourcePredicates); err != nil {
		return err
	}

	// add watch for cni daemon set
	operatorNamespace := common.GetOperatorNamespace()
	if err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}},
		handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
			if obj.GetNamespace() != operatorNamespace {
				return nil
			}
			smcpList := &v2.ServiceMeshControlPlaneList{}
			if err := mgr.GetClient().List(ctx, smcpList); err != nil {
				log.Error(err, "error listing ServiceMeshControlPlane objects in CNI DaemonSet watcher")
				return nil
			}
			requests := make([]reconcile.Request, 0, len(smcpList.Items))
			for _, smcp := range smcpList.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: common.ToNamespacedName(&smcp),
				})
			}
			return requests
		}),
		ownedResourcePredicates); err != nil {
		return err
	}

	return nil
}

var enqueueRequestForSMCP = handler.EnqueueRequestsFromMapFunc(
	func(obj client.Object) []reconcile.Request {
		labels := obj.GetLabels()
		if labels[common.KubernetesAppManagedByKey] == common.KubernetesAppManagedByValue {
			ownerNamespace := labels[common.OwnerKey]
			ownerName := labels[common.OwnerNameKey]
			if ownerNamespace != "" && ownerName != "" {
				return []reconcile.Request{{NamespacedName: types.NamespacedName{
					Namespace: ownerNamespace,
					Name:      ownerName,
				}}}
			}
		}
		return nil
	})

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

var _ reconcile.Reconciler = &ControlPlaneReconciler{}

// ControlPlaneReconciler handles reconciliation of ServiceMeshControlPlane
// objects. It creates a ControlPlaneInstanceReconciler for each instance.
type ControlPlaneReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	common.ControllerResources
	cniConfig cni.Config

	earliestReconciliationTimes map[types.NamespacedName]time.Time
	reconcilers                 map[types.NamespacedName]ControlPlaneInstanceReconciler
	mu                          sync.Mutex

	instanceReconcilerFactory func(common.ControllerResources, *v2.ServiceMeshControlPlane, cni.Config) ControlPlaneInstanceReconciler
}

// ControlPlaneInstanceReconciler reconciles a specific instance of a ServiceMeshControlPlane
type ControlPlaneInstanceReconciler interface {
	Reconcile(ctx context.Context) (reconcile.Result, error)
	UpdateReadiness(ctx context.Context) error
	PatchAddons(ctx context.Context, spec *v2.ControlPlaneSpec) (reconcile.Result, error)
	Delete(ctx context.Context) error
	SetInstance(instance *v2.ServiceMeshControlPlane)
	IsFinished() bool
}

// Reconcile reads that state of the cluster for a ServiceMeshControlPlane object and makes changes based on the state read
// and what is in the ServiceMeshControlPlane.Spec
func (r *ControlPlaneReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := createLogger().WithValues("ServiceMeshControlPlane", request)

	if earliestReconciliationTime, ok := r.earliestReconciliationTimes[request.NamespacedName]; ok {
		if earliestReconciliationTime.After(time.Now()) {
			log.Info("Skipping reconciliation of ServiceMeshControlPlane because reconciliation is paused")
			return reconcile.Result{
				Requeue:      true,
				RequeueAfter: time.Until(earliestReconciliationTime),
			}, nil
		}
		delete(r.earliestReconciliationTimes, request.NamespacedName)
	}
	ctx = hacks.WrapContext(ctx, r.earliestReconciliationTimes)

	log.Info("Processing ServiceMeshControlPlane")
	defer func() {
		log.Info("Completed ServiceMeshControlPlane processing")
	}()

	// Fetch the ServiceMeshControlPlane instance
	instance := &v2.ServiceMeshControlPlane{}
	err := r.Client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("ServiceMeshControlPlane deleted")
			delete(r.earliestReconciliationTimes, request.NamespacedName)
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	key, reconciler := r.getOrCreateReconciler(instance)
	defer r.deleteReconcilerIfFinished(key, reconciler)

	deleted := instance.GetDeletionTimestamp() != nil
	finalizers := sets.NewString(instance.Finalizers...)

	if deleted {
		if !finalizers.Has(common.FinalizerName) {
			log.Info("Deletion of ServiceMeshControlPlane complete")
			return reconcile.Result{}, nil
		}
		err := reconciler.Delete(ctx)
		return reconcile.Result{}, err
	} else if !finalizers.Has(common.FinalizerName) {
		log.V(1).Info("Adding finalizer", "finalizer", common.FinalizerName)
		finalizers.Insert(common.FinalizerName)
		instance.SetFinalizers(finalizers.List())
		err = r.Client.Update(ctx, instance)
		return reconcile.Result{}, err
	}

	if isFullyReconciled(instance) {
		if err := reconciler.UpdateReadiness(ctx); err != nil {
			return common.RequeueWithError(err)
		}
		return reconciler.PatchAddons(ctx, &instance.Spec)
	}

	return reconciler.Reconcile(ctx)
}

func isFullyReconciled(instance *v2.ServiceMeshControlPlane) bool {
	return status.CurrentReconciledVersion(instance.GetGeneration()) == instance.Status.GetReconciledVersion() &&
		instance.Status.GetCondition(status.ConditionTypeReconciled).Status == status.ConditionStatusTrue
}

func (r *ControlPlaneReconciler) getOrCreateReconciler(newInstance *v2.ServiceMeshControlPlane) (types.NamespacedName, ControlPlaneInstanceReconciler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := common.ToNamespacedName(newInstance)
	if reconciler, ok := r.reconcilers[key]; ok {
		reconciler.SetInstance(newInstance)
		return key, reconciler
	}
	newReconciler := r.instanceReconcilerFactory(r.ControllerResources, newInstance, r.cniConfig)
	r.reconcilers[key] = newReconciler
	return key, newReconciler
}

func (r *ControlPlaneReconciler) deleteReconcilerIfFinished(key types.NamespacedName, reconciler ControlPlaneInstanceReconciler) {
	if reconciler == nil {
		return
	}
	if reconciler.IsFinished() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.reconcilers, key)
	}
}

// Don't use this function to obtain a logger. Get it by invoking
// common.LogFromContext(ctx) to ensure that the logger has the
// correct context info and logs it.
func createLogger() logr.Logger {
	return logf.Log.WithName(controllerName)
}
