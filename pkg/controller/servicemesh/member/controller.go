package member

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	errors2 "github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	maxStatusUpdateRetriesOnConflict = 3

	statusAnnotationControlPlaneRef = "controlPlaneRef"
)

// Add creates a new ServiceMeshMember Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetEventRecorderFor(controllerName)))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(cl client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder) *MemberReconciler {
	return &MemberReconciler{
		ControllerResources: common.ControllerResources{
			Client:        cl,
			Scheme:        scheme,
			EventRecorder: eventRecorder,
		},
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *MemberReconciler) error {
	ctx := common.NewContextWithLog(common.NewContext(), createLogger())
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{MaxConcurrentReconciles: common.Config.Controller.MemberReconcilers, Reconciler: r})
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
				n.GetDeletionTimestamp() != nil
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
			return r.getRequestsForMembersReferencing(ctx, ns.Meta.GetName(), mgr.GetClient())
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
			return r.getRequestsForMembersReferencing(ctx, smmr.Meta.GetNamespace(), mgr.GetClient())
		}),
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *MemberReconciler) getRequestsForMembersReferencing(ctx context.Context, ns string, cl client.Client) []reconcile.Request {
	log := common.LogFromContext(ctx)
	list := &maistra.ServiceMeshMemberList{}
	err := cl.List(ctx, list, client.MatchingField("spec.controlPlaneRef.namespace", ns))
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
	common.ControllerResources
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *MemberReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := createLogger().WithValues("ServiceMeshMember", request)
	ctx := common.NewReconcileContext(reqLogger)

	reqLogger.Info("Processing ServiceMeshMember")
	defer func() {
		reqLogger.Info("processing complete")
	}()

	member := &maistra.ServiceMeshMember{}
	err := r.Client.Get(ctx, request.NamespacedName, member)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	mayContinue, err := common.HandleFinalization(ctx, member, r.finalizeMember, r.Client, r.EventRecorder)
	if err != nil || !mayContinue {
		return reconcile.Result{}, err
	}
	return r.reconcileMember(ctx, member)
}

func (r *MemberReconciler) reconcileMember(ctx context.Context, member *maistra.ServiceMeshMember) (reconcile.Result, error) {
	reqLogger := common.LogFromContext(ctx)
	memberRoll := &maistra.ServiceMeshMemberRoll{}
	err := r.Client.Get(ctx, getMemberRollKey(member), memberRoll)
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		// MemberRoll doesn't exist, let's create it
		memberRoll = &maistra.ServiceMeshMemberRoll{
			ObjectMeta: meta.ObjectMeta{
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

		reqLogger.Info("Creating ServiceMeshMemberRoll", "ServiceMeshMemberRoll", common.ToNamespacedName(memberRoll.ObjectMeta).String())
		err = r.Client.Create(ctx, memberRoll)
		if err != nil {
			if errors.IsNotFound(err) {
				errorMessage := "Could not create ServiceMeshMemberRoll, because the referenced namespace doesn't exist"
				reqLogger.Info(errorMessage, "namespace", memberRoll.Namespace)
				statusUpdateErr := r.reportError(ctx, member, false, maistra.ConditionReasonMemberCannotCreateMemberRoll, errorMessage)
				return reconcile.Result{}, statusUpdateErr
			} else if errors.IsConflict(err) {
				// local cache is stale; this isn't an error, so we shouldn't log it as such;
				// instead, we stop reconciling and wait for the watch event to arrive and trigger another reconciliation
				return reconcile.Result{}, nil
			} else {
				// we're dealing with a different type of error (either a validation error or an actual (e.g. I/O) error
				wrappedErr := errors2.Wrapf(err, "Could not create ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
				_ = r.reportError(ctx, member, false, maistra.ConditionReasonMemberCannotCreateMemberRoll, wrappedErr.Error())

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
			reqLogger.Info("Adding ServiceMeshMember to ServiceMeshMemberRoll", "ServiceMeshMemberRoll", common.ToNamespacedName(memberRoll.ObjectMeta).String())
			memberRoll.Spec.Members = append(memberRoll.Spec.Members, member.Namespace)

			err = r.Client.Update(ctx, memberRoll)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsConflict(err) {
					// local cache is stale; this isn't an error, so we shouldn't log it as such;
					// instead, we stop reconciling and wait for the watch event to arrive and trigger another reconciliation
					return reconcile.Result{}, nil
				} else {
					// we're dealing with either a validation error or an actual (e.g. I/O) error
					wrappedErr := errors2.Wrapf(err, "Could not update ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
					_ = r.reportError(ctx, member, false, maistra.ConditionReasonMemberCannotUpdateMemberRoll, wrappedErr.Error())
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

	if member.Status.Annotations == nil {
		member.Status.Annotations = map[string]string{}
	}
	member.Status.Annotations[statusAnnotationControlPlaneRef] = member.Spec.ControlPlaneRef.String()

	err = r.updateStatus(ctx, member, true, r.isNamespaceConfigured(memberRoll, member.Namespace), "", "")
	return reconcile.Result{}, err
}

func (r *MemberReconciler) finalizeMember(ctx context.Context, obj runtime.Object) (continueReconciliation bool, err error) {
	reqLogger := common.LogFromContext(ctx)
	member := obj.(*maistra.ServiceMeshMember)
	memberRoll := &maistra.ServiceMeshMemberRoll{}
	err = r.Client.Get(ctx, getMemberRollKey(member), memberRoll)
	if err != nil {
		// TODO: what if the MemberRoll is not found in the local cache, but it does exists in the API? Can we even detect this?
		if !errors.IsNotFound(err) {
			return false, err
		}
	} else if memberRoll.DeletionTimestamp == nil {
		for i, m := range memberRoll.Spec.Members {
			if m == member.Namespace {
				memberRoll.Spec.Members = append(memberRoll.Spec.Members[:i], memberRoll.Spec.Members[i+1:]...)
				break
			}
		}

		memberRollCreatedByThisController := memberRoll.Annotations[common.CreatedByKey] == controllerName
		if len(memberRoll.Spec.Members) == 0 && memberRollCreatedByThisController {
			reqLogger.Info("Deleting ServiceMeshMemberRoll", "ServiceMeshMemberRoll", common.ToNamespacedName(memberRoll.ObjectMeta).String())
			err = r.Client.Delete(ctx, memberRoll)     // TODO: need to add resourceVersion precondition to delete request (need newer apimachinery to do that)
			if err != nil && !errors.IsNotFound(err) { // if NotFound, MemberRoll has been deleted, which is what we wanted. This means this is not an error, but a success.
				err = errors2.Wrapf(err, "Could not delete ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
				_ = r.reportError(ctx, member, r.isNamespaceConfigured(memberRoll, member.Namespace), maistra.ConditionReasonMemberCannotDeleteMemberRoll, err.Error())
				return false, err
			}
		} else {
			reqLogger.Info("Removing ServiceMeshMember from ServiceMeshMemberRoll", "ServiceMeshMemberRoll", common.ToNamespacedName(memberRoll.ObjectMeta).String())
			err = r.Client.Update(ctx, memberRoll)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsConflict(err) {
					// local cache is stale; this isn't an error, so we shouldn't log it as such;
					// instead, we stop reconciling and wait for the watch event to arrive and trigger another reconciliation
					return false, nil
				}
				err = errors2.Wrapf(err, "Could not update ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
				_ = r.reportError(ctx, member, r.isNamespaceConfigured(memberRoll, member.Namespace), maistra.ConditionReasonMemberCannotUpdateMemberRoll, err.Error())
				return false, err
			}
		}
	}
	return true, nil
}

func (r *MemberReconciler) isNamespaceConfigured(memberRoll *maistra.ServiceMeshMemberRoll, namespace string) bool {
	return sets.NewString(memberRoll.Status.ConfiguredMembers...).Has(namespace)
}

func (r *MemberReconciler) reportError(ctx context.Context, member *maistra.ServiceMeshMember, ready bool, reason maistra.ServiceMeshMemberConditionReason, message string) error {
	r.recordEvent(member, core.EventTypeWarning, eventReasonFailedReconcile, message)
	return r.updateStatus(ctx, member, false, ready, reason, message)
}

func (r *MemberReconciler) updateStatus(ctx context.Context, member *maistra.ServiceMeshMember, reconciled, ready bool, reason maistra.ServiceMeshMemberConditionReason, message string) error {
	reqLogger := common.LogFromContext(ctx)
	member.Status.ObservedGeneration = member.Generation
	member.Status.SetCondition(maistra.ServiceMeshMemberCondition{
		Type:    maistra.ConditionTypeMemberReconciled,
		Status:  common.BoolToConditionStatus(reconciled),
		Reason:  reason,
		Message: message,
	})
	member.Status.SetCondition(maistra.ServiceMeshMemberCondition{
		Type:    maistra.ConditionTypeMemberReady,
		Status:  common.BoolToConditionStatus(ready),
		Reason:  reason,
		Message: message,
	})

	// TODO: use Client().Status().Patch() and remove the retry code below after we upgrade to controller-runtime 0.2+
	err := r.Client.Status().Update(ctx, member)
	if err == nil {
		return nil
	}

	// We only retry on conflict, because a retry will almost certainly succeed, since we first obtain a
	// fresh instance of the object. We don't retry on any other errors, as it's likely that we'll get the
	// same error again (plus, it's good to let the human operator know there was an error even if the
	// retry would succeed).
	converter := runtime.NewTestUnstructuredConverter(equality.Semantic)
	for retry := 0; retry < maxStatusUpdateRetriesOnConflict; retry++ {
		if errors.IsNotFound(err) {
			// The Member has disappeared, which means it was deleted by someone. We shouldn't treat this as an error
			// and we shouldn't retry.
			reqLogger.Info("Couldn't update status, because ServiceMeshMember has been deleted")
			return nil
		} else if errors.IsConflict(err) {
			reqLogger.Info("Ran into conflict when updating ServiceMeshMember's status. Retrying...")

			// This controller owns the ServiceMeshMember's status and can thus always override it (no-one else should
			// modify the status). We can't simply do a client.Get(), as that would again return the locally cached
			// object, which means that the update may fail again. We thus need to fetch the object from the API
			// server directly, update its status, and submit it back to the API Server. Hence the use of Unstructured.
			freshMember := unstructured.Unstructured{}
			freshMember.SetAPIVersion(maistra.SchemeGroupVersion.String())
			freshMember.SetKind("ServiceMeshMember")

			err = r.Client.Get(ctx, types.NamespacedName{member.Namespace, member.Name}, &freshMember)
			if err != nil {
				break
			}

			unstructuredStatus, err := converter.ToUnstructured(member.Status)
			if err != nil {
				break
			}

			err = unstructured.SetNestedField(freshMember.UnstructuredContent(), unstructuredStatus, "status")
			if err != nil {
				break
			}

			err = r.Client.Status().Update(ctx, &freshMember)
			if err == nil {
				return nil
			}
		} else {
			break
		}
	}
	return errors2.Wrapf(err, "Could not update status of ServiceMeshMember %s/%s", member.Namespace, member.Name)
}

func (r *MemberReconciler) recordEvent(member *maistra.ServiceMeshMember, eventType string, reason maistra.ConditionReason, message string) {
	r.EventRecorder.Event(member, eventType, string(reason), message)
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

// Don't use this function to obtain a logger. Get it by invoking
// common.LogFromContext(ctx) to ensure that the logger has the
// correct context info and logs it.
func createLogger() logr.Logger {
	return logf.Log.WithName(controllerName)
}
