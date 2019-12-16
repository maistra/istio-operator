package member

import (
	"context"
	"reflect"

	errors2 "github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	maistra "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "servicemeshmember-controller"

	eventReasonFailedReconcile     maistra.ConditionReason = "FailedReconcile"
	eventReasonSuccessfulReconcile maistra.ConditionReason = "Reconciled"
)

var log = logf.Log.WithName("controller_member")

// Add creates a new ServiceMeshMember Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *MemberReconciler {
	return &MemberReconciler{
		ResourceManager: common.ResourceManager{
			Client:       mgr.GetClient(),
			PatchFactory: common.NewPatchFactory(mgr.GetClient()),
			Log:          log,
		},
		scheme:        mgr.GetScheme(),
		eventRecorder: mgr.GetRecorder(controllerName),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *MemberReconciler) error {
	// Create a new controller
	c, err := controller.New("servicemeshmember-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ServiceMeshMember
	err = c.Watch(&source.Kind{Type: &maistra.ServiceMeshMember{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
		UpdateFunc: func(event event.UpdateEvent) bool {
			o := event.MetaOld
			n := event.MetaNew
			return o.GetResourceVersion() == n.GetResourceVersion() || // ensure reconciliation is triggered by periodic resyncs
				o.GetGeneration() != n.GetGeneration() ||
				!reflect.DeepEqual(o.GetAnnotations(), n.GetAnnotations()) ||
				!reflect.DeepEqual(o.GetFinalizers(), n.GetFinalizers()) ||
				!reflect.DeepEqual(o.GetDeletionTimestamp(), n.GetDeletionTimestamp())
		},
	})
	if err != nil {
		return err
	}

	err = mgr.GetFieldIndexer().IndexField(&maistra.ServiceMeshMember{}, "spec.controlPlaneRef.namespace", func(obj runtime.Object) []string {
		roll := obj.(*maistra.ServiceMeshMember)
		return []string{roll.Spec.ControlPlaneRef.Namespace}
	})
	if err != nil {
		return err
	}

	// watch namespaces so we can create the SMMR when the control plane namespace is created
	err = c.Watch(&source.Kind{Type: &core.Namespace{}}, &handler.EnqueueRequestsFromMapFunc{
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
	err = c.Watch(&source.Kind{Type: &maistra.ServiceMeshMemberRoll{}}, &handler.EnqueueRequestsFromMapFunc{
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
	list := &maistra.ServiceMeshMemberList{}
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
	scheme        *runtime.Scheme
	eventRecorder record.EventRecorder
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *MemberReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("ServiceMeshMember", request)
	reqLogger.Info("Processing ServiceMeshMember")

	member := &maistra.ServiceMeshMember{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, member)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	deleted := member.GetDeletionTimestamp() != nil
	finalizers := sets.NewString(member.Finalizers...)
	if deleted {
		if !finalizers.Has(common.FinalizerName) {
			reqLogger.Info("ServiceMeshMember deleted")
			return reconcile.Result{}, nil
		}

		memberRoll := &maistra.ServiceMeshMemberRoll{}
		err = r.Client.Get(context.TODO(), getMemberRollKey(member), memberRoll)
		if err != nil {
			if !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
		} else if memberRoll.DeletionTimestamp == nil {
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
				if err != nil && !errors.IsNotFound(err) { // if NotFound, MemberRoll has been deleted, which is what we wanted. This means this is not an error, but a success.
					err = errors2.Wrapf(err, "Could not delete ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
					_ = r.reportError(member, r.isNamespaceConfigured(memberRoll, member.Namespace), maistra.ConditionReasonMemberCannotDeleteMemberRoll, err.Error())
					return reconcile.Result{}, err
				}
			} else {
				err = r.Client.Update(context.TODO(), memberRoll)
				if err != nil {
					if errors.IsNotFound(err) || errors.IsConflict(err) {
						// local cache is stale; this isn't an error, so we shouldn't log it as such;
						// instead, we stop reconciling and wait for the watch event to arrive and trigger another reconciliation
						return reconcile.Result{}, nil
					}
					err = errors2.Wrapf(err, "Could not update ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
					_ = r.reportError(member, r.isNamespaceConfigured(memberRoll, member.Namespace), maistra.ConditionReasonMemberCannotUpdateMemberRoll, err.Error())
					return reconcile.Result{}, err
				}
			}
		}

		reqLogger.Info("Removing finalizer from ServiceMeshMember")
		finalizers.Delete(common.FinalizerName)
		member.SetFinalizers(finalizers.List())
		err = r.Client.Update(context.TODO(), member)
		if err != nil {
			err = errors2.Wrapf(err, "Could not update ServiceMeshMember %s/%s when removing finalizer", member.Namespace, member.Name)
			r.recordEvent(member, core.EventTypeWarning, eventReasonFailedReconcile, err.Error())
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil

	} else if !finalizers.Has(common.FinalizerName) {
		reqLogger.Info("Adding finalizer to ServiceMeshMember", "finalizer", common.FinalizerName)
		finalizers.Insert(common.FinalizerName)
		member.SetFinalizers(finalizers.List())
		err = r.Client.Update(context.TODO(), member)
		return reconcile.Result{}, err
	}

	memberRoll := &maistra.ServiceMeshMemberRoll{}
	err = r.Client.Get(context.TODO(), getMemberRollKey(member), memberRoll)
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		// MemberRoll doesn't exist, let's create it
		memberRoll = &maistra.ServiceMeshMemberRoll{
			ObjectMeta: v12.ObjectMeta{
				Name:      common.MemberRollName,
				Namespace: member.Spec.ControlPlaneRef.Namespace,
				Annotations: map[string]string{
					common.CreatedByKey: controllerName,
				},
			},
			Spec: maistra.ServiceMeshMemberRollSpec{
				Members: []string{member.Namespace},
			},
		}

		err = r.Client.Create(context.TODO(), memberRoll)
		if err != nil {
			if errors.IsNotFound(err) {
				errorMessage := "Could not create ServiceMeshMemberRoll, because the referenced namespace doesn't exist"
				reqLogger.Info(errorMessage, "namespace", memberRoll.Namespace)
				statusUpdateErr := r.reportError(member, false, maistra.ConditionReasonMemberCannotCreateMemberRoll, errorMessage)
				return reconcile.Result{}, statusUpdateErr
			} else if errors.IsConflict(err) {
				// local cache is stale; this isn't an error, so we shouldn't log it as such;
				// instead, we stop reconciling and wait for the watch event to arrive and trigger another reconciliation
				return reconcile.Result{}, nil
			} else {
				// we're dealing with a different type of error (either a validation error or an actual (e.g. I/O) error
				wrappedErr := errors2.Wrapf(err, "Could not create ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
				_ = r.reportError(member, false, maistra.ConditionReasonMemberCannotCreateMemberRoll, wrappedErr.Error())

				// 400 Bad Request is returned by the validation webhook. This isn't a controller error, but a user error, so we shouldn't log it as such.
				// This happens when the namespace is already a member of a different MemberRoll.
				if errors.IsBadRequest(err) {
					reqLogger.Info(wrappedErr.Error())
					return reconcile.Result{
						Requeue: true,
					}, nil
				}
				return reconcile.Result{}, wrappedErr
			}
		}
		r.recordEvent(member, core.EventTypeNormal, eventReasonSuccessfulReconcile, "Successfully created ServiceMeshMemberRoll and added namespace to it")

	} else {
		if !contains(member.Namespace, memberRoll.Spec.Members) {
			memberRoll.Spec.Members = append(memberRoll.Spec.Members, member.Namespace)

			err = r.Client.Update(context.TODO(), memberRoll)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsConflict(err) {
					// local cache is stale; this isn't an error, so we shouldn't log it as such;
					// instead, we stop reconciling and wait for the watch event to arrive and trigger another reconciliation
					return reconcile.Result{}, nil
				} else {
					// we're dealing with either a validation error or an actual (e.g. I/O) error
					wrappedErr := errors2.Wrapf(err, "Could not update ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
					_ = r.reportError(member, false, maistra.ConditionReasonMemberCannotUpdateMemberRoll, wrappedErr.Error())
					if errors.IsBadRequest(err) {
						reqLogger.Info(wrappedErr.Error())
						return reconcile.Result{
							Requeue: true,
						}, nil
					}
					return reconcile.Result{}, wrappedErr
				}
			}
			r.recordEvent(member, core.EventTypeNormal, eventReasonSuccessfulReconcile, "Successfully added namespace to ServiceMeshMemberRoll")
		}
	}

	err = r.updateStatus(member, true, r.isNamespaceConfigured(memberRoll, member.Namespace), "", "")
	return reconcile.Result{}, err
}

func (r *MemberReconciler) isNamespaceConfigured(memberRoll *maistra.ServiceMeshMemberRoll, namespace string) bool {
	return sets.NewString(memberRoll.Status.ConfiguredMembers...).Has(namespace)
}

func (r *MemberReconciler) reportError(member *maistra.ServiceMeshMember, ready bool, reason maistra.ServiceMeshMemberConditionReason, message string) error {
	r.recordEvent(member, core.EventTypeWarning, eventReasonFailedReconcile, message)
	return r.updateStatus(member, false, ready, reason, message)
}

func (r *MemberReconciler) updateStatus(member *maistra.ServiceMeshMember, reconciled, ready bool, reason maistra.ServiceMeshMemberConditionReason, message string) error {
	member.Status.ObservedGeneration = member.Generation // TODO: if we have re-read the member from the cache, the Generation may no longer be the same as in the member we reconciled. We shouldn't store the new member's generation as the observedGeneration, but the original one
	member.Status.SetCondition(maistra.ServiceMeshMemberCondition{
		Type:    maistra.ConditionTypeMemberReconciled,
		Status:  boolToConditionStatus(reconciled),
		Reason:  reason,
		Message: message,
	})
	member.Status.SetCondition(maistra.ServiceMeshMemberCondition{
		Type:    maistra.ConditionTypeMemberReady,
		Status:  boolToConditionStatus(ready),
		Reason:  reason,
		Message: message,
	})

	err := r.Client.Status().Update(context.TODO(), member)
	if err != nil {
		return errors2.Wrapf(err, "Could not update status of ServiceMeshMember %s/%s", member.Namespace, member.Name)
		r.Log.Error(err, "Error updating ServiceMeshMember status")
	}
	return nil
}

func (r *MemberReconciler) recordEvent(member *maistra.ServiceMeshMember, eventType string, reason maistra.ConditionReason, message string) {
	r.eventRecorder.Event(member, eventType, string(reason), message)
}

func boolToConditionStatus(b bool) core.ConditionStatus {
	if b {
		return core.ConditionTrue
	} else {
		return core.ConditionFalse
	}
}

func getMemberRollKey(member *maistra.ServiceMeshMember) client.ObjectKey {
	memberRollKey := client.ObjectKey{
		Name:      common.MemberRollName,
		Namespace: member.Spec.ControlPlaneRef.Namespace,
	}
	return memberRollKey
}

func contains(needle string, haystack []string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
