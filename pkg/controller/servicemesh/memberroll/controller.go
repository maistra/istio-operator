package memberroll

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
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
	"sigs.k8s.io/yaml"

	"github.com/maistra/istio-operator/pkg/apis/external"
	kialiv1alpha1 "github.com/maistra/istio-operator/pkg/apis/external/kiali/v1alpha1"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

const (
	controllerName = "servicemeshmemberroll-controller"

	statusAnnotationConfiguredMemberCount = "configuredMemberCount"
	statusAnnotationKialiName             = "kialiName"

	prometheusConfigMapName         = "prometheus"
	prometheusConfigurationFilename = "prometheus.yml"

	prometheusScrapeConfigKeyName   = "scrape_configs"
	prometheusScrapeSDConfigKeyName = "kubernetes_sd_configs"
)

// Add creates a new ServiceMeshMemberRoll Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	kialiReconciler := defaultKialiReconciler{Client: mgr.GetClient()}
	prometheusReconciler := defaultPrometheusReconciler{Client: mgr.GetClient()}

	return add(mgr, newReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetEventRecorderFor(controllerName), &kialiReconciler, &prometheusReconciler))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(cl client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder, kialiReconciler KialiReconciler, prometheusReconciler PrometheusReconciler) *MemberRollReconciler {
	return &MemberRollReconciler{
		ControllerResources: common.ControllerResources{
			Client:        cl,
			Scheme:        scheme,
			EventRecorder: eventRecorder,
		},
		kialiReconciler:      kialiReconciler,
		prometheusReconciler: prometheusReconciler,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *MemberRollReconciler) error {
	log := createLogger()
	ctx := common.NewContextWithLog(common.NewContext(), log)

	// Create a new controller
	wrappedReconciler := common.NewConflictHandlingReconciler(r)
	c, err := controller.New(controllerName, mgr, controller.Options{
		MaxConcurrentReconciles: common.Config.Controller.MemberRollReconcilers,
		Reconciler:              wrappedReconciler,
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ServiceMeshMemberRoll
	err = c.Watch(&source.Kind{Type: &maistrav1.ServiceMeshMemberRoll{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO: should this be moved somewhere else?
	err = mgr.GetFieldIndexer().IndexField(ctx, &maistrav1.ServiceMeshMemberRoll{}, "spec.members", func(obj runtime.Object) []string {
		roll := obj.(*maistrav1.ServiceMeshMemberRoll)
		return roll.Spec.Members
	})
	if err != nil {
		return err
	}

	// watch namespaces and trigger reconcile requests as those that match a member roll come and go
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(ns handler.MapObject) []reconcile.Request {
			return toRequests(ctx, mgr.GetClient(), ns.Meta.GetName())
		}),
	}, predicate.Funcs{
		GenericFunc: func(_ event.GenericEvent) bool {
			// we don't need to process the member roll on generic events
			return false
		},
	})
	if err != nil {
		return err
	}

	// watch control planes and trigger reconcile requests as they come and go
	err = c.Watch(&source.Kind{Type: &maistrav2.ServiceMeshControlPlane{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(smcpMap handler.MapObject) []reconcile.Request {
			namespacedName := types.NamespacedName{Name: common.MemberRollName, Namespace: smcpMap.Meta.GetNamespace()}
			err := mgr.GetClient().Get(ctx, namespacedName, &maistrav1.ServiceMeshMemberRoll{})
			if err != nil {
				if !errors.IsNotFound(err) {
					log.Error(err, "Could not list ServiceMeshMemberRolls")
				}
				return []reconcile.Request{}
			}

			return []reconcile.Request{{NamespacedName: namespacedName}}
		}),
	}, predicate.Funcs{})
	if err != nil {
		return err
	}

	// Watch ServiceMeshMembers
	err = c.Watch(&source.Kind{Type: &maistrav1.ServiceMeshMember{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(mapObject handler.MapObject) []reconcile.Request {
			member := mapObject.Object.(*maistrav1.ServiceMeshMember)
			var requests []reconcile.Request

			// reconcile ServiceMeshMemberRoll referenced in the ServiceMeshMember
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: member.Spec.ControlPlaneRef.Namespace,
					Name:      common.MemberRollName,
				},
			})

			// reconcile ServiceMeshMemberRolls that have the ServiceMeshMember's namespace in spec.members
			requests = append(requests, toRequests(ctx, mgr.GetClient(), member.Namespace)...)
			return requests
		}),
	})
	if err != nil {
		return err
	}

	return nil
}

func toRequests(ctx context.Context, cl client.Client, namespace string) []reconcile.Request {
	log := common.LogFromContext(ctx)
	list := &maistrav1.ServiceMeshMemberRollList{}
	err := cl.List(ctx, list)
	if err != nil {
		log.Error(err, "Could not list ServiceMeshMemberRolls")
	}

	var requests []reconcile.Request
	for _, smmr := range list.Items {
		if isMember(namespace, smmr) {
			requests = append(requests, reconcile.Request{
				NamespacedName: common.ToNamespacedName(&smmr),
			})
		}
	}
	return requests
}

func isMember(namespace string, smmr maistrav1.ServiceMeshMemberRoll) bool {
	return (smmr.Spec.IsClusterScoped() && !isExcludedNamespace(namespace)) || contains(smmr.Spec.Members, namespace)
}

func contains(members []string, ns string) bool {
	for _, member := range members {
		if member == ns {
			return true
		}
	}
	return false
}

var _ reconcile.Reconciler = &MemberRollReconciler{}

// MemberRollReconciler reconciles a ServiceMeshMemberRoll object
type MemberRollReconciler struct {
	common.ControllerResources

	kialiReconciler      KialiReconciler
	prometheusReconciler PrometheusReconciler
}

// Reconcile reads that state of the cluster for a ServiceMeshMemberRoll object and makes changes based on the state read
// and what is in the ServiceMeshMemberRoll.Spec
func (r *MemberRollReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := createLogger().WithValues("ServiceMeshMemberRoll", request)
	ctx := common.NewReconcileContext(reqLogger)

	reqLogger.Info("Processing ServiceMeshMemberRoll")
	defer func() {
		reqLogger.Info("processing complete")
	}()

	// 1. fetch the object
	object := &maistrav1.ServiceMeshMemberRoll{}
	err := r.Client.Get(ctx, request.NamespacedName, object)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	// 2. finalize object if it's marked for deletion or add finalizer if it has none
	hasFinalizer := common.HasFinalizer(object)
	isMarkedForDeletion := object.DeletionTimestamp != nil
	if isMarkedForDeletion {
		if hasFinalizer {
			ok, err := r.finalizeObject(ctx, object)
			if err != nil {
				return reconcile.Result{}, err
			}
			if !ok {
				// Object wasn't finalized yet because ServiceMeshMember objects still exist.
				// Don't remove the finalizer and return with no errors. Another reconcile
				// attempt will be triggered when the ServiceMeshMember object deletion event arrives.
				return reconcile.Result{}, nil
			}
			err = common.RemoveFinalizer(ctx, object, r.Client)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	} else if !hasFinalizer {
		err = common.AddFinalizer(ctx, object, r.Client)
		return reconcile.Result{}, err
	}

	// 3. object is not deleted and already has a finalizer; reconcile it
	return r.reconcileObject(ctx, object)
}

func (r *MemberRollReconciler) reconcileObject(ctx context.Context, roll *maistrav1.ServiceMeshMemberRoll) (reconcile.Result, error) {
	log := common.LogFromContext(ctx)
	log.Info("Reconciling ServiceMeshMemberRoll")

	meshNamespace := roll.Namespace

	if roll.Name != common.MemberRollName {
		log.Info("Skipping reconciliation of SMMR with invalid name", "project", meshNamespace, "name", roll.Name)
		setReadyCondition(roll, false, maistrav1.ConditionReasonInvalidName,
			fmt.Sprintf("the ServiceMeshMemberRoll name is invalid; must be %q", common.MemberRollName))
		return reconcile.Result{}, r.updateStatus(ctx, roll)
	}

	// 1. fetch the SMCP object(s) and check if exactly one exists
	var mesh *maistrav2.ServiceMeshControlPlane
	meshList := &maistrav2.ServiceMeshControlPlaneList{}
	if err := r.Client.List(ctx, meshList, client.InNamespace(meshNamespace)); err != nil {
		return reconcile.Result{}, pkgerrors.Wrap(err, "Error retrieving ServiceMeshControlPlane resources")
	}
	switch len(meshList.Items) {
	case 0:
		mesh = nil
	case 1:
		mesh = &meshList.Items[0]
	default: // more than 1 SMCP found
		reason := maistrav1.ConditionReasonMultipleSMCP
		message := "Multiple ServiceMeshControlPlane resources exist in the namespace"
		log.Info("Skipping reconciliation of SMMR", "project", meshNamespace, "reason", reason, "message", message)
		setReadyCondition(roll, false, reason, message)
		return reconcile.Result{}, r.updateStatus(ctx, roll)
	}

	requiredNamespaces, err := r.getRequiredNamespaces(ctx, roll, meshNamespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	memberStatusMap := map[string]maistrav1.ServiceMeshMemberStatusSummary{}
	for _, c := range roll.Status.MemberStatuses {
		memberStatusMap[c.Namespace] = c
	}

	// 2. create ServiceMeshMember object for each ns in spec.members
	if mesh != nil {
		for _, ns := range requiredNamespaces.List() {
			member, err := r.ensureMemberExists(ctx, ns, mesh.Name, meshNamespace)
			if err != nil {
				return reconcile.Result{}, err
			} else if member == nil {
				setMemberCondition(memberStatusMap, ns, maistrav1.ServiceMeshMemberCondition{
					Type:    maistrav1.ConditionTypeMemberReconciled,
					Status:  corev1.ConditionFalse,
					Reason:  maistrav1.ConditionReasonMemberNamespaceNotExists,
					Message: fmt.Sprintf("Namespace %s does not exist", ns),
				})
				continue
			}

			ref := member.Spec.ControlPlaneRef
			if ref.Name != mesh.Name || ref.Namespace != meshNamespace {
				setMemberCondition(memberStatusMap, ns, maistrav1.ServiceMeshMemberCondition{
					Type:    maistrav1.ConditionTypeMemberReconciled,
					Status:  corev1.ConditionFalse,
					Reason:  maistrav1.ConditionReasonMemberReferencesDifferentControlPlane,
					Message: fmt.Sprintf("ServiceMeshMember %s/%s exists, but references ServiceMeshControlPlane %s/%s", ns, common.MemberName, ref.Namespace, ref.Name),
				})
			}
		}
	}

	// 3. gather status of all members that belong to this roll
	members := &maistrav1.ServiceMeshMemberList{}
	if err = r.Client.List(ctx, members, client.MatchingFields{"spec.controlPlaneRef.namespace": meshNamespace}); err != nil {
		return reconcile.Result{}, err
	}

	// 4. delete ServiceMeshMembers that were created by this controller, but are no longer in spec.members
	for _, member := range members.Items {
		if member.DeletionTimestamp == nil && isCreatedByThisController(&member) && (mesh == nil || !requiredNamespaces.Has(member.Namespace)) {
			err := r.Client.Delete(ctx, &member)
			if err != nil && !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
		}
	}

	// 5. check each ServiceMeshMember object to see if it's configured or terminating
	allKnownMembers := requiredNamespaces.Insert(getNamespaces(members)...).Delete(meshNamespace)
	configuredMembers := sets.NewString() // reconciled, but not necessarily up-to-date with the smcp
	upToDateMembers := sets.NewString()   // reconciled AND up-to-date with the smcp
	terminatingMembers := sets.NewString()
	for _, member := range members.Items {
		if member.DeletionTimestamp != nil {
			terminatingMembers.Insert(member.Namespace)
		} else {
			configured, upToDate := getMemberReconciliationStatus(&member, mesh)
			if configured {
				configuredMembers.Insert(member.Namespace)
				if upToDate {
					upToDateMembers.Insert(member.Namespace)
				}
			}
		}
		setMemberCondition(memberStatusMap, member.Namespace, member.Status.GetCondition(maistrav1.ConditionTypeMemberReconciled))
	}

	// 6. tell Kiali about all the namespaces in the mesh
	var kialiErr error
	if mesh != nil && mesh.Status.AppliedSpec.IsKialiEnabled() {
		kialiName := mesh.Status.AppliedSpec.Addons.Kiali.ResourceName()
		roll.Status.SetAnnotation(statusAnnotationKialiName, kialiName)

		meshIsClusterScoped, err := mesh.Spec.IsClusterScoped()
		if err != nil {
			return reconcile.Result{}, err
		}

		if meshIsClusterScoped && roll.Spec.IsClusterScoped() {
			kialiErr = r.kialiReconciler.reconcileKiali(ctx, kialiName, meshNamespace, []string{"**"}, getExcludedNamespaces())
		} else {
			kialiErr = r.kialiReconciler.reconcileKiali(ctx, kialiName, meshNamespace, allKnownMembers.List(), []string{})
		}
	}

	// 7. tell Prometheus about all the namespaces in the mesh
	var prometheusErr error
	if mesh.Status.AppliedSpec.IsPrometheusEnabled() {
		prometheusErr = r.prometheusReconciler.reconcilePrometheus(ctx, prometheusConfigMapName, meshNamespace, allKnownMembers.List())
	}

	// 7. update the status
	roll.Status.Members = allKnownMembers.List()
	roll.Status.PendingMembers = allKnownMembers.Difference(upToDateMembers).Difference(terminatingMembers).List()
	roll.Status.ConfiguredMembers = configuredMembers.List()
	roll.Status.TerminatingMembers = terminatingMembers.List()
	if mesh == nil {
		roll.Status.ServiceMeshGeneration = 0
		roll.Status.ServiceMeshReconciledVersion = ""
	} else {
		roll.Status.ServiceMeshGeneration = mesh.Status.ObservedGeneration
		roll.Status.ServiceMeshReconciledVersion = mesh.Status.GetReconciledVersion()
	}
	roll.Status.MemberStatuses = []maistrav1.ServiceMeshMemberStatusSummary{}
	for _, ns := range allKnownMembers.List() {
		memberStatus, exists := memberStatusMap[ns]
		if !exists {
			memberStatus = maistrav1.ServiceMeshMemberStatusSummary{
				Namespace:  ns,
				Conditions: []maistrav1.ServiceMeshMemberCondition{},
			}
		}
		roll.Status.MemberStatuses = append(roll.Status.MemberStatuses, memberStatus)
	}

	if mesh == nil {
		setReadyCondition(roll, false,
			maistrav1.ConditionReasonSMCPMissing,
			"No ServiceMeshControlPlane exists in the namespace")
	} else if len(roll.Status.PendingMembers) > 0 {
		setReadyCondition(roll, false,
			maistrav1.ConditionReasonReconcileError,
			fmt.Sprintf("The following namespaces are not yet configured: %v", roll.Status.PendingMembers))
	} else if kialiErr != nil {
		setReadyCondition(roll, false,
			maistrav1.ConditionReasonReconcileError,
			fmt.Sprintf("Kiali could not be configured: %v", kialiErr))
	} else if prometheusErr != nil {
		setReadyCondition(roll, false,
			maistrav1.ConditionReasonReconcileError,
			fmt.Sprintf("Prometheus could not be configured: %v", prometheusErr))
	} else {
		setReadyCondition(roll, true,
			maistrav1.ConditionReasonConfigured,
			"All namespaces have been configured successfully")
	}

	err = r.updateStatus(ctx, roll)
	if err != nil {
		return reconcile.Result{}, err
	} else if kialiErr != nil {
		return reconcile.Result{}, kialiErr
	} else if prometheusErr != nil {
		return reconcile.Result{}, prometheusErr
	}
	return reconcile.Result{}, nil
}

func (r *MemberRollReconciler) getRequiredNamespaces(ctx context.Context, roll *maistrav1.ServiceMeshMemberRoll, meshNamespace string) (sets.String, error) {
	if !roll.Spec.IsClusterScoped() {
		return sets.NewString(roll.Spec.Members...).Delete(meshNamespace), nil
	}

	nsList := &corev1.NamespaceList{}
	err := r.Client.List(ctx, nsList)
	if err != nil {
		return nil, err
	}
	requiredNamespaces := sets.NewString()
	for _, ns := range nsList.Items {
		if ns.Name != meshNamespace && !isExcludedNamespace(ns.Name) {
			requiredNamespaces.Insert(ns.Name)
		}
	}
	return requiredNamespaces, nil
}

func isExcludedNamespace(ns string) bool {
	for _, regex := range getExcludedNamespaces() {
		// ideally we'd compile the patterns on init, but common.GetOperatorNamespace() isn't available at that time yet
		match, err := regexp.Match(regex, []byte(ns))
		if err != nil {
			panic(err)
		}
		if match {
			return true
		}
	}
	return false
}

func getExcludedNamespaces() []string {
	return []string{
		"^" + common.GetOperatorNamespace() + "$",
		"^kube$",
		"^openshift$",
		"^kube-.*",
		"^openshift-.*",
	}
}

func setMemberCondition(memberStatusMap map[string]maistrav1.ServiceMeshMemberStatusSummary, ns string, condition maistrav1.ServiceMeshMemberCondition) {
	memberStatus, exists := memberStatusMap[ns]
	if !exists {
		memberStatus = maistrav1.ServiceMeshMemberStatusSummary{
			Namespace:  ns,
			Conditions: []maistrav1.ServiceMeshMemberCondition{},
		}
	}

	now := metav1.NewTime(time.Now().Truncate(time.Second))
	for i, prevCondition := range memberStatus.Conditions {
		if prevCondition.Type == condition.Type {
			if prevCondition.Status != condition.Status {
				condition.LastTransitionTime = now
			} else {
				condition.LastTransitionTime = prevCondition.LastTransitionTime
			}
			memberStatus.Conditions[i] = condition
			memberStatusMap[ns] = memberStatus
			return
		}
	}

	// If the condition does not exist, initialize the lastTransitionTime
	condition.LastTransitionTime = now
	memberStatus.Conditions = append(memberStatus.Conditions, condition)
	memberStatusMap[ns] = memberStatus
}

func setReadyCondition(roll *maistrav1.ServiceMeshMemberRoll, ready bool, reason maistrav1.ServiceMeshMemberRollConditionReason, message string) {
	roll.Status.SetCondition(maistrav1.ServiceMeshMemberRollCondition{
		Type:    maistrav1.ConditionTypeMemberRollReady,
		Status:  toConditionStatus(ready),
		Reason:  reason,
		Message: message,
	})
}

func toConditionStatus(ready bool) corev1.ConditionStatus {
	if ready {
		return corev1.ConditionTrue
	}
	return corev1.ConditionFalse
}

func isCreatedByThisController(member *maistrav1.ServiceMeshMember) bool {
	return member.Annotations != nil && member.Annotations[common.CreatedByKey] == controllerName
}

func getMemberReconciliationStatus(member *maistrav1.ServiceMeshMember, mesh *maistrav2.ServiceMeshControlPlane) (configured, upToDate bool) {
	ref := member.Spec.ControlPlaneRef
	condition := member.Status.GetCondition(maistrav1.ConditionTypeMemberReconciled)

	configured = mesh != nil && ref.Name == mesh.Name && ref.Namespace == mesh.Namespace && condition.Status == corev1.ConditionTrue
	upToDate = mesh != nil && member.Status.ServiceMeshReconciledVersion == mesh.Status.GetReconciledVersion()
	return configured, upToDate
}

// Returns nil ServiceMeshMember if namespace doesn't exist
func (r *MemberRollReconciler) ensureMemberExists(ctx context.Context, ns, meshName, meshNamespace string) (*maistrav1.ServiceMeshMember, error) {
	var member maistrav1.ServiceMeshMember
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: ns, Name: common.MemberName}, &member)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, pkgerrors.Wrapf(err, "Could not get ServiceMeshMember %s/%s", ns, common.MemberName)
		}
		member = maistrav1.ServiceMeshMember{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      common.MemberName,
				Annotations: map[string]string{
					common.CreatedByKey: controllerName,
				},
			},
			Spec: maistrav1.ServiceMeshMemberSpec{
				ControlPlaneRef: maistrav1.ServiceMeshControlPlaneRef{
					Name:      meshName,
					Namespace: meshNamespace,
				},
			},
		}

		// Check if Namespace exists, so we don't hit the API server unnecessarily
		namespace := corev1.Namespace{}
		err := r.Client.Get(ctx, client.ObjectKey{Name: ns}, &namespace)
		if err != nil {
			if errors.IsNotFound(err) {
				// Namespace doesn't exist. We'll create the ServiceMeshMember when someone creates the namespace.
				return nil, nil
			}
			return nil, err
		}
		if namespace.DeletionTimestamp != nil {
			// Namespace is being deleted. We can't create the ServiceMeshMember, as the CREATE operation would fail.
			return nil, nil
		}

		err = r.Client.Create(ctx, &member)
		if err != nil {
			if errors.IsNotFound(err) {
				// Namespace doesn't exist. This isn't an error. We'll create the object when someone creates the namespace.
				return nil, nil
			}
			return nil, pkgerrors.Wrapf(err, "Could not create ServiceMeshMember %s/%s", ns, common.MemberName)
		}
	}
	return &member, nil
}

func (r *MemberRollReconciler) finalizeObject(ctx context.Context, roll *maistrav1.ServiceMeshMemberRoll) (ok bool, err error) {
	members := &maistrav1.ServiceMeshMemberList{}
	err = r.Client.List(ctx, members, client.MatchingFields{"spec.controlPlaneRef.namespace": roll.Namespace})
	if err != nil {
		return false, err
	}

	for _, member := range members.Items {
		if member.DeletionTimestamp != nil || !isCreatedByThisController(&member) {
			continue
		}
		err := r.Client.Delete(ctx, &member)
		if err != nil && !errors.IsNotFound(err) {
			return false, pkgerrors.Wrapf(err, "Could not delete ServiceMeshMember %s/%s", member.Namespace, member.Name)
		}
	}

	kialiName := roll.Status.GetAnnotation(statusAnnotationKialiName)
	if kialiName != "" {
		err = r.kialiReconciler.reconcileKiali(ctx, kialiName, roll.Namespace, []string{}, []string{})
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func (r *MemberRollReconciler) updateStatus(ctx context.Context, instance *maistrav1.ServiceMeshMemberRoll) error {
	log := common.LogFromContext(ctx)

	instance.Status.SetAnnotation(
		statusAnnotationConfiguredMemberCount,
		fmt.Sprintf("%d/%d", len(instance.Status.ConfiguredMembers), len(instance.Status.Members)))
	instance.Status.ObservedGeneration = instance.GetGeneration()

	err := r.Client.Status().Patch(ctx, instance, common.NewStatusPatch(instance.Status))
	if err != nil {
		log.Error(err, "error updating status for ServiceMeshMemberRoll")
	}
	return err
}

func (r *MemberRollReconciler) findConfiguredNamespaces(ctx context.Context, meshNamespace string) (corev1.NamespaceList, error) {
	list := corev1.NamespaceList{}
	labelSelector := map[string]string{common.MemberOfKey: meshNamespace}
	err := r.Client.List(ctx, &list, client.MatchingLabels(labelSelector), client.InNamespace(""))
	return list, err
}

type KialiReconciler interface {
	reconcileKiali(ctx context.Context, kialiCRName, kialiCRNamespace string, accessibleNamespaces, excludedNamespaces []string) error
}

type defaultKialiReconciler struct {
	Client client.Client
}

func (r *defaultKialiReconciler) reconcileKiali(ctx context.Context, kialiCRName, kialiCRNamespace string,
	accessibleNamespaces, excludedNamespaces []string,
) error {
	reqLogger := common.LogFromContext(ctx)
	reqLogger.Info("Attempting to get Kiali CR", "kialiCRNamespace", kialiCRNamespace, "kialiCRName", kialiCRName)

	kialiCR := &kialiv1alpha1.Kiali{}
	kialiCR.SetNamespace(kialiCRNamespace)
	kialiCR.SetName(kialiCRName)
	err := r.Client.Get(ctx, client.ObjectKey{Name: kialiCRName, Namespace: kialiCRNamespace}, kialiCR)
	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) {
			reqLogger.Info("Kiali CR does not exist, Kiali probably not enabled")
			return nil
		}
		return pkgerrors.Wrap(err, "error retrieving Kiali CR from mesh")
	}

	// just get an array of strings consisting of the list of namespaces to be accessible to Kiali
	accessibleNamespacesSet := sets.NewString(accessibleNamespaces...)
	excludedNamespacesSet := sets.NewString(excludedNamespaces...)
	existingAccessible, accessibleFound, _ := kialiCR.Spec.GetStringSlice("deployment.accessible_namespaces")
	existingExcluded, excludedFound, _ := kialiCR.Spec.GetStringSlice("api.namespaces.exclude")
	if accessibleFound && accessibleNamespacesSet.Equal(sets.NewString(existingAccessible...)) &&
		excludedFound && excludedNamespacesSet.Equal(sets.NewString(existingExcluded...)) {
		reqLogger.Info("Kiali CR deployment.accessible_namespaces and api.namespaces.exclude already up to date")
		return nil
	}

	reqLogger.Info("Updating Kiali CR deployment.accessible_namespaces and api.namespaces.exclude",
		"deployment.accessibleNamespaces", accessibleNamespacesSet, "api.namespaces.exclude", excludedNamespaces)

	updatedKiali := &kialiv1alpha1.Kiali{
		Base: external.Base{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kialiCRName,
				Namespace: kialiCRNamespace,
			},
			Spec: maistrav1.NewHelmValues(make(map[string]interface{})),
		},
	}
	err = updatedKiali.Spec.SetStringSlice("deployment.accessible_namespaces", accessibleNamespacesSet.List())
	if err != nil {
		return pkgerrors.Wrapf(err, "cannot set deployment.accessible_namespaces in Kiali CR %s/%s", kialiCRNamespace, kialiCRName)
	}

	err = updatedKiali.Spec.SetStringSlice("api.namespaces.exclude", excludedNamespacesSet.List())
	if err != nil {
		return pkgerrors.Wrapf(err, "cannot set api.namespaces.exclude in Kiali CR %s/%s", kialiCRNamespace, kialiCRName)
	}

	err = r.Client.Patch(ctx, updatedKiali, client.Merge)
	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) {
			reqLogger.Info(fmt.Sprintf("skipping kiali update, %s/%s is no longer available", kialiCRNamespace, kialiCRName))
			return nil
		}
		return pkgerrors.Wrapf(err, "cannot update Kiali CR %s/%s with new accessible namespaces", kialiCRNamespace, kialiCRName)
	}

	reqLogger.Info("Kiali CR deployment.accessible_namespaces updated", "accessibleNamespaces", accessibleNamespacesSet)
	return nil
}

type PrometheusReconciler interface {
	reconcilePrometheus(ctx context.Context, prometheusCMName, prometheusNamespace string, configuredMembers []string) error
}

type defaultPrometheusReconciler struct {
	Client client.Client
}

func (r *defaultPrometheusReconciler) reconcilePrometheus(ctx context.Context, prometheusCMName, prometheusNamespace string, members []string) error {
	reqLogger := common.LogFromContext(ctx)
	reqLogger.Info("Attempting to get Prometheus ConfigMap", "prometheusNamespace", prometheusNamespace, "prometheusCMName", prometheusCMName)

	cm := &corev1.ConfigMap{}

	err := r.Client.Get(ctx, client.ObjectKey{Name: prometheusCMName, Namespace: prometheusNamespace}, cm)

	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) || errors.IsGone(err) {
			reqLogger.Info("Prometheus Config Map does not exist.")
			return nil
		}
		return pkgerrors.Wrap(err, "error retrieving Prometheus ConfigMap from mesh")
	}

	data := make(map[string]interface{})

	err = yaml.Unmarshal([]byte(cm.Data[prometheusConfigurationFilename]), &data)

	if err != nil {
		return pkgerrors.Wrap(err, "error unmarshaling prometheus.yml")
	}

	f, ok, err := unstructured.NestedFieldNoCopy(data, prometheusScrapeConfigKeyName)

	var scrapes []interface{}

	if err == nil && ok {
		scrapes, ok = f.([]interface{})

		if !ok {
			return fmt.Errorf("error no scrape_configs found from %v", f)
		}
	} else {
		return pkgerrors.Wrap(err, "error getting scrape_configs from ConfigMap")
	}

	for _, s := range scrapes {
		scrape, ok := s.(map[string]interface{})

		if !ok {
			return fmt.Errorf("error converting scrape_config from %v", s)
		}

		if scrape["job_name"] == "pilot" {
			continue
		}

		f, ok, err := unstructured.NestedFieldNoCopy(scrape, prometheusScrapeSDConfigKeyName)

		if err == nil {
			if ok {
				sds, ok := f.([]interface{})

				if ok {
					for _, v := range sds {
						sd, ok := v.(map[string]interface{})

						if ok {
							reqLogger.Info(fmt.Sprintf("Updating sd %v", v))

							err = unstructured.SetNestedStringSlice(sd, members, "namespaces", "names")

							if err != nil {
								return pkgerrors.Wrap(err, fmt.Sprintf("error setting sd %v", v))
							}
						}
					}
				} else {
					return fmt.Errorf("error can not process sd %v", f)
				}
			} else {
				reqLogger.Info(fmt.Sprintf("Ignoring scrape %v", s))
			}
		} else {
			return pkgerrors.Wrap(err, fmt.Sprintf("error getting sd from %v", s))
		}
	}

	updatedPrometheus := &corev1.ConfigMap{}

	updatedPrometheus.SetName(prometheusCMName)
	updatedPrometheus.SetNamespace(prometheusNamespace)

	updatedConfigurationFileData, err := yaml.Marshal(data)

	if err != nil {
		return pkgerrors.Wrap(err, "error marshalling updated prometheus configuration")
	}

	updatedConfigurationFile := string(updatedConfigurationFileData)

	reqLogger.Info(fmt.Sprintf("Prometheus updated configuration file %v", updatedConfigurationFile))

	updatedPrometheus.Data = map[string]string{
		prometheusConfigurationFilename: updatedConfigurationFile,
	}

	err = r.Client.Patch(ctx, updatedPrometheus, client.Merge)

	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) || errors.IsGone(err) {
			reqLogger.Info(fmt.Sprintf("skipping Prometheus update, %s/%s is no longer available", prometheusNamespace, prometheusCMName))
			return nil
		}
		return pkgerrors.Wrapf(err, "cannot update Prometheus ConfigMap %s/%s with namespaces %v", prometheusNamespace, prometheusCMName, members)
	}

	reqLogger.Info("Prometheus ConfigMap scraping namespaces updated", "namespaces", members)

	return nil
}

func (r *MemberRollReconciler) getAllNamespaces(ctx context.Context) (sets.String, error) {
	namespaceList := &corev1.NamespaceList{}
	err := r.Client.List(ctx, namespaceList)
	if err != nil {
		return nil, err
	}
	allNamespaces := sets.NewString()
	for _, namespace := range namespaceList.Items {
		allNamespaces.Insert(namespace.Name)
	}
	return allNamespaces, nil
}

// Don't use this function to obtain a logger. Get it by invoking
// common.LogFromContext(ctx) to ensure that the logger has the
// correct context info and logs it.
func createLogger() logr.Logger {
	return logf.Log.WithName(controllerName)
}

func getNamespaces(members *maistrav1.ServiceMeshMemberList) []string {
	namespaces := sets.NewString()
	for _, m := range members.Items {
		namespaces.Insert(m.GetNamespace())
	}
	return namespaces.List()
}
