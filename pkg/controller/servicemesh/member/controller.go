package member

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

const (
	controllerName = "servicemeshmember-controller"

	// Event reasons
	eventReasonFailedReconcile           status.ConditionReason = "FailedReconcile"
	eventReasonSuccessfulReconcile       status.ConditionReason = "Reconciled"
	eventReasonControlPlaneMissing                              = "ErrorControlPlaneMissing"
	eventReasonErrorConfiguringNamespace                        = "ErrorConfiguringNamespace"
	eventReasonErrorRemovingNamespace                           = "ErrorRemovingNamespace"

	statusAnnotationControlPlaneRef = "controlPlaneRef"
)

// Add creates a new ServiceMeshMember Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	cniConfig, err := cni.InitConfig(mgr)
	if err != nil {
		return err
	}
	return add(mgr, newReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetEventRecorderFor(controllerName), newNamespaceReconciler, cniConfig))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(cl client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder,
	newNamespaceReconfilerFunc NewNamespaceReconcilerFunc, cniConfig cni.Config,
) *MemberReconciler {
	return &MemberReconciler{
		ControllerResources: common.ControllerResources{
			Client:        cl,
			Scheme:        scheme,
			EventRecorder: eventRecorder,
		},
		cniConfig:              cniConfig,
		newNamespaceReconciler: newNamespaceReconfilerFunc,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *MemberReconciler) error {
	ctx := common.NewContextWithLog(common.NewContext(), createLogger())
	// Create a new controller
	wrappedReconciler := common.NewConflictHandlingReconciler(r)
	c, err := controller.New(controllerName, mgr,
		controller.Options{
			MaxConcurrentReconciles: common.Config.Controller.MemberReconcilers,
			Reconciler:              wrappedReconciler,
		})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ServiceMeshMember
	err = c.Watch(&source.Kind{Type: &maistrav1.ServiceMeshMember{}}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
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

	// TODO: this needs to be moved outside the controller, since it configures global stuff (and is required by multiple controllers)
	err = mgr.GetFieldIndexer().IndexField(ctx, &maistrav1.ServiceMeshMember{}, "spec.controlPlaneRef.namespace", func(obj runtime.Object) []string {
		roll := obj.(*maistrav1.ServiceMeshMember)
		return []string{roll.Spec.ControlPlaneRef.Namespace}
	})
	if err != nil {
		return err
	}

	// watch SMCPs so we can update the resources in the app namespace when the SMCP is updated
	err = c.Watch(&source.Kind{Type: &maistrav2.ServiceMeshControlPlane{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(ns handler.MapObject) []reconcile.Request {
			return r.getRequestsForMembersWithReferenceToNamespace(ctx, ns.Meta.GetNamespace(), mgr.GetClient())
		}),
	}, predicate.Funcs{
		GenericFunc: func(_ event.GenericEvent) bool { return false }, // no need to process member on generic SMCP events
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *MemberReconciler) getRequestsForMembersWithReferenceToNamespace(ctx context.Context, ns string, cl client.Client) []reconcile.Request {
	log := common.LogFromContext(ctx)
	list := &maistrav1.ServiceMeshMemberList{}
	err := cl.List(ctx, list, client.MatchingFields{"spec.controlPlaneRef.namespace": ns})
	if err != nil {
		log.Error(err, "Could not list ServiceMeshMembers")
	}

	var requests []reconcile.Request
	for _, smm := range list.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: common.ToNamespacedName(&smm),
		})
	}
	return requests
}

var _ reconcile.Reconciler = &MemberReconciler{}

// MemberReconciler reconciles ServiceMeshMember objects
type MemberReconciler struct {
	common.ControllerResources

	cniConfig              cni.Config
	newNamespaceReconciler NewNamespaceReconcilerFunc
}

type NewNamespaceReconcilerFunc func(ctx context.Context, cl client.Client,
	meshNamespace string, meshVersion versions.Version, clusterWideMode, isCNIEnabled bool) (NamespaceReconciler, error)

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *MemberReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := createLogger().WithValues("ServiceMeshMember", request)
	ctx := common.NewReconcileContext(reqLogger)

	reqLogger.Info("Processing ServiceMeshMember")
	defer func() {
		reqLogger.Info("processing complete")
	}()

	object := &maistrav1.ServiceMeshMember{}
	err := r.Client.Get(ctx, request.NamespacedName, object)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	hasFinalizer := common.HasFinalizer(object)
	isMarkedForDeletion := object.DeletionTimestamp != nil
	if isMarkedForDeletion {
		if hasFinalizer {
			err = r.finalizeObject(ctx, object)
			if err != nil {
				return reconcile.Result{}, err
			}
			err = common.RemoveFinalizer(ctx, object, r.Client)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	} else if !hasFinalizer {
		err = common.AddFinalizer(ctx, object, r.Client)
		return reconcile.Result{}, err
	}

	return r.reconcileObject(ctx, object)
}

func (r *MemberReconciler) reconcileObject(ctx context.Context, member *maistrav1.ServiceMeshMember) (reconcile.Result, error) {
	log := common.LogFromContext(ctx)

	if member.Name != common.MemberName {
		return reconcile.Result{}, r.reportError(ctx, member, maistrav1.ConditionReasonMemberNameInvalid,
			fmt.Errorf("the ServiceMeshMember name is invalid; must be %q", common.MemberName))
	}

	if member.Namespace == member.Spec.ControlPlaneRef.Namespace {
		return reconcile.Result{}, nil
	}

	member.Status.SetAnnotation(statusAnnotationControlPlaneRef, member.Spec.ControlPlaneRef.String())

	// 1. Fetch the referenced SMCP
	smcp := &maistrav2.ServiceMeshControlPlane{}
	err := r.Client.Get(ctx, toObjectKey(member.Spec.ControlPlaneRef), smcp)
	if err != nil {
		if errors.IsNotFound(err) {
			err2 := r.reportError(ctx, member, eventReasonControlPlaneMissing, fmt.Errorf("the referenced ServiceMeshControlPlane object does not exist"))
			return reconcile.Result{}, err2
		}
		return reconcile.Result{}, err
	}

	// 2. Create the SMMR if it doesn't exist
	err = r.createMemberRollIfNeeded(ctx, member)
	if err != nil {
		if errors.IsNotFound(err) { // true when mesh namespace doesn't exist
			err2 := r.reportError(ctx, member, maistrav1.ConditionReasonMemberCannotCreateMemberRoll, err)
			return reconcile.Result{}, err2
		}
		return reconcile.Result{}, err
	}

	// 3. Configure the namespace
	meshVersion, err := versions.ParseVersion(smcp.Spec.Version)
	if err != nil {
		log.Error(err, fmt.Sprintf("unsupported mesh version: %s", smcp.Spec.Version))
		return reconcile.Result{}, err
	}

	clusterWideMode, err := meshVersion.Strategy().IsClusterScoped(&smcp.Spec)
	if err != nil {
		return reconcile.Result{}, err
	}

	reconciler, err := r.newNamespaceReconciler(ctx, r.Client, member.Spec.ControlPlaneRef.Namespace, meshVersion, clusterWideMode, r.cniConfig.Enabled)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = reconciler.reconcileNamespaceInMesh(ctx, member.Namespace)
	if err != nil {
		return reconcile.Result{}, r.reportError(ctx, member, eventReasonErrorConfiguringNamespace, err)
	}

	// 4. Update the status
	// TODO: should the following two fields be updated every time we update the status?
	member.Status.ServiceMeshGeneration = smcp.Status.ObservedGeneration
	member.Status.ServiceMeshReconciledVersion = smcp.Status.GetReconciledVersion()
	err = r.updateStatus(ctx, member, true, true, "", "")
	return reconcile.Result{}, err
}

func (r *MemberReconciler) createMemberRollIfNeeded(ctx context.Context, member *maistrav1.ServiceMeshMember) error {
	log := common.LogFromContext(ctx)
	ns := member.Spec.ControlPlaneRef.Namespace

	memberRoll := &maistrav1.ServiceMeshMemberRoll{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: common.MemberRollName, Namespace: ns}, memberRoll)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// MemberRoll doesn't exist, let's create it
		memberRoll = &maistrav1.ServiceMeshMemberRoll{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.MemberRollName,
				Namespace: ns,
				Annotations: map[string]string{
					common.CreatedByKey: controllerName,
				},
			},
		}

		log.Info("Creating ServiceMeshMemberRoll", "ServiceMeshMemberRoll", common.ToNamespacedName(memberRoll).String())
		err = r.Client.Create(ctx, memberRoll)
		if err != nil {
			return pkgerrors.Wrapf(err, "Could not create ServiceMeshMemberRoll %s/%s", ns, memberRoll.Name)
		}
		r.recordEvent(member, corev1.EventTypeNormal, eventReasonSuccessfulReconcile, "Successfully created ServiceMeshMemberRoll")
	}
	return nil
}

func toObjectKey(controlPlaneRef maistrav1.ServiceMeshControlPlaneRef) client.ObjectKey {
	return client.ObjectKey{
		Namespace: controlPlaneRef.Namespace,
		Name:      controlPlaneRef.Name,
	}
}

func (r *MemberReconciler) finalizeObject(ctx context.Context, obj runtime.Object) error {
	member := obj.(*maistrav1.ServiceMeshMember)

	reconciler, err := r.newNamespaceReconciler(ctx, r.Client, member.Spec.ControlPlaneRef.Namespace, nil, false, r.cniConfig.Enabled)
	if err != nil {
		return err
	}
	err = reconciler.removeNamespaceFromMesh(ctx, member.Namespace)
	if err != nil {
		return r.reportError(ctx, member, eventReasonErrorRemovingNamespace, err)
	}

	err = r.removeMemberRollIfNeeded(ctx, member)
	if err != nil {
		err2 := r.reportError(ctx, member, maistrav1.ConditionReasonMemberCannotDeleteMemberRoll, err)
		if err2 != nil {
			return err2
		}
		return err
	}
	return nil
}

func (r *MemberReconciler) removeMemberRollIfNeeded(ctx context.Context, member *maistrav1.ServiceMeshMember) error {
	log := common.LogFromContext(ctx)
	memberRoll := &maistrav1.ServiceMeshMemberRoll{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: common.MemberRollName, Namespace: member.Spec.ControlPlaneRef.Namespace}, memberRoll)
	if err != nil {
		// TODO: what if the MemberRoll is not found in the local cache, but it exists in the API? Can we even detect this?
		if errors.IsNotFound(err) {
			// if the member roll is missing, then this is exactly the result we want
			return nil
		}
		return err
	}
	if memberRoll.DeletionTimestamp == nil {
		memberRollCreatedByThisController := memberRoll.Annotations[common.CreatedByKey] == controllerName
		if !memberRollCreatedByThisController || len(memberRoll.Spec.Members) > 0 {
			return nil
		}

		members := &maistrav1.ServiceMeshMemberList{}
		err = r.Client.List(ctx, members, client.MatchingFields{"spec.controlPlaneRef.namespace": memberRoll.Namespace})
		if err != nil {
			return err
		}

		if len(members.Items) == 0 || (len(members.Items) == 1 && members.Items[0].UID == member.UID) {
			log.Info("Deleting ServiceMeshMemberRoll", "ServiceMeshMemberRoll", common.ToNamespacedName(memberRoll).String())
			err = r.Client.Delete(ctx, memberRoll) // TODO: need to add resourceVersion precondition to delete request (need newer apimachinery to do that)
			if err != nil && !errors.IsNotFound(err) {
				// if NotFound, MemberRoll has been deleted, which is what we wanted. This means this is not an error, but a success.
				return pkgerrors.Wrapf(err, "Could not delete ServiceMeshMemberRoll %s/%s", memberRoll.Namespace, memberRoll.Name)
			}
		}
	}
	return nil
}

func (r *MemberReconciler) reportError(ctx context.Context, member *maistrav1.ServiceMeshMember,
	reason maistrav1.ServiceMeshMemberConditionReason, err error,
) error {
	if common.IsConflict(err) {
		// Conflicts aren't recorded in the SMM status or Events, because
		// we expect them to happen occasionally. A conflict is not an error,
		// but we must return it so that the SMM object is requeued by the
		// conflictHandlingReconciler wrapper. The wrapper also ensures that
		// the conflict is logged at INFO level, not as an error.
		return err
	}
	r.recordEvent(member, corev1.EventTypeWarning, eventReasonFailedReconcile, err.Error())
	return r.updateStatus(ctx, member, false, false, reason, err.Error())
}

func (r *MemberReconciler) updateStatus(ctx context.Context, member *maistrav1.ServiceMeshMember, reconciled, ready bool,
	reason maistrav1.ServiceMeshMemberConditionReason, message string,
) error {
	member.Status.ObservedGeneration = member.Generation
	member.Status.SetCondition(maistrav1.ServiceMeshMemberCondition{
		Type:    maistrav1.ConditionTypeMemberReconciled,
		Status:  common.BoolToConditionStatus(reconciled),
		Reason:  reason,
		Message: message,
	})
	member.Status.SetCondition(maistrav1.ServiceMeshMemberCondition{
		Type:    maistrav1.ConditionTypeMemberReady,
		Status:  common.BoolToConditionStatus(ready),
		Reason:  reason,
		Message: message,
	})

	err := r.Client.Status().Patch(ctx, member, common.NewStatusPatch(member.Status))
	if err != nil && !errors.IsNotFound(err) {
		return pkgerrors.Wrapf(err, "Could not update status of ServiceMeshMember %s/%s", member.Namespace, member.Name)
	}
	return nil
}

func (r *MemberReconciler) recordEvent(member *maistrav1.ServiceMeshMember, eventType string, reason status.ConditionReason, message string) {
	r.EventRecorder.Event(member, eventType, string(reason), message)
}

// Don't use this function to obtain a logger. Get it by invoking
// common.LogFromContext(ctx) to ensure that the logger has the
// correct context info and logs it.
func createLogger() logr.Logger {
	return logf.Log.WithName(controllerName)
}

type NamespaceReconciler interface {
	reconcileNamespaceInMesh(ctx context.Context, namespace string) error
	removeNamespaceFromMesh(ctx context.Context, namespace string) error
}
