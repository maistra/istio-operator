package memberroll

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const netAttachDefName = "istio-cni" // must match name of .conf file in multus.d

var log = logf.Log.WithName("controller_servicemeshmemberroll")

// Add creates a new ServiceMeshMemberRoll Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMemberList{ResourceManager: common.ResourceManager{Client: mgr.GetClient(), Log: log}, scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("servicemeshmemberroll-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ServiceMeshMemberRoll
	err = c.Watch(&source.Kind{Type: &v1.ServiceMeshMemberRoll{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO: should this be moved somewhere else?
	err = mgr.GetFieldIndexer().IndexField(&v1.ServiceMeshMemberRoll{}, "spec.members", func(obj runtime.Object) []string {
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
			err := mgr.GetClient().List(context.TODO(), client.MatchingField("spec.members", ns.Meta.GetName()), list)
			if err != nil {
				log.Error(err, "Could not list ServiceMeshMemberRolls")
			}

			var requests []reconcile.Request
			for _, smmr := range list.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      smmr.Name,
						Namespace: smmr.Namespace,
					},
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
	err = c.Watch(&source.Kind{Type: &v1.ServiceMeshControlPlane{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(smcpMap handler.MapObject) []reconcile.Request {
			if smcp, ok := smcpMap.Object.(*v1.ServiceMeshControlPlane); !ok {
				return nil
			} else if installCondition := smcp.Status.GetCondition(v1.ConditionTypeReconciled); installCondition.Status != v1.ConditionStatusTrue {
				// skip processing if the smcp is not fully reconciled (e.g. it's installing or updating)
				return nil
			}
			list := &v1.ServiceMeshMemberRollList{}
			err := mgr.GetClient().List(context.TODO(), client.InNamespace(smcpMap.Meta.GetNamespace()), list)
			if err != nil {
				log.Error(err, "Could not list ServiceMeshMemberRolls")
			}

			var requests []reconcile.Request
			for _, smmr := range list.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      smmr.Name,
						Namespace: smmr.Namespace,
					},
				})
			}
			return requests
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

var _ reconcile.Reconciler = &ReconcileMemberList{}

// ReconcileMemberList reconciles a ServiceMeshMemberRoll object
type ReconcileMemberList struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	common.ResourceManager
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ServiceMeshMemberRoll object and makes changes based on the state read
// and what is in the ServiceMeshMemberRoll.Spec
func (r *ReconcileMemberList) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("ServiceMeshMemberRoll", request)
	reqLogger.Info("Processing ServiceMeshMemberRoll")

	defer func() {
		reqLogger.Info("processing complete")
	}()

	// Fetch the ServiceMeshMemberRoll instance
	instance := &v1.ServiceMeshMemberRoll{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
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

		// create reconciler
		reconciler, err := newNamespaceReconciler(r.Client, reqLogger, instance.Namespace, false)
		if err != nil {
			return reconcile.Result{}, pkgerrors.Wrapf(err, "Error creating namespace reconciler")
		}

		for _, namespace := range instance.Spec.Members {
			err := reconciler.removeNamespaceFromMesh(namespace)
			if err != nil && !(errors.IsNotFound(err) || errors.IsGone(err)) {
				return reconcile.Result{}, pkgerrors.Wrapf(err, "Error cleaning up mesh member namespace %s", namespace)
			}
		}

		// Kiali is prohibited from seeing any namespace other than the control plane itself
		err = r.reconcileKiali(instance.Namespace, []string{instance.Namespace}, reqLogger)
		if err != nil {
			return reconcile.Result{}, err
		}

		// get fresh SMMR from cache to minimize the chance of a conflict during update (the SMMR might have been updated during the execution of Reconcile())
		instance = &v1.ServiceMeshMemberRoll{}
		if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err == nil {
			finalizers = sets.NewString(instance.Finalizers...)
			finalizers.Delete(common.FinalizerName)
			instance.SetFinalizers(finalizers.List())
			if err := r.Client.Update(context.TODO(), instance); err == nil {
				reqLogger.Info("Removed finalizer")
			} else if !(errors.IsGone(err) || errors.IsNotFound(err)) {
				return reconcile.Result{}, pkgerrors.Wrap(err, "Error removing ServiceMeshMemberRoll finalizer")
			}
		} else if !(errors.IsGone(err) || errors.IsNotFound(err)) {
			return reconcile.Result{}, pkgerrors.Wrap(err, "Error getting ServiceMeshMemberRoll prior to removing finalizer")
		}

		return reconcile.Result{}, err
	} else if !finalizers.Has(common.FinalizerName) {
		reqLogger.Info("Adding finalizer to ServiceMeshMemberRoll", "finalizer", common.FinalizerName)
		finalizers.Insert(common.FinalizerName)
		instance.SetFinalizers(finalizers.List())
		err = r.Client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "error adding finalizer to ServiceMeshMemberRoll")
		}
		return reconcile.Result{}, err
	}

	reqLogger.Info("Reconciling ServiceMeshMemberRoll")

	meshList := &v1.ServiceMeshControlPlaneList{}
	err = r.Client.List(context.TODO(), client.InNamespace(instance.Namespace), meshList)
	if err != nil {
		reqLogger.Error(err, "error retrieving ServiceMeshControlPlane resources")
		return reconcile.Result{}, err
	}
	meshCount := len(meshList.Items)
	if meshCount != 1 {
		if meshCount > 0 {
			reqLogger.Info("cannot reconcile ServiceMeshControlPlane: multiple ServiceMeshControlPlane resources exist in project")
		} else {
			reqLogger.Info(fmt.Sprintf("failed to locate ServiceMeshControlPlane for project %s", instance.Namespace))
		}
		// when a control plane is created/deleted our watch will pick it up and issue a new reconcile event
		return reconcile.Result{}, nil
	}

	mesh := meshList.Items[0]

	// verify owner reference, so member roll gets deleted with control plane
	addOwner := true
	for _, ownerRef := range instance.GetOwnerReferences() {
		if ownerRef.UID == mesh.GetUID() {
			addOwner = false
			break
		}
	}
	if addOwner {
		// add owner reference to the mesh so we can clean up if the mesh gets deleted
		reqLogger.Info("Adding OwnerReference to ServiceMeshMemberRoll")
		owner := metav1.NewControllerRef(&mesh, v1.SchemeGroupVersion.WithKind("ServiceMeshControlPlane"))
		owner.Controller = nil
		owner.BlockOwnerDeletion = nil
		instance.SetOwnerReferences([]metav1.OwnerReference{*owner})
		err = r.Client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "error adding OwnerReference to ServiceMeshMemberRoll")
		}
		return reconcile.Result{}, err
	}

	if mesh.Status.ObservedGeneration == 0 {
		reqLogger.Info("Initial service mesh installation has not completed")
		// a new reconcile request will be issued when the control plane resource is updated
		return reconcile.Result{}, nil
	} else if meshReconcileStatus := mesh.Status.GetCondition(v1.ConditionTypeReconciled); meshReconcileStatus.Status != v1.ConditionStatusTrue {
		// a new reconcile request will be issued when the control plane resource is updated
		reqLogger.Info("skipping reconciliation because mesh is not in a known good state")
		return reconcile.Result{}, nil
	}

	allErrors := []error{}
	allNamespaces, err := r.getAllNamespaces()
	if err != nil {
		reqLogger.Error(err, "could not list all namespaces")
		return reconcile.Result{}, err
	}
	requiredMembers := toSet(instance.Spec.Members)
	configuredMembers := toSet(instance.Status.ConfiguredMembers)
	deletedMembers := difference(configuredMembers, allNamespaces)
	unconfiguredMembers := intersection(difference(requiredMembers, configuredMembers), allNamespaces)

	// always include the mesh namespace in the configured list
	configuredMembers[instance.Namespace] = struct{}{}
	// never include the mesh namespace in unconfigured list
	delete(unconfiguredMembers, instance.Namespace)

	if instance.Generation != instance.Status.ObservedGeneration { // member roll has been updated

		reqLogger.Info("Reconciling new generation of ServiceMeshMemberRoll")

		instance.Status.ConfiguredMembers = make([]string, 0, len(instance.Spec.Members))

		// setup projects
		configuredNamespaces, err := common.FetchMeshResources(r.Client, corev1.SchemeGroupVersion.WithKind("Namespace"), mesh.Namespace, "")
		if err != nil {
			reqLogger.Error(err, "error listing mesh member namespaces")
			return reconcile.Result{}, err
		}

		// create reconciler
		reconciler, err := newNamespaceReconciler(r.Client, reqLogger, mesh.Namespace, common.IsCNIEnabled)
		if err != nil {
			return reconcile.Result{}, err
		}

		existingMembers := nameSet(configuredNamespaces.Items)
		for namespaceToRemove := range difference(existingMembers, requiredMembers) {
			if namespaceToRemove == instance.Namespace {
				// we never operate on the control plane namespace
				continue
			}
			err = reconciler.removeNamespaceFromMesh(namespaceToRemove)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
		for namespaceToReconcile := range requiredMembers {
			if namespaceToReconcile == instance.Namespace {
				// we never operate on the control plane namespace
				reqLogger.Info("ignoring control plane namespace in members list of ServiceMeshMemberRoll")
				continue
			}
			err = reconciler.reconcileNamespaceInMesh(namespaceToReconcile)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsGone(err) {
					reqLogger.Info("namespace to configure with mesh is missing", "Namespace", namespaceToReconcile)
				} else {
					allErrors = append(allErrors, err)
				}
			} else {
				instance.Status.ConfiguredMembers = append(instance.Status.ConfiguredMembers, namespaceToReconcile)
			}
		}
		instance.Status.ServiceMeshGeneration = mesh.Status.ObservedGeneration
		instance.Status.ServiceMeshReconciledVersion = mesh.Status.GetReconciledVersion()
	} else if len(unconfiguredMembers) > 0 { // required namespace that was missing has been created
		reqLogger.Info("Reconciling newly created namespaces associated with this ServiceMeshMemberRoll")

		// create reconciler
		reconciler, err := newNamespaceReconciler(r.Client, reqLogger, mesh.Namespace, common.IsCNIEnabled)
		if err != nil {
			return reconcile.Result{}, err
		}

		for namespaceToReconcile := range unconfiguredMembers {
			if namespaceToReconcile == instance.Namespace {
				// we never operate on the control plane namespace
				continue
			}
			err = reconciler.reconcileNamespaceInMesh(namespaceToReconcile)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsGone(err) {
					reqLogger.Info("namespace to configure with mesh is missing")
				} else {
					allErrors = append(allErrors, err)
				}
			} else {
				instance.Status.ConfiguredMembers = append(instance.Status.ConfiguredMembers, namespaceToReconcile)
			}
			// we don't update the ServiceMeshGeneration in case the other members need to be updated
		}
	} else if mesh.Status.GetReconciledVersion() != instance.Status.ServiceMeshReconciledVersion { // service mesh has been updated
		reqLogger.Info("Reconciling ServiceMeshMemberRoll namespaces with new generation of ServiceMeshControlPlane")

		// create reconciler
		reconciler, err := newNamespaceReconciler(r.Client, reqLogger, mesh.Namespace, common.IsCNIEnabled)
		if err != nil {
			return reconcile.Result{}, err
		}

		instance.Status.ConfiguredMembers = make([]string, 0, len(instance.Spec.Members))
		for namespaceToReconcile := range requiredMembers {
			if namespaceToReconcile == instance.Namespace {
				// we never operate on the control plane namespace
				continue
			}
			err = reconciler.reconcileNamespaceInMesh(namespaceToReconcile)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsGone(err) {
					reqLogger.Info("namespace to configure with mesh is missing")
				} else {
					allErrors = append(allErrors, err)
				}
			} else {
				instance.Status.ConfiguredMembers = append(instance.Status.ConfiguredMembers, namespaceToReconcile)
			}
		}
		instance.Status.ServiceMeshGeneration = mesh.Status.ObservedGeneration
		instance.Status.ServiceMeshReconciledVersion = mesh.Status.GetReconciledVersion()
	} else if len(deletedMembers) > 0 { // namespace that was configured has been deleted
		// nothing to do, but we need to update the ConfiguredMembers field
		reqLogger.Info("Removing deleted namespaces from ConfiguredMembers")
		instance.Status.ConfiguredMembers = make([]string, 0, len(instance.Spec.Members))
		for _, member := range instance.Spec.Members {
			if member == instance.Namespace {
				// we never operate on the control plane namespace
				continue
			}
			if _, ok := allNamespaces[member]; ok {
				instance.Status.ConfiguredMembers = append(instance.Status.ConfiguredMembers, member)
			}
		}
	} else {
		// nothing to do
		reqLogger.Info("nothing to reconcile")
		return reconcile.Result{}, nil
	}

	err = utilerrors.NewAggregate(allErrors)
	if err == nil {
		instance.Status.ObservedGeneration = instance.GetGeneration()
		err = r.Client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "error updating status for ServiceMeshMemberRoll")
		}
	}

	// tell Kiali about all the namespaces in the mesh
	err = r.reconcileKiali(instance.Namespace, instance.Status.ConfiguredMembers, reqLogger)

	return reconcile.Result{}, err
}

func (r *ReconcileMemberList) reconcileKiali(kialiCRNamespace string, configuredMembers []string, reqLogger logr.Logger) error {

	reqLogger.Info("Attempting to get Kiali CR", "kialiCRNamespace", kialiCRNamespace)

	kialiCRName := "kiali"
	kialiCR := &unstructured.Unstructured{}
	kialiCR.SetAPIVersion("kiali.io/v1alpha1")
	kialiCR.SetKind("Kiali")
	kialiCR.SetNamespace(kialiCRNamespace)
	kialiCR.SetName(kialiCRName)
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: kialiCRName, Namespace: kialiCRNamespace}, kialiCR)
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

	reqLogger.Info("Updating Kiali CR deployment.accessible_namespaces", "accessibleNamespaces", accessibleNamespaces)

	err = unstructured.SetNestedStringSlice(kialiCR.UnstructuredContent(), accessibleNamespaces, "spec", "deployment", "accessible_namespaces")
	if err != nil {
		return pkgerrors.Wrapf(err, "cannot set deployment.accessible_namespaces in Kiali CR %s/%s", kialiCRNamespace, kialiCRName)
	}

	err = r.Client.Update(context.TODO(), kialiCR)
	if err != nil {
		return pkgerrors.Wrapf(err, "cannot update Kiali CR %s/%s with new accessible namespaces", kialiCRNamespace, kialiCRName)
	}

	reqLogger.Info("Kiali CR deployment.accessible_namespaces updated", "accessibleNamespaces", accessibleNamespaces)
	return nil
}

func (r *ReconcileMemberList) getAllNamespaces() (map[string]struct{}, error) {
	namespaceList := &corev1.NamespaceList{}
	namespaceList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NamespaceList"))
	err := r.Client.List(context.TODO(), nil, namespaceList)
	if err != nil {
		return nil, err
	}
	allNamespaces := map[string]struct{}{}
	for _, namespace := range namespaceList.Items {
		allNamespaces[namespace.Name] = struct{}{}
	}
	return allNamespaces, nil
}

type namespaceReconciler struct {
	client               client.Client
	logger               logr.Logger
	meshNamespace        string
	isCNIEnabled         bool
	networkingStrategy   networkingStrategy
	roleBindingsList     *unstructured.UnstructuredList
	requiredRoleBindings map[string]struct{}
}

func newNamespaceReconciler(client client.Client, logger logr.Logger, meshNamespace string, isCNIEnabled bool) (*namespaceReconciler, error) {
	var err error
	reconciler := &namespaceReconciler{
		client:               client,
		logger:               logger.WithValues("MeshNamespace", meshNamespace),
		meshNamespace:        meshNamespace,
		isCNIEnabled:         isCNIEnabled,
		requiredRoleBindings: map[string]struct{}{},
	}
	err = reconciler.initializeNetworkingStrategy()
	if err != nil {
		return nil, err
	}

	reconciler.roleBindingsList, err = common.FetchOwnedResources(client, rbacv1.SchemeGroupVersion.WithKind("RoleBinding"), meshNamespace, meshNamespace)
	if err != nil {
		reconciler.logger.Error(err, "error retrieving RoleBinding resources for mesh")
		return nil, err
	}
	for _, rb := range reconciler.roleBindingsList.Items {
		reconciler.requiredRoleBindings[rb.GetName()] = struct{}{}
	}
	return reconciler, nil
}

func (r *namespaceReconciler) initializeNetworkingStrategy() error {
	// configure networks
	clusterNetwork := &unstructured.Unstructured{}
	clusterNetwork.SetAPIVersion("network.openshift.io/v1")
	clusterNetwork.SetKind("ClusterNetwork")
	r.networkingStrategy = &subnetStrategy{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Name: "default"}, clusterNetwork)
	if err != nil {
		if errors.IsNotFound(err) {
			r.logger.Info("default cluster network not defined, skipping network configuration")
			return nil
		}
		return err
	}
	networkPlugin, ok, err := unstructured.NestedString(clusterNetwork.UnstructuredContent(), "pluginName")
	if err != nil {
		return pkgerrors.Wrap(err, "cluster network plugin not defined")
	}
	if ok {
		switch networkPlugin {
		case "redhat/openshift-ovs-subnet":
			// nothing to do
		case "redhat/openshift-ovs-networkpolicy":
			r.networkingStrategy, err = newNetworkPolicyStrategy(r)
		case "redhat/openshift-ovs-multitenant":
			r.networkingStrategy, err = newMultitenantStrategy(r)
		default:
			return fmt.Errorf("unsupported cluster network plugin: %s", networkPlugin)
		}
	} else {
		r.logger.Info("cluster network plugin not defined, skipping network configuration")
	}
	return err
}

func (r *namespaceReconciler) removeNamespaceFromMesh(namespace string) error {
	logger := r.logger.WithValues("namespace", namespace)
	logger.Info("cleaning up resources in namespace removed from mesh")

	// get namespace
	namespaceResource := &corev1.Namespace{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource)
	if err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			logger.Info("namespace to remove from mesh is missing")
			return nil
		}
		logger.Error(err, "error retrieving namespace to remove from mesh")
		return err
	}

	allErrors := []error{}

	// XXX: Disable for now.  This should not be required when using CNI plugin
	// remove service accounts from SCC
	// saList, err := common.FetchMeshResources(r.Client, corev1.SchemeGroupVersion.WithKind("ServiceAccount"), meshNamespace, namespace)
	// if err == nil {
	// 	saNames := nameList(saList.Items)
	// 	err = r.RemoveUsersFromSCC("anyuid", saNames...)
	// 	if err != nil {
	// 		logger.Error(err, "error removing ServiceAccounts associated with mesh from anyuid SecurityContextConstraints", "ServiceAccounts", saNames)
	// 		allErrors = append(allErrors, err)
	// 	}
	// 	err = r.RemoveUsersFromSCC("privileged", saNames...)
	// 	if err != nil {
	// 		logger.Error(err, "error removing ServiceAccounts associated with mesh from privileged SecurityContextConstraints", "ServiceAccounts", saNames)
	// 		allErrors = append(allErrors, err)
	// 	}
	// } else {
	// 	logger.Error(err, "error could not retrieve ServiceAccounts associated with mesh")
	// 	allErrors = append(allErrors, err)
	// }

	// delete role bindings
	rbList, err := common.FetchMeshResources(r.client, rbacv1.SchemeGroupVersion.WithKind("RoleBinding"), r.meshNamespace, namespace)
	if err == nil {
		for _, rb := range rbList.Items {
			logger.Info("deleting RoleBinding for mesh ServiceAccount", "RoleBinding", rb.GetName())
			err = r.client.Delete(context.TODO(), &rb)
			if err != nil {
				logger.Error(err, "error removing RoleBinding associated with mesh", "RoleBinding", rb.GetName())
				allErrors = append(allErrors, err)
			}
		}
	} else {
		logger.Error(err, "error could not retrieve RoleBindings associated with mesh")
		allErrors = append(allErrors, err)
	}

	// remove NetworkAttachmentDefinition so that Multus CNI no longer invokes Istio CNI for pods in this namespace
	err = r.removeNetworkAttachmentDefinition(namespace, r.meshNamespace, logger)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// delete network policies
	err = r.networkingStrategy.removeNamespaceFromMesh(namespace)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// remove mesh labels
	// get fresh Namespace from cache to minimize the chance of a conflict during update (the Namespace might have been updated during the execution of removeNamespaceFromMesh())
	namespaceResource = &corev1.Namespace{}
	if err := r.client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource); err == nil {
		common.DeleteLabel(namespaceResource, common.MemberOfKey)
		if err := r.client.Update(context.TODO(), namespaceResource); err == nil {
			logger.Info("Removed member-of label from namespace")
		} else if !(errors.IsGone(err) || errors.IsNotFound(err)) {
			allErrors = append(allErrors, fmt.Errorf("Error removing member-of label from namespace %s: %v", namespace, err))
			return utilerrors.NewAggregate(allErrors)
		}
	} else if !(errors.IsGone(err) || errors.IsNotFound(err)) {
		allErrors = append(allErrors, fmt.Errorf("Error getting namespace %s prior to removing member-of label: %v", namespace, err))
	}

	return utilerrors.NewAggregate(allErrors)
}

func (r *namespaceReconciler) reconcileNamespaceInMesh(namespace string) error {
	logger := r.logger.WithValues("namespace", namespace)
	logger.Info("configuring namespace for use with mesh")

	// get namespace
	namespaceResource := &corev1.Namespace{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource)
	if err != nil {
		return err
	}

	memberOf := ""
	if namespaceResource.Labels != nil {
		memberOf = namespaceResource.Labels[common.MemberOfKey]
	}
	isMemberOfDifferentMesh := memberOf != "" && memberOf != r.meshNamespace
	if isMemberOfDifferentMesh {
		return fmt.Errorf("Cannot reconcile namespace %s in mesh %s, as it is already a member of %s", namespace, r.meshNamespace, memberOf)
	}

	// configure networking
	err = r.networkingStrategy.reconcileNamespaceInMesh(namespace)
	if err != nil {
		return err
	}

	allErrors := []error{}

	// add role bindings
	err = r.reconcileRoleBindings(namespace, r.logger)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	if r.isCNIEnabled {
		// add NetworkAttachmentDefinition to tell Multus to invoke Istio CNI for pods in this namespace
		err = r.addNetworkAttachmentDefinition(namespace, r.meshNamespace, logger)
	} else {
		err = r.removeNetworkAttachmentDefinition(namespace, r.meshNamespace, logger)
	}
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// XXX: Disable for now.  This should not be required when using CNI plugin
	// add service accounts to SCC
	// err = r.reconcilePodServiceAccounts(namespace, mesh, logger)
	// if err != nil {
	// 	allErrors = append(allErrors, err)
	// }

	// add mesh labels
	if !common.HasLabel(namespaceResource, common.MemberOfKey) {
		// get fresh Namespace from cache to minimize the chance of a conflict during update (the Namespace might have been updated during the execution of reconcileNamespaceInMesh())
		namespaceResource = &corev1.Namespace{}
		if err := r.client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource); err == nil {
			common.SetLabel(namespaceResource, common.MemberOfKey, r.meshNamespace)
			if err := r.client.Update(context.TODO(), namespaceResource); err == nil {
				logger.Info("Added member-of label to namespace")
			} else {
				allErrors = append(allErrors, fmt.Errorf("Error adding member-of label to namespace %s: %v", namespace, err))
			}
		} else {
			allErrors = append(allErrors, fmt.Errorf("Error getting namespace %s prior to adding member-of label: %v", namespace, err))
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

func (r *namespaceReconciler) reconcileRoleBindings(namespace string, reqLogger logr.Logger) error {
	namespaceRoleBindings, err := common.FetchMeshResources(r.client, rbacv1.SchemeGroupVersion.WithKind("RoleBinding"), r.meshNamespace, namespace)
	if err != nil {
		reqLogger.Error(err, "error retrieving RoleBinding resources for namespace")
		return err
	}

	allErrors := []error{}

	// add required role bindings
	existingRoleBindings := nameSet(namespaceRoleBindings.Items)
	addedRoleBindings := map[string]struct{}{}
	for _, meshRoleBinding := range r.roleBindingsList.Items {
		roleBindingName := meshRoleBinding.GetName()
		if _, ok := existingRoleBindings[roleBindingName]; !ok {
			reqLogger.Info("creating RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
			roleBinding := &unstructured.Unstructured{}
			roleBinding.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
			roleBinding.SetNamespace(namespace)
			roleBinding.SetName(meshRoleBinding.GetName())
			roleBinding.SetLabels(meshRoleBinding.GetLabels())
			roleBinding.SetAnnotations(meshRoleBinding.GetAnnotations())
			if subjects, ok, _ := unstructured.NestedSlice(meshRoleBinding.UnstructuredContent(), "subjects"); ok {
				unstructured.SetNestedSlice(roleBinding.UnstructuredContent(), subjects, "subjects")
			}
			if roleRef, ok, _ := unstructured.NestedFieldNoCopy(meshRoleBinding.UnstructuredContent(), "roleRef"); ok {
				unstructured.SetNestedField(roleBinding.UnstructuredContent(), roleRef, "roleRef")
			}
			common.SetLabel(roleBinding, common.MemberOfKey, r.meshNamespace)
			err = r.client.Create(context.TODO(), roleBinding)
			if err == nil {
				addedRoleBindings[roleBindingName] = struct{}{}
			} else {
				reqLogger.Error(err, "error creating RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
				allErrors = append(allErrors, err)
			}
		} // XXX: else if existingRoleBinding.annotations[mesh-generation] != meshRoleBinding.annotations[generation] then update?
	}

	existingRoleBindings = union(existingRoleBindings, addedRoleBindings)

	// delete obsolete role bindings
	for roleBindingName := range difference(existingRoleBindings, r.requiredRoleBindings) {
		reqLogger.Info("deleting RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
		roleBinding := &unstructured.Unstructured{}
		roleBinding.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
		roleBinding.SetName(roleBindingName)
		roleBinding.SetNamespace(namespace)
		err = r.client.Delete(context.TODO(), roleBinding, client.PropagationPolicy(metav1.DeletePropagationForeground))
		if err != nil && !(errors.IsNotFound(err) || errors.IsGone(err)) {
			reqLogger.Error(err, "error deleting RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
			allErrors = append(allErrors, err)
		}
	}

	// if there were errors, we've logged them and there's not really anything we can do, as we're in an uncertain state
	// maybe a following reconcile will add the required role binding that failed.  if it was a delete that failed, we're
	// just leaving behind some cruft.
	return utilerrors.NewAggregate(allErrors)
}

func (r *namespaceReconciler) addNetworkAttachmentDefinition(namespace, meshNamespace string, reqLogger logr.Logger) error {
	netAttachDef := &unstructured.Unstructured{}
	netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})

	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: netAttachDefName}, netAttachDef)
	if err == nil {
		// resource exists, do nothing
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("Could not get NetworkAttachmentDefinition %s/%s: %v", namespace, netAttachDefName, err)
	}

	// TODO: update resource if its state isn't what we want

	netAttachDef.SetNamespace(namespace)
	netAttachDef.SetName(netAttachDefName)
	err = r.client.Create(context.TODO(), netAttachDef)
	if err != nil {
		return fmt.Errorf("Could not create NetworkAttachmentDefinition %s/%s: %v", namespace, netAttachDefName, err)
	}
	return nil
}

func (r *namespaceReconciler) removeNetworkAttachmentDefinition(namespace, meshNamespace string, reqLogger logr.Logger) error {
	netAttachDef := &unstructured.Unstructured{}
	netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})

	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: netAttachDefName}, netAttachDef)
	if err != nil {
		if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
			// resource doesn't exist, so everything's fine
			return nil
		}
		return fmt.Errorf("Could not get NetworkAttachmentDefinition %s/%s: %v", namespace, netAttachDefName, err)
	}

	err = r.client.Delete(context.TODO(), netAttachDef, client.PropagationPolicy(metav1.DeletePropagationOrphan))
	if err == nil {
		// resource successfully deleted
		return nil
	}
	if !errors.IsNotFound(err) {
		// resource was deleted between our Get call and our Delete call - everything is fine
		return nil
	}
	return fmt.Errorf("Could not delete NetworkAttachmentDefinition %s/%s: %v", namespace, netAttachDefName, err)
}

func (r *ReconcileMemberList) reconcilePodServiceAccounts(namespace string, mesh *v1.ServiceMeshControlPlane, reqLogger logr.Logger) error {
	// scan for pods with injection labels
	serviceAccounts := map[string]struct{}{}
	podList := &unstructured.UnstructuredList{}
	podList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("PodList"))
	err := r.Client.List(context.TODO(), client.InNamespace(namespace), podList)
	if err == nil {
		// update account privileges used by deployments with injection labels
		for _, pod := range podList.Items {
			if val, ok := pod.GetAnnotations()["sidecar.istio.io/inject"]; ok && (val == "y" || val == "yes" || val == "true" || val == "on") {
				// XXX: this is pretty hacky.  we need to recreate the logic that determines whether or not injection is
				// enabled on the pod.  maybe we just have the user add ServiceAccounts to the ServiceMeshMember spec
				if podSA, ok, err := unstructured.NestedString(pod.UnstructuredContent(), "spec", "serviceAccountName"); ok || err == nil {
					if len(podSA) == 0 {
						podSA = "default"
					}
					serviceAccounts[podSA] = struct{}{}
				}
			}
		}
	} else {
		// skip trying to add, but delete whatever's left
		reqLogger.Error(err, "cannot update ServiceAccount SCC settings: error occurred scanning for Pods")
	}

	meshServiceAccounts, err := common.FetchMeshResources(r.Client, corev1.SchemeGroupVersion.WithKind("ServiceAccount"), mesh.Namespace, namespace)
	currentlyManagedServiceAccounts := nameSet(meshServiceAccounts.Items)
	if err != nil {
		// worst case, we'll try to associate the service accounts again
		reqLogger.Error(err, "cannot list ServiceAcccounts configured for use with mesh")
	}

	allErrors := []error{}

	if len(serviceAccounts) > 0 {
		// add labels before we add the ServiceAccount to the SCCs
		erroredServiceAccounts := map[string]struct{}{}
		for saName := range serviceAccounts {
			if _, ok := currentlyManagedServiceAccounts[saName]; ok {
				continue
			}
			saResource := &unstructured.Unstructured{}
			saResource.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ServiceAccount"))
			err = r.Client.Get(context.TODO(), client.ObjectKey{Name: saName, Namespace: namespace}, saResource)
			if err != nil {
				erroredServiceAccounts[saName] = struct{}{}
				reqLogger.Error(err, "error retrieving ServiceAccount to configure SCC", "ServiceAccount", saName)
				allErrors = append(allErrors, err)
			} else if !common.HasLabel(saResource, common.MemberOfKey) {
				common.SetLabel(saResource, common.MemberOfKey, mesh.Namespace)
				err = r.Client.Update(context.TODO(), saResource)
				if err != nil {
					erroredServiceAccounts[saName] = struct{}{}
					reqLogger.Error(err, "error setting label on ServiceAccount to configure SCC", "ServiceAccount", saName)
					allErrors = append(allErrors, err)
				}
			}
		}

		// XXX: use privileged and anyuid for now
		serviceAccountsToUpdate := toList(difference(serviceAccounts, erroredServiceAccounts))
		_, err = r.AddUsersToSCC("privileged", serviceAccountsToUpdate...)
		if err != nil {
			reqLogger.Error(err, "error adding ServiceAccounts to privileged SecurityContextConstraints", "ServiceAccounts", serviceAccountsToUpdate)
			allErrors = append(allErrors, err)
		}
		_, err = r.AddUsersToSCC("anyuid", serviceAccountsToUpdate...)
		if err != nil {
			reqLogger.Error(err, "error adding ServiceAccounts to anyuid SecurityContextConstraints", "ServiceAccounts", serviceAccountsToUpdate)
			allErrors = append(allErrors, err)
		}
	}

	// remove unused service accounts that may have been previously configured
	removedServiceAccounts := difference(currentlyManagedServiceAccounts, serviceAccounts)
	removedServiceAccountsList := toList(removedServiceAccounts)
	if err := r.RemoveUsersFromSCC("privileged", removedServiceAccountsList...); err != nil {
		reqLogger.Error(err, "error removing unused ServiceAccounts from privileged SecurityContextConstraints", "ServiceAccounts", removedServiceAccountsList)
		allErrors = append(allErrors, err)
	}
	if err := r.RemoveUsersFromSCC("anyuid", removedServiceAccountsList...); err != nil {
		reqLogger.Error(err, "error removing unused ServiceAccounts from anyuid SecurityContextConstraints", "ServiceAccounts", removedServiceAccountsList)
		allErrors = append(allErrors, err)
	}

	// Remove the labels, now that we've removed them from the SCCs
	for _, saResource := range meshServiceAccounts.Items {
		if _, ok := removedServiceAccounts[saResource.GetName()]; !ok {
			continue
		}
		common.DeleteLabel(&saResource, common.MemberOfKey)
		err = r.Client.Update(context.TODO(), &saResource)
		if err != nil {
			reqLogger.Error(err, "error removing member-of label from ServiceAccount", "ServiceAccount", saResource.GetName())
			// don't return these errors
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

type networkingStrategy interface {
	reconcileNamespaceInMesh(namespace string) error
	removeNamespaceFromMesh(namespace string) error
}

func toSet(values []string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func toList(set map[string]struct{}) []string {
	list := make([]string, 0, len(set))
	for val := range set {
		list = append(list, val)
	}
	return list
}

func difference(source, remove map[string]struct{}) map[string]struct{} {
	diff := map[string]struct{}{}
	for val := range source {
		if _, ok := remove[val]; !ok {
			diff[val] = struct{}{}
		}
	}
	return diff
}

func intersection(set1, set2 map[string]struct{}) map[string]struct{} {
	commonSet := map[string]struct{}{}
	if len(set1) > len(set2) {
		temp := set1
		set1 = set2
		set2 = temp
	}
	for val := range set1 {
		if _, ok := set2[val]; ok {
			commonSet[val] = struct{}{}
		}
	}
	return commonSet
}

func union(set1, set2 map[string]struct{}) map[string]struct{} {
	unionSet := map[string]struct{}{}
	for val := range set1 {
		unionSet[val] = struct{}{}
	}
	for val := range set2 {
		unionSet[val] = struct{}{}
	}
	return unionSet
}

func nameList(items []unstructured.Unstructured) []string {
	list := make([]string, 0, len(items))
	for _, object := range items {
		list = append(list, object.GetName())
	}
	return list
}

func nameSet(items []unstructured.Unstructured) map[string]struct{} {
	set := map[string]struct{}{}
	for _, object := range items {
		set[object.GetName()] = struct{}{}
	}
	return set
}
