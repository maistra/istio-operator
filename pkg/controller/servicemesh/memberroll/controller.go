package memberroll

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/maistra/istio-operator/pkg/apis/external"
	kialiv1alpha1 "github.com/maistra/istio-operator/pkg/apis/external/kiali/v1alpha1"
	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

const (
	controllerName = "servicemeshmemberroll-controller"

	statusAnnotationConfiguredMemberCount = "configuredMemberCount"
)

// Add creates a new ServiceMeshMemberRoll Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	kialiReconciler := defaultKialiReconciler{Client: mgr.GetClient()}
	cniConfig, err := cni.InitConfig(mgr)
	if err != nil {
		return err
	}
	return add(mgr, newReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetEventRecorderFor(controllerName), newNamespaceReconciler, &kialiReconciler, cniConfig))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(cl client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder, namespaceReconcilerFactory NamespaceReconcilerFactory, kialiReconciler KialiReconciler, cniConfig cni.Config) *MemberRollReconciler {
	return &MemberRollReconciler{
		ControllerResources: common.ControllerResources{
			Client:        cl,
			Scheme:        scheme,
			EventRecorder: eventRecorder,
		},
		cniConfig:                  cniConfig,
		namespaceReconcilerFactory: namespaceReconcilerFactory,
		kialiReconciler:            kialiReconciler,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *MemberRollReconciler) error {
	log := createLogger()
	ctx := common.NewContextWithLog(common.NewContext(), log)

	// Create a new controller
	wrappedReconciler := common.NewConflictHandlingReconciler(r)
	c, err := controller.New(controllerName, mgr, controller.Options{MaxConcurrentReconciles: common.Config.Controller.MemberRollReconcilers, Reconciler: wrappedReconciler})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ServiceMeshMemberRoll
	err = c.Watch(&source.Kind{Type: &v1.ServiceMeshMemberRoll{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO: should this be moved somewhere else?
	err = mgr.GetFieldIndexer().IndexField(ctx, &v1.ServiceMeshMemberRoll{}, "spec.members", func(obj runtime.Object) []string {
		roll := obj.(*v1.ServiceMeshMemberRoll)
		return roll.Spec.Members
	})
	if err != nil {
		return err
	}

	// watch namespaces and trigger reconcile requests as those that match a member roll come and go
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(ns handler.MapObject) []reconcile.Request {
			list := &v1.ServiceMeshMemberRollList{}
			err := mgr.GetClient().List(ctx, list, client.MatchingField("spec.members", ns.Meta.GetName()))
			if err != nil {
				log.Error(err, "Could not list ServiceMeshMemberRolls")
			}

			var requests []reconcile.Request
			for _, smmr := range list.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: common.ToNamespacedName(&smmr),
				})
			}
			return requests
		}),
	}, predicate.Funcs{
		UpdateFunc: func(_ event.UpdateEvent) bool {
			// we don't need to process the member roll on updates
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			// we don't need to process the member roll on generic events
			return false
		},
	})
	if err != nil {
		return err
	}

	// watch control planes and trigger reconcile requests as they come and go
	err = c.Watch(&source.Kind{Type: &v2.ServiceMeshControlPlane{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(smcpMap handler.MapObject) []reconcile.Request {
			namespacedName := types.NamespacedName{Name: common.MemberRollName, Namespace: smcpMap.Meta.GetNamespace()}
			err := mgr.GetClient().Get(ctx, namespacedName, &v1.ServiceMeshMemberRoll{})
			if err != nil {
				if !errors.IsNotFound(err) {
					log.Error(err, "Could not list ServiceMeshMemberRolls")
				}
				return []reconcile.Request{}
			}

			return []reconcile.Request{{NamespacedName: namespacedName}}
		}),
	}, predicate.Funcs{
		DeleteFunc: func(_ event.DeleteEvent) bool {
			// we don't need to process the member roll on deletions (we add an owner reference, so it gets deleted automatically)
			return false
		},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &MemberRollReconciler{}

type NamespaceReconcilerFactory func(ctx context.Context, cl client.Client, meshNamespace string, meshVersion versions.Version, isCNIEnabled bool) (NamespaceReconciler, error)

// MemberRollReconciler reconciles a ServiceMeshMemberRoll object
type MemberRollReconciler struct {
	common.ControllerResources
	cniConfig cni.Config

	namespaceReconcilerFactory NamespaceReconcilerFactory
	kialiReconciler            KialiReconciler
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

	// Fetch the ServiceMeshMemberRoll instance
	instance := &v1.ServiceMeshMemberRoll{}
	err := r.Client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("ServiceMeshMemberRoll deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	deleted := instance.GetDeletionTimestamp() != nil
	finalizers := sets.NewString(instance.Finalizers...)
	if deleted {
		if !finalizers.Has(common.FinalizerName) {
			reqLogger.Info("ServiceMeshMemberRoll deleted")
			return reconcile.Result{}, nil
		}
		reqLogger.Info("Deleting ServiceMeshMemberRoll")

		configuredNamespaces, err := r.findConfiguredNamespaces(ctx, instance.Namespace)
		if err != nil {
			reqLogger.Error(err, "error listing mesh member namespaces")
			return reconcile.Result{}, err
		}

		configuredMembers, err, nsErrors := r.reconcileNamespaces(ctx, nil, nameSet(&configuredNamespaces), instance.Namespace, versions.DefaultVersion)
		if err != nil {
			return reconcile.Result{}, err
		}
		instance.Status.ConfiguredMembers = configuredMembers
		if len(nsErrors) > 0 {
			return reconcile.Result{}, utilerrors.NewAggregate(nsErrors)
		}

		mesh, err := r.getActiveMesh(ctx, instance)
		if err != nil {
			return reconcile.Result{}, err
		}
		if mesh != nil && mesh.Status.AppliedSpec.Addons != nil &&
			mesh.Status.AppliedSpec.Addons.Kiali != nil &&
			mesh.Status.AppliedSpec.Addons.Kiali.Enabled != nil &&
			*mesh.Status.AppliedSpec.Addons.Kiali.Enabled {
			err = r.kialiReconciler.reconcileKiali(ctx, mesh.Status.AppliedSpec.Addons.Kiali.Name, instance.Namespace, []string{})
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// get fresh SMMR from cache to minimize the chance of a conflict during update (the SMMR might have been updated during the execution of Reconcile())
		instance = &v1.ServiceMeshMemberRoll{}
		if err := r.Client.Get(ctx, request.NamespacedName, instance); err == nil {
			finalizers = sets.NewString(instance.Finalizers...)
			finalizers.Delete(common.FinalizerName)
			instance.SetFinalizers(finalizers.List())
			if err := r.Client.Update(ctx, instance); err == nil {
				reqLogger.Info("Removed finalizer")
			} else if !(errors.IsGone(err) || errors.IsNotFound(err)) {
				return reconcile.Result{}, pkgerrors.Wrap(err, "Error removing ServiceMeshMemberRoll finalizer")
			}
		} else if !(errors.IsGone(err) || errors.IsNotFound(err)) {
			return reconcile.Result{}, pkgerrors.Wrap(err, "Error getting ServiceMeshMemberRoll prior to removing finalizer")
		}

		return reconcile.Result{}, nil
	} else if !finalizers.Has(common.FinalizerName) {
		reqLogger.Info("Adding finalizer to ServiceMeshMemberRoll", "finalizer", common.FinalizerName)
		finalizers.Insert(common.FinalizerName)
		instance.SetFinalizers(finalizers.List())
		err = r.Client.Update(ctx, instance)
		return reconcile.Result{}, err
	}

	reqLogger.Info("Reconciling ServiceMeshMemberRoll")

	mesh, err := r.getActiveMesh(ctx, instance)
	if mesh == nil || err != nil {
		return reconcile.Result{}, err
	}

	var newConfiguredMembers []string
	var nsErrors []error
	allNamespaces, err := r.getAllNamespaces(ctx)
	if err != nil {
		return reconcile.Result{}, pkgerrors.Wrap(err, "could not list all namespaces")
	}
	requiredMembers := sets.NewString(instance.Spec.Members...)
	configuredMembers := sets.NewString(instance.Status.ConfiguredMembers...)
	deletedMembers := configuredMembers.Difference(allNamespaces)
	unconfiguredMembers := allNamespaces.Intersection(requiredMembers.Difference(configuredMembers))

	// never include the mesh namespace in unconfigured list
	delete(unconfiguredMembers, instance.Namespace)

	meshVersion, err := versions.ParseVersion(mesh.Spec.Version)
	if err != nil {
		reqLogger.Error(err, fmt.Sprintf("unsupported mesh version: %s", mesh.Spec.Version))
		return reconcile.Result{}, err
	}

	// this must be checked first to ensure the correct cni network is attached to the members
	if mesh.Status.GetReconciledVersion() != instance.Status.ServiceMeshReconciledVersion { // service mesh has been updated
		reqLogger.Info("Reconciling ServiceMeshMemberRoll namespaces with new generation of ServiceMeshControlPlane")

		instance.Status.ConfiguredMembers = make([]string, 0, len(instance.Spec.Members))
		newConfiguredMembers, err, nsErrors = r.reconcileNamespaces(ctx, requiredMembers, nil, instance.Namespace, meshVersion)
		if err != nil {
			return reconcile.Result{}, err
		}
		instance.Status.ConfiguredMembers = newConfiguredMembers
		instance.Status.ServiceMeshGeneration = mesh.Status.ObservedGeneration
		instance.Status.ServiceMeshReconciledVersion = mesh.Status.GetReconciledVersion()
	} else if instance.Generation != instance.Status.ObservedGeneration { // member roll has been updated

		reqLogger.Info("Reconciling new generation of ServiceMeshMemberRoll")

		instance.Status.ConfiguredMembers = make([]string, 0, len(instance.Spec.Members))

		// setup namespaces
		configuredNamespaces, err := r.findConfiguredNamespaces(ctx, mesh.Namespace)
		if err != nil {
			reqLogger.Error(err, "error listing mesh member namespaces")
			return reconcile.Result{}, err
		}

		existingMembers := nameSet(&configuredNamespaces)
		namespacesToRemove := existingMembers.Difference(requiredMembers)
		newConfiguredMembers, err, nsErrors = r.reconcileNamespaces(ctx, requiredMembers, namespacesToRemove, instance.Namespace, meshVersion)
		if err != nil {
			return reconcile.Result{}, err
		}
		instance.Status.ConfiguredMembers = newConfiguredMembers
		instance.Status.ServiceMeshGeneration = mesh.Status.ObservedGeneration
		instance.Status.ServiceMeshReconciledVersion = mesh.Status.GetReconciledVersion()
	} else if len(unconfiguredMembers) > 0 { // required namespace that was missing has been created
		reqLogger.Info("Reconciling newly created namespaces associated with this ServiceMeshMemberRoll")

		newConfiguredMembers, err, nsErrors = r.reconcileNamespaces(ctx, requiredMembers, nil, instance.Namespace, meshVersion)
		if err != nil {
			return reconcile.Result{}, err
		}
		instance.Status.ConfiguredMembers = newConfiguredMembers
		// we don't update the ServiceMeshGeneration in case the other members need to be updated
	} else if len(deletedMembers) > 0 { // namespace that was configured has been deleted
		// nothing to do, but we need to update the ConfiguredMembers field
		reqLogger.Info("Removing deleted namespaces from ConfiguredMembers")
		instance.Status.ConfiguredMembers = make([]string, 0, len(instance.Spec.Members))
		for _, member := range instance.Spec.Members {
			if member == instance.Namespace {
				// we never operate on the control plane namespace
				continue
			}
			if allNamespaces.Has(member) {
				instance.Status.ConfiguredMembers = append(instance.Status.ConfiguredMembers, member)
			}
		}
	} else {
		// nothing to do
		reqLogger.Info("nothing to reconcile")
		return reconcile.Result{}, nil
	}

	if requiredMembers.Equal(sets.NewString(instance.Status.ConfiguredMembers...)) {
		instance.Status.SetCondition(v1.ServiceMeshMemberRollCondition{
			Type:    v1.ConditionTypeMemberRollReady,
			Status:  corev1.ConditionTrue,
			Reason:  v1.ConditionReasonConfigured,
			Message: "All namespaces have been configured successfully",
		})
	} else {
		instance.Status.SetCondition(v1.ServiceMeshMemberRollCondition{
			Type:    v1.ConditionTypeMemberRollReady,
			Status:  corev1.ConditionFalse,
			Reason:  v1.ConditionReasonNamespaceMissing,
			Message: "A namespace listed in .spec.members does not exist",
		})
	}

	err = utilerrors.NewAggregate(nsErrors)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = r.updateStatus(ctx, instance)

	// tell Kiali about all the namespaces in the mesh
	var kialiErr error
	if mesh.Status.AppliedSpec.Addons != nil &&
		mesh.Status.AppliedSpec.Addons.Kiali != nil &&
		mesh.Status.AppliedSpec.Addons.Kiali.Enabled != nil &&
		*mesh.Status.AppliedSpec.Addons.Kiali.Enabled {
		kialiErr = r.kialiReconciler.reconcileKiali(ctx, mesh.Status.AppliedSpec.Addons.Kiali.Name, instance.Namespace, instance.Status.ConfiguredMembers)
	}

	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, kialiErr
}

func (r *MemberRollReconciler) getActiveMesh(ctx context.Context, instance *v1.ServiceMeshMemberRoll) (*v2.ServiceMeshControlPlane, error) {
	reqLogger := common.LogFromContext(ctx)
	meshList := &v2.ServiceMeshControlPlaneList{}
	if err := r.Client.List(ctx, meshList, client.InNamespace(instance.Namespace)); err != nil {
		return nil, pkgerrors.Wrap(err, "Error retrieving ServiceMeshControlPlane resources")
	}
	meshCount := len(meshList.Items)
	if meshCount != 1 {
		var err error
		if instance.GetDeletionTimestamp() == nil {
			if meshCount > 0 {
				reqLogger.Info("Skipping reconciliation of SMMR, because multiple ServiceMeshControlPlane resources exist in the project", "project", instance.Namespace)
				instance.Status.SetCondition(v1.ServiceMeshMemberRollCondition{
					Type:    v1.ConditionTypeMemberRollReady,
					Status:  corev1.ConditionFalse,
					Reason:  v1.ConditionReasonMultipleSMCP,
					Message: "Multiple ServiceMeshControlPlane resources exist in the namespace",
				})
			} else {
				reqLogger.Info("Skipping reconciliation of SMMR, because no ServiceMeshControlPlane exists in the project.", "project", instance.Namespace)
				instance.Status.ConfiguredMembers = make([]string, 0)
				instance.Status.SetCondition(v1.ServiceMeshMemberRollCondition{
					Type:    v1.ConditionTypeMemberRollReady,
					Status:  corev1.ConditionFalse,
					Reason:  v1.ConditionReasonSMCPMissing,
					Message: "No ServiceMeshControlPlane exists in the namespace",
				})
			}
			err = r.updateStatus(ctx, instance)
		}
		// when a control plane is created/deleted our watch will pick it up and issue a new reconcile event
		return nil, err
	}

	mesh := &meshList.Items[0]

	if mesh.Status.ObservedGeneration == 0 {
		reqLogger.Info("Initial service mesh installation has not completed")
		instance.Status.SetCondition(v1.ServiceMeshMemberRollCondition{
			Type:    v1.ConditionTypeMemberRollReady,
			Status:  corev1.ConditionFalse,
			Reason:  v1.ConditionReasonSMCPNotReconciled,
			Message: "Initial service mesh installation has not completed",
		})
		// a new reconcile request will be issued when the control plane resource is updated
		return nil, r.updateStatus(ctx, instance)
	} else if meshReconcileStatus := mesh.Status.GetCondition(status.ConditionTypeReconciled); meshReconcileStatus.Status != status.ConditionStatusTrue {
		reqLogger.Info("skipping reconciliation because mesh is not in a known good state")
		instance.Status.SetCondition(v1.ServiceMeshMemberRollCondition{
			Type:    v1.ConditionTypeMemberRollReady,
			Status:  corev1.ConditionFalse,
			Reason:  v1.ConditionReasonSMCPNotReconciled,
			Message: "Service mesh installation is not in a known good state",
		})
		// a new reconcile request will be issued when the control plane resource is updated
		return nil, r.updateStatus(ctx, instance)
	}
	return mesh, nil
}

func (r *MemberRollReconciler) updateStatus(ctx context.Context, instance *v1.ServiceMeshMemberRoll) error {
	log := common.LogFromContext(ctx)

	if instance.Status.Annotations == nil {
		instance.Status.Annotations = map[string]string{}
	}
	instance.Status.Annotations[statusAnnotationConfiguredMemberCount] = fmt.Sprintf("%d/%d", len(instance.Status.ConfiguredMembers), len(instance.Spec.Members))
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

func (r *MemberRollReconciler) reconcileNamespaces(ctx context.Context, namespacesToReconcile, namespacesToRemove sets.String, controlPlaneNamespace string, controlPlaneVersion versions.Version) (configuredMembers []string, err error, nsErrors []error) {
	reqLogger := common.LogFromContext(ctx)
	// current configuredNamespaces are namespacesToRemove minus control plane namespace
	configured := sets.NewString(namespacesToRemove.List()...)
	configured.Delete(controlPlaneNamespace)
	// create reconciler
	reconciler, err := r.namespaceReconcilerFactory(ctx, r.Client, controlPlaneNamespace, controlPlaneVersion, r.cniConfig.Enabled)
	if err != nil {
		return nil, err, nil
	}

	for ns := range namespacesToRemove {
		if ns == controlPlaneNamespace {
			// we never operate on the control plane namespace
			continue
		}
		err = reconciler.removeNamespaceFromMesh(ctx, ns)
		if err != nil {
			nsErrors = append(nsErrors, err)
		} else {
			configured.Delete(ns)
		}
	}
	for ns := range namespacesToReconcile {
		if ns == controlPlaneNamespace {
			// we never operate on the control plane namespace
			reqLogger.Info("ignoring control plane namespace in members list of ServiceMeshMemberRoll")
			continue
		}
		err = reconciler.reconcileNamespaceInMesh(ctx, ns)
		if err != nil {
			if errors.IsNotFound(err) || errors.IsGone(err) { // TODO: this check should be performed inside reconcileNamespaceInMesh
				reqLogger.Info("namespace to configure with mesh is missing", "namespace", ns)
			} else {
				nsErrors = append(nsErrors, err)
			}
		} else {
			configured.Insert(ns)
		}
	}
	configuredMembers = configured.List()
	return configuredMembers, nil, nsErrors
}

type KialiReconciler interface {
	reconcileKiali(ctx context.Context, kialiCRName, kialiCRNamespace string, configuredMembers []string) error
}

type defaultKialiReconciler struct {
	Client client.Client
}

func (r *defaultKialiReconciler) reconcileKiali(ctx context.Context, kialiCRName, kialiCRNamespace string, configuredMembers []string) error {
	reqLogger := common.LogFromContext(ctx)
	reqLogger.Info("Attempting to get Kiali CR", "kialiCRNamespace", kialiCRNamespace)

	kialiCR := &kialiv1alpha1.Kiali{}
	kialiCR.SetNamespace(kialiCRNamespace)
	kialiCR.SetName(kialiCRName)
	err := r.Client.Get(ctx, client.ObjectKey{Name: kialiCRName, Namespace: kialiCRNamespace}, kialiCR)
	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) || errors.IsGone(err) {
			reqLogger.Info("Kiali CR does not exist, Kiali probably not enabled")
			return nil
		}
		return pkgerrors.Wrap(err, "error retrieving Kiali CR from mesh")
	}

	// just get an array of strings consisting of the list of namespaces to be accessible to Kiali
	var accessibleNamespaces []string
	if len(configuredMembers) == 0 {
		// no configured members available - just allow access only to the control plane namespace
		accessibleNamespaces = []string{kialiCRNamespace}
	} else {
		// we are in multitenency mode with some namespaces being made available to kiali
		accessibleNamespaces = make([]string, 0, len(configuredMembers))
		for _, cm := range configuredMembers {
			accessibleNamespaces = append(accessibleNamespaces, cm)
		}
	}

	if existingNamespaces, found, _ := kialiCR.Spec.GetStringSlice("deployment.accessible_namespaces"); found && sets.NewString(accessibleNamespaces...).Equal(sets.NewString(existingNamespaces...)) {
		reqLogger.Info("Kiali CR deployment.accessible_namespaces already up to date")
		return nil
	}

	reqLogger.Info("Updating Kiali CR deployment.accessible_namespaces", "accessibleNamespaces", accessibleNamespaces)

	updatedKiali := &kialiv1alpha1.Kiali{
		Base: external.Base{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kialiCRName,
				Namespace: kialiCRNamespace,
			},
			Spec: v1.NewHelmValues(make(map[string]interface{})),
		},
	}
	err = updatedKiali.Spec.SetStringSlice("deployment.accessible_namespaces", accessibleNamespaces)
	if err != nil {
		return pkgerrors.Wrapf(err, "cannot set deployment.accessible_namespaces in Kiali CR %s/%s", kialiCRNamespace, kialiCRName)
	}

	err = r.Client.Patch(ctx, updatedKiali, client.Merge)
	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) || errors.IsGone(err) {
			reqLogger.Info(fmt.Sprintf("skipping kiali update, %s/%s is no longer available", kialiCRNamespace, kialiCRName))
			return nil
		}
		return pkgerrors.Wrapf(err, "cannot update Kiali CR %s/%s with new accessible namespaces", kialiCRNamespace, kialiCRName)
	}

	reqLogger.Info("Kiali CR deployment.accessible_namespaces updated", "accessibleNamespaces", accessibleNamespaces)
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

type NamespaceReconciler interface {
	reconcileNamespaceInMesh(ctx context.Context, namespace string) error
	removeNamespaceFromMesh(ctx context.Context, namespace string) error
}

func nameSet(list runtime.Object) sets.String {
	set := sets.NewString()
	err := meta.EachListItem(list, func(obj runtime.Object) error {
		o, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		set.Insert(o.GetName())
		return nil
	})
	if err != nil {
		// meta.EachListItem only returns an error if you pass in something that's not a ResourceList, so
		// it we don't expect it to ever return an error.
		panic(err)
	}
	return set
}

// Don't use this function to obtain a logger. Get it by invoking
// common.LogFromContext(ctx) to ensure that the logger has the
// correct context info and logs it.
func createLogger() logr.Logger {
	return logf.Log.WithName(controllerName)
}
