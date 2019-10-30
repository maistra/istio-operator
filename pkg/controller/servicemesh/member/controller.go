package member

import (
	"context"

	errors2 "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "servicemeshmember-controller"
)

var log = logf.Log.WithName("controller_member")

// Add creates a new ServiceMeshMember Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &MemberReconciler{ResourceManager: common.ResourceManager{Client: mgr.GetClient(), PatchFactory: common.NewPatchFactory(mgr.GetClient()), Log: log}, scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("servicemeshmember-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ServiceMeshMember
	err = c.Watch(&source.Kind{Type: &v1.ServiceMeshMember{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = mgr.GetFieldIndexer().IndexField(&v1.ServiceMeshMember{}, "spec.controlPlaneRef.namespace", func(obj runtime.Object) []string {
		roll := obj.(*v1.ServiceMeshMember)
		return []string{roll.Spec.ControlPlaneRef.Namespace}
	})
	if err != nil {
		return err
	}

	// watch namespaces so we can create the SMMR when the control plane namespace is created
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(ns handler.MapObject) []reconcile.Request {
			return getRequestsForMembersReferencing(ns.Meta.GetName(), mgr.GetClient())
		}),
	}, predicate.Funcs{
		UpdateFunc: func(_ event.UpdateEvent) bool {
			// we don't need to process the member on namespace updates
			return false
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			// we don't need to process the member on namespace deletions
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			// we don't need to process the member on generic namespace events
			return false
		},
	})
	if err != nil {
		return err
	}

	// watch member rolls to revert any incompatible changes users make (e.g. user removes a member namespace, but the Member object is still there)
	err = c.Watch(&source.Kind{Type: &v1.ServiceMeshMemberRoll{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(smmr handler.MapObject) []reconcile.Request {
			return getRequestsForMembersReferencing(smmr.Meta.GetNamespace(), mgr.GetClient())
		}),
	})
	if err != nil {
		return err
	}

	return nil
}

func getRequestsForMembersReferencing(ns string, cl client.Client) []reconcile.Request {
	list := &v1.ServiceMeshMemberList{}
	err := cl.List(context.TODO(), client.MatchingField("spec.controlPlaneRef.namespace", ns), list)
	if err != nil {
		log.Error(err, "Could not list ServiceMeshMembers")
	}

	var requests []reconcile.Request
	for _, smm := range list.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      smm.Name,
				Namespace: smm.Namespace,
			},
		})
	}
	return requests
}

var _ reconcile.Reconciler = &MemberReconciler{}

// MemberReconciler reconciles ServiceMeshMember objects
type MemberReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	common.ResourceManager
	scheme *runtime.Scheme
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *MemberReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("ServiceMeshMember", request)
	reqLogger.Info("Processing ServiceMeshMember")

	member := &v1.ServiceMeshMember{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, member)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	memberRollKey := client.ObjectKey{
		Name:      common.MemberRollName,
		Namespace: member.Spec.ControlPlaneRef.Namespace,
	}
	memberRoll := &v1.ServiceMeshMemberRoll{}
	isNewMemberRoll := false
	err = r.Client.Get(context.TODO(), memberRollKey, memberRoll)
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		// MemberRoll doesn't exist, let's create it
		isNewMemberRoll = true
		memberRoll = &v1.ServiceMeshMemberRoll{
			ObjectMeta: v12.ObjectMeta{
				Name:      common.MemberRollName,
				Namespace: member.Spec.ControlPlaneRef.Namespace,
				Annotations: map[string]string{
					common.CreatedByKey: controllerName,
				},
			},
		}
	}

	deleted := member.GetDeletionTimestamp() != nil
	finalizers := sets.NewString(member.Finalizers...)
	if deleted {
		if !finalizers.Has(common.FinalizerName) {
			reqLogger.Info("ServiceMeshMember deleted")
			return reconcile.Result{}, nil
		}

		if !isNewMemberRoll {
			reqLogger.Info("Removing ServiceMeshMember from ServiceMeshMemberRoll")
			for i, m := range memberRoll.Spec.Members {
				if m == member.Namespace {
					memberRoll.Spec.Members = append(memberRoll.Spec.Members[:i], memberRoll.Spec.Members[i+1:]...)
					break
				}
			}

			memberRollCreatedByThisController := memberRoll.Annotations[common.CreatedByKey] == controllerName
			if len(memberRoll.Spec.Members) == 0 && memberRollCreatedByThisController {
				err = r.Client.Delete(context.TODO(), memberRoll)
				if err != nil {
					return reconcile.Result{}, errors2.Wrapf(err, "Could not delete ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
				}
			} else {
				err = r.Client.Update(context.TODO(), memberRoll)
				if err != nil {
					return reconcile.Result{}, errors2.Wrapf(err, "Could not update ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
				}
			}
		}

		reqLogger.Info("Removing finalizer from ServiceMeshMember")
		finalizers.Delete(common.FinalizerName)
		member.SetFinalizers(finalizers.List())
		err = r.Client.Update(context.TODO(), member)
		if err != nil {
			return reconcile.Result{}, errors2.Wrapf(err, "Could not update ServiceMeshMember %s/%s when removing finalizer", member.Namespace, member.Name)
		}

		return reconcile.Result{}, nil

	} else if !finalizers.Has(common.FinalizerName) {
		reqLogger.Info("Adding finalizer to ServiceMeshMember", "finalizer", common.FinalizerName)
		finalizers.Insert(common.FinalizerName)
		member.SetFinalizers(finalizers.List())
		err = r.Client.Update(context.TODO(), member)
		return reconcile.Result{}, err
	}

	if !contains(member.Namespace, memberRoll.Spec.Members) {
		memberRoll.Spec.Members = append(memberRoll.Spec.Members, member.Namespace)

		if isNewMemberRoll {
			err = r.Client.Create(context.TODO(), memberRoll)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info("Could not create ServiceMeshMemberRoll, because the referenced namespace doesn't exist", "namespace", memberRoll.Namespace)
					return reconcile.Result{}, nil
				}
				return reconcile.Result{}, errors2.Wrapf(err, "Could not create ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
			}
		} else {
			err = r.Client.Update(context.TODO(), memberRoll)
			if err != nil {
				return reconcile.Result{}, errors2.Wrapf(err, "Could not update ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
			}
		}
	}

	err = r.Client.Update(context.TODO(), member)
	if err != nil {
		return reconcile.Result{}, errors2.Wrapf(err, "Could not update ServiceMeshMember %s/%s", member.Namespace, member.Name)
	}
	return reconcile.Result{}, nil
}

func contains(needle string, haystack []string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
