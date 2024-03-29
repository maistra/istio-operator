package podlocality

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"gomodules.xyz/jsonpatch/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

const (
	controllerName = "podlocality-controller"

	// NodeRegionLabel is the well-known label for kubernetes node region
	NodeRegionLabel = "failure-domain.beta.kubernetes.io/region"
	// NodeZoneLabel is the well-known label for kubernetes node zone
	NodeZoneLabel = "failure-domain.beta.kubernetes.io/zone"
	// NodeRegionLabelGA is the well-known label for kubernetes node region in ga
	NodeRegionLabelGA = "topology.kubernetes.io/region"
	// NodeZoneLabelGA is the well-known label for kubernetes node zone in ga
	NodeZoneLabelGA = "topology.kubernetes.io/zone"
	// IstioSubzoneLabel is custom subzone label for locality-based routing in Kubernetes see: https://github.com/istio/istio/issues/19114
	IstioSubzoneLabel = "topology.istio.io/subzone"
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
	return &PodLocalityReconciler{
		ControllerResources: common.ControllerResources{
			Client: cl,
			Scheme: scheme,
		},
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *PodLocalityReconciler) error {
	log := createLogger()
	ctx := common.NewContextWithLog(common.NewContext(), log)

	// Create a new controller
	wrappedReconciler := common.NewConflictHandlingReconciler(r)
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: wrappedReconciler})
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

	err = mgr.GetFieldIndexer().IndexField(ctx, &v1.Pod{}, "spec.nodeName", func(obj runtime.Object) []string {
		pod := obj.(*v1.Pod)
		return []string{pod.Spec.NodeName}
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &v1.Node{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			list := &v1.PodList{}
			err := mgr.GetClient().List(ctx, list, client.MatchingFields{"spec.nodeName": a.Meta.GetName()})
			if err != nil {
				log.Error(err, "Could not list pods")
			}

			var requests []reconcile.Request
			for _, pod := range list.Items {
				if podHasSidecar(pod) {
					requests = append(requests, reconcile.Request{
						NamespacedName: common.ToNamespacedName(&pod),
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
				!topologyLabelsMatch(evt.MetaOld.GetLabels(), evt.MetaNew.GetLabels())
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

type podTopologyLabels struct {
	NodeRegionLabel   string
	NodeZoneLabel     string
	NodeRegionLabelGA string
	NodeZoneLabelGA   string
	IstioSubzoneLabel string
}

type podLabelPatch struct {
	labels podTopologyLabels
}

func newPodLabelPatch(labels podTopologyLabels) *podLabelPatch {
	return &podLabelPatch{
		labels: labels,
	}
}

func (p *podLabelPatch) Type() types.PatchType {
	return types.JSONPatchType
}

func (p *podLabelPatch) Data(obj runtime.Object) ([]byte, error) {
	data := []jsonpatch.Operation{
		{
			Operation: "add",
			Path:      "/metadata/labels/failure-domain.beta.kubernetes.io~1region",
			Value:     p.labels.NodeRegionLabel,
		},
		{
			Operation: "add",
			Path:      "/metadata/labels/failure-domain.beta.kubernetes.io~1zone",
			Value:     p.labels.NodeZoneLabel,
		},
		{
			Operation: "add",
			Path:      "/metadata/labels/topology.kubernetes.io~1region",
			Value:     p.labels.NodeRegionLabelGA,
		},
		{
			Operation: "add",
			Path:      "/metadata/labels/topology.kubernetes.io~1zone",
			Value:     p.labels.NodeZoneLabelGA,
		},
		{
			Operation: "add",
			Path:      "/metadata/labels/topology.istio.io~1subzone",
			Value:     p.labels.IstioSubzoneLabel,
		},
	}
	json, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json, nil
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
	reqLogger := createLogger().WithValues("Pod", request)
	ctx := common.NewReconcileContext(reqLogger)
	reqLogger.Info("Processing Pod")

	// Fetch the Pod
	pod := &v1.Pod{}
	err := r.Client.Get(ctx, request.NamespacedName, pod)
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
	err = r.Client.Get(ctx, client.ObjectKey{Name: pod.Spec.NodeName}, node)
	if err != nil {
		return reconcile.Result{}, err
	}

	if topologyLabelsMatch(node.Labels, map[string]string{}) {
		reqLogger.V(5).Info("The node the pod is scheduled on neither has the region, zone nor subzone labels. Nothing to do.")
		return reconcile.Result{}, nil
	}

	if topologyLabelsMatch(pod.Labels, node.Labels) {
		reqLogger.Info("Pod's locality labels match the node's. Nothing to do.")
		return reconcile.Result{}, nil
	}

	patch := newPodLabelPatch(podTopologyLabels{
		NodeRegionLabel:   node.Labels[NodeRegionLabel],
		NodeZoneLabel:     node.Labels[NodeZoneLabel],
		NodeRegionLabelGA: node.Labels[NodeRegionLabelGA],
		NodeZoneLabelGA:   node.Labels[NodeZoneLabelGA],
		IstioSubzoneLabel: node.Labels[IstioSubzoneLabel],
	})

	err = r.Client.Patch(ctx, pod, patch)
	if err != nil {
		reqLogger.Info(fmt.Sprintf("Error updating pod's labels: %v", err))
		return reconcile.Result{}, err
	}

	reqLogger.Info("Successfully added zone and region labels to pod.")
	return reconcile.Result{}, nil
}

// Don't use this function to obtain a logger. Get it by invoking
// common.LogFromContext(ctx) to ensure that the logger has the
// correct context info and logs it.
func createLogger() logr.Logger {
	return logf.Log.WithName(controllerName)
}

func topologyLabelsMatch(meta1Labels, meta2Labels map[string]string) bool {
	if meta1Labels[NodeRegionLabel] == meta2Labels[NodeRegionLabel] &&
		meta1Labels[NodeZoneLabel] == meta2Labels[NodeZoneLabel] &&
		meta1Labels[NodeRegionLabelGA] == meta2Labels[NodeRegionLabelGA] &&
		meta1Labels[NodeZoneLabelGA] == meta2Labels[NodeZoneLabelGA] &&
		meta1Labels[IstioSubzoneLabel] == meta2Labels[IstioSubzoneLabel] {
		return true
	}
	return false
}
