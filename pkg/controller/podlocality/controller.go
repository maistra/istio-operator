package podlocality

import (
	"context"
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/maistra/istio-operator/pkg/controller/common"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	apimachineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "podlocality-controller"

	// NodeRegionLabel is the well-known label for kubernetes node region
	NodeRegionLabel = "failure-domain.beta.kubernetes.io/region"
	// NodeZoneLabel is the well-known label for kubernetes node zone
	NodeZoneLabel = "failure-domain.beta.kubernetes.io/zone"
	// IstioSidecarStatusAnnotation is the annotation Istio adds to the pod when the sidecar is injected
	IstioSidecarStatusAnnotation = "sidecar.istio.io/status"
)

// Add creates a new PodLocality Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr.GetClient(), mgr.GetScheme()))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(cl client.Client, scheme *runtime.Scheme) *PodLocalityReconciler {
	return &PodLocalityReconciler{ControllerResources: common.ControllerResources{
		Client:       cl,
		Scheme:       scheme,
		PatchFactory: common.NewPatchFactory(cl),
		Log:          logf.Log.WithName(controllerName)}}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *PodLocalityReconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &v1.Pod{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		CreateFunc: func(evt event.CreateEvent) bool {
			return hasSidecarAnnotation(evt.Object) && hasNode(evt.Object)
		},
		DeleteFunc: func(evt event.DeleteEvent) bool { return false },
		UpdateFunc: func(evt event.UpdateEvent) bool {
			return hasSidecarAnnotation(evt.ObjectNew) && hasNode(evt.ObjectNew) && !hasNode(evt.ObjectOld)
		},
		GenericFunc: func(evt event.GenericEvent) bool { return false },
	})
	if err != nil {
		return err
	}

	err = mgr.GetFieldIndexer().IndexField(&v1.Pod{}, "spec.nodeName", func(obj apimachineryruntime.Object) []string {
		pod := obj.(*v1.Pod)
		return []string{pod.Spec.NodeName}
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &v1.Node{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			list := &v1.PodList{}
			err := mgr.GetClient().List(context.TODO(), client.MatchingField("spec.nodeName", a.Meta.GetName()), list)
			if err != nil {
				r.Log.Error(err, "Could not list pods")
			}

			var requests []reconcile.Request
			for _, pod := range list.Items {
				if podHasSidecar(pod) {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      pod.Name,
							Namespace: pod.Namespace,
						},
					})
				}
			}
			return requests
		}),
	}, predicate.Funcs{
		CreateFunc: func(evt event.CreateEvent) bool { return evt.Meta != nil },
		DeleteFunc: func(evt event.DeleteEvent) bool { return false },
		UpdateFunc: func(evt event.UpdateEvent) bool {
			return evt.MetaOld != nil && evt.MetaNew != nil &&
				(evt.MetaOld.GetLabels()[NodeRegionLabel] != evt.MetaNew.GetLabels()[NodeRegionLabel] ||
					evt.MetaOld.GetLabels()[NodeZoneLabel] != evt.MetaNew.GetLabels()[NodeZoneLabel])
		},
		GenericFunc: func(evt event.GenericEvent) bool { return false },
	})
	if err != nil {
		return err
	}

	return nil
}

func hasNode(pod runtime.Object) bool {
	return pod != nil && pod.(*v1.Pod).Spec.NodeName != ""
}

func hasSidecarAnnotation(pod runtime.Object) bool {
	return pod != nil && podHasSidecar(*pod.(*v1.Pod))
}

func podHasSidecar(pod v1.Pod) bool {
	return pod.Annotations[IstioSidecarStatusAnnotation] != ""
}

var _ reconcile.Reconciler = &PodLocalityReconciler{}

// PodLocalityReconciler copies the node's region/zone labels to the pod after it's scheduled to a node
type PodLocalityReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	common.ControllerResources
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *PodLocalityReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := r.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Processing Pod")

	// Fetch the Pod
	pod := &v1.Pod{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	if pod.Spec.NodeName == "" {
		// nothing to do. Pod not scheduled yet.
		return reconcile.Result{}, nil
	}

	node := &v1.Node{}
	err = r.Client.Get(context.TODO(), client.ObjectKey{Name: pod.Spec.NodeName}, node)
	if err != nil {
		return reconcile.Result{}, err
	}

	if node.Labels[NodeRegionLabel] == "" && node.Labels[NodeZoneLabel] == "" {
		reqLogger.Info("The node the pod is scheduled on neither has the region nor zone labels. Nothing to do.")
		return reconcile.Result{}, nil
	}

	if pod.Labels[NodeRegionLabel] == node.Labels[NodeRegionLabel] && pod.Labels[NodeZoneLabel] == node.Labels[NodeZoneLabel] {
		reqLogger.Info("Pod's locality labels match the node's. Nothing to do.")
		return reconcile.Result{}, nil
	}

	pod.Labels[NodeRegionLabel] = node.Labels[NodeRegionLabel]
	pod.Labels[NodeZoneLabel] = node.Labels[NodeZoneLabel]

	err = r.Client.Update(context.TODO(), pod)
	if err != nil {
		reqLogger.Info(fmt.Sprintf("Error updating pod's labels: %v", err))
		return reconcile.Result{}, err
	}

	reqLogger.Info("Successfully added zone and region labels to pod.")
	return reconcile.Result{}, nil
}
