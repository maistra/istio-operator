package memberroll

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

var log = logf.Log.WithName("controller_servicemeshmemberroll")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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
		ToRequests: handler.ToRequestsFunc(func(ns handler.MapObject) []reconcile.Request {
			list := &v1.ServiceMeshMemberRollList{}
			err := mgr.GetClient().List(context.TODO(), client.InNamespace(ns.Meta.GetNamespace()), list)
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

const (
	finalizer = "istio-operator-MemberRoll"
)

// Reconcile reads that state of the cluster for a ServiceMeshMemberRoll object and makes changes based on the state read
// and what is in the ServiceMeshMemberRoll.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMemberList) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
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
	finalizers := instance.GetFinalizers()
	finalizerIndex := common.IndexOf(finalizers, finalizer)
	if deleted {
		if finalizerIndex < 0 {
			reqLogger.Info("ServiceMeshMemberRoll deleted")
			return reconcile.Result{}, nil
		}
		reqLogger.Info("Deleting ServiceMeshMemberRoll")
		for _, namespace := range instance.Spec.Members {
			err := r.removeNamespaceFromMesh(namespace, instance.Namespace, reqLogger)
			if err != nil && !(errors.IsNotFound(err) || errors.IsGone(err)) {
				reqLogger.Error(err, "error cleaning up mesh member namespace")
				// XXX: do we prevent removing the finalizer?
			}
		}
		// XXX: for now, nuke the resources, regardless of errors
		for tries := 0; tries < 5; tries++ {
			finalizers = append(finalizers[:finalizerIndex], finalizers[finalizerIndex+1:]...)
			instance.SetFinalizers(finalizers)
			err = r.Client.Update(context.TODO(), instance)
			if err != nil {
				if errors.IsConflict(err) {
					instance = &v1.ServiceMeshMemberRoll{}
					err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
					if err == nil {
						finalizers = instance.GetFinalizers()
						finalizerIndex = common.IndexOf(finalizers, finalizer)
						if finalizerIndex >= 0 {
							continue
						}
					}
				}
			}
			break
		}

		// tell Kiali that MT mode is disabled. This allows Kiali to see the full cluster.
		r.reconcileKiali(instance.Namespace, nil, reqLogger)

		return reconcile.Result{}, err
	} else if finalizerIndex < 0 {
		reqLogger.Info("Adding finalizer to ServiceMeshMemberRoll", "finalizer", finalizer)
		finalizers = append(finalizers, finalizer)
		instance.SetFinalizers(finalizers)
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
			reqLogger.Error(nil, "cannot reconcile ServiceMeshControlPlane: multiple ServiceMeshControlPlane resources exist in project")
		} else {
			reqLogger.Error(nil, fmt.Sprintf("failed to locate ServiceMeshControlPlane for project %s", instance.Namespace))
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
		reqLogger.Error(nil, "skipping reconciliation because mesh is not in a known good state")
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
	unconfiguredMembers := intersection(difference(requiredMembers, configuredMembers), allNamespaces)
	deletedMembers := difference(configuredMembers, allNamespaces)
	if instance.Generation != instance.Status.ObservedGeneration { // member roll has been updated

		reqLogger.Info("Reconciling new generation of ServiceMeshMemberRoll")

		instance.Status.ConfiguredMembers = make([]string, 0, len(instance.Spec.Members))

		// setup projects
		configuredNamespaces, err := common.FetchMeshResources(r.Client, corev1.SchemeGroupVersion.WithKind("Namespace"), mesh.Namespace, "")
		if err != nil {
			reqLogger.Error(err, "error listing mesh member namespaces")
			return reconcile.Result{}, err
		}
		existingMembers := nameSet(configuredNamespaces.Items)
		for namespaceToRemove := range difference(existingMembers, requiredMembers) {
			if namespaceToRemove == instance.Namespace {
				// we never operate on the control plane namespace
				continue
			}
			err = r.removeNamespaceFromMesh(namespaceToRemove, mesh.Namespace, reqLogger)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
		for namespaceToReconcile := range requiredMembers {
			if namespaceToReconcile == instance.Namespace {
				// we never operate on the control plane namespace
				reqLogger.Error(nil, "ignoring control plane namespace in members list of ServiceMeshMemberRoll")
				continue
			}
			err = r.reconcileNamespaceInMesh(namespaceToReconcile, &mesh, reqLogger)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsGone(err) {
					reqLogger.Error(nil, "namespace to configure with mesh is missing")
				} else {
					allErrors = append(allErrors, err)
				}
			} else {
				instance.Status.ConfiguredMembers = append(instance.Status.ConfiguredMembers, namespaceToReconcile)
			}
		}
		instance.Status.ServiceMeshGeneration = mesh.Status.ObservedGeneration
	} else if len(unconfiguredMembers) > 0 { // required namespace that was missing has been created
		reqLogger.Info("Reconciling newly created namespaces associated with this ServiceMeshMemberRoll")
		for namespaceToReconcile := range unconfiguredMembers {
			if namespaceToReconcile == instance.Namespace {
				// we never operate on the control plane namespace
				continue
			}
			err = r.reconcileNamespaceInMesh(namespaceToReconcile, &mesh, reqLogger)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsGone(err) {
					reqLogger.Error(nil, "namespace to configure with mesh is missing")
				} else {
					allErrors = append(allErrors, err)
				}
			} else {
				instance.Status.ConfiguredMembers = append(instance.Status.ConfiguredMembers, namespaceToReconcile)
			}
			// we don't update the ServiceMeshGeneration in case the other members need to be updated
		}
	} else if mesh.Status.ObservedGeneration != instance.Status.ServiceMeshGeneration { // service mesh has been updated
		reqLogger.Info("Reconciling ServiceMeshMemberRoll namespaces with new generation of ServiceMeshControlPlane")
		instance.Status.ConfiguredMembers = make([]string, 0, len(instance.Spec.Members))
		for namespaceToReconcile := range requiredMembers {
			if namespaceToReconcile == instance.Namespace {
				// we never operate on the control plane namespace
				continue
			}
			err = r.reconcileNamespaceInMesh(namespaceToReconcile, &mesh, reqLogger)
			if err != nil {
				if errors.IsNotFound(err) || errors.IsGone(err) {
					reqLogger.Error(nil, "namespace to configure with mesh is missing")
				} else {
					allErrors = append(allErrors, err)
				}
			} else {
				instance.Status.ConfiguredMembers = append(instance.Status.ConfiguredMembers, namespaceToReconcile)
			}
		}
		instance.Status.ServiceMeshGeneration = mesh.Status.ObservedGeneration
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
	r.reconcileKiali(instance.Namespace, requiredMembers, reqLogger)

	return reconcile.Result{}, err
}

func (r *ReconcileMemberList) reconcileKiali(kialiCRNamespace string, requiredMembers map[string]struct{}, reqLogger logr.Logger) error {

	reqLogger.Info("Attempting to get Kiali CR", "kialiCRNamespace", kialiCRNamespace)

	kialiCR := &unstructured.Unstructured{}
	kialiCR.SetAPIVersion("kiali.io/v1alpha1")
	kialiCR.SetKind("Kiali")
	kialiCR.SetNamespace(kialiCRNamespace)
	kialiCR.SetName("kiali")
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: "kiali", Namespace: kialiCRNamespace}, kialiCR)
	if err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			reqLogger.Info("Kiali CR does not exist, Kiali probably not enabled")
			return nil
		}
		reqLogger.Error(err, "error retrieving Kiali CR from mesh")
		return err
	}

	// just get an array of strings consisting of the list of namespaces to be accessible to Kiali
	var accessibleNamespaces []string
	if len(requiredMembers) == 0 {
		// we are not in multitenency mode - kiali can access the entire cluster
		accessibleNamespaces = []string{"**"}
	} else {
		// we are in multitenency mode
		accessibleNamespaces = make([]string, 0, len(requiredMembers))
		for key := range requiredMembers {
			accessibleNamespaces = append(accessibleNamespaces, key)
		}
	}

	reqLogger.Info("Updating Kiali CR deployment.accessible_namespaces", "accessibleNamespaces", accessibleNamespaces)

	err = unstructured.SetNestedStringSlice(kialiCR.UnstructuredContent(), accessibleNamespaces, "spec", "deployment", "accessible_namespaces")
	if err != nil {
		reqLogger.Error(err, "cannot set deployment.accessible_namespaces in Kiali CR", "kialiCRNamespace", kialiCRNamespace)
	}

	err = r.Client.Update(context.TODO(), kialiCR)
	if err != nil {
		reqLogger.Error(err, "cannot update Kiali CR with new accessible namespaces", "kialiCRNamespace", kialiCRNamespace)
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

func (r *ReconcileMemberList) removeNamespaceFromMesh(namespace string, meshNamespace string, reqLogger logr.Logger) error {
	logger := reqLogger.WithValues("namespace", namespace)
	logger.Info("cleaning up resources in namespace removed from mesh")

	// get namespace
	namespaceResource := &unstructured.Unstructured{}
	namespaceResource.SetAPIVersion(corev1.SchemeGroupVersion.String())
	namespaceResource.SetKind("Namespace")
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource)
	if err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			logger.Error(nil, "namespace to remove from mesh is missing")
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
	rbList, err := common.FetchMeshResources(r.Client, rbacv1.SchemeGroupVersion.WithKind("RoleBinding"), meshNamespace, namespace)
	if err == nil {
		for _, rb := range rbList.Items {
			logger.Info("deleting RoleBinding for mesh ServiceAccount", "RoleBinding", rb.GetName())
			err = r.Client.Delete(context.TODO(), &rb)
			if err != nil {
				logger.Error(err, "error removing RoleBinding associated with mesh", "RoleBinding", rb.GetName())
				allErrors = append(allErrors, err)
			}
		}
	} else {
		logger.Error(err, "error could not retrieve RoleBindings associated with mesh")
		allErrors = append(allErrors, err)
	}

	// delete network policies

	// remove mesh labels
	for tries := 0; tries < 5; tries++ {
		common.DeleteLabel(namespaceResource, common.MemberOfKey)
		common.DeleteLabel(namespaceResource, common.LegacyMemberOfKey)
		err = r.Client.Update(context.TODO(), namespaceResource)
		if err != nil {
			if errors.IsConflict(err) {
				namespaceResource = &unstructured.Unstructured{}
				namespaceResource.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
				err := r.Client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource)
				if err == nil {
					continue
				}
			}
			allErrors = append(allErrors, err)
		}
		break
	}
	if err != nil {
		logger.Error(err, "error removing member-of label from member namespace")
		allErrors = append(allErrors, err)
	}

	return utilerrors.NewAggregate(allErrors)
}

func (r *ReconcileMemberList) reconcileNamespaceInMesh(namespace string, mesh *v1.ServiceMeshControlPlane, reqLogger logr.Logger) error {
	logger := reqLogger.WithValues("namespace", namespace)
	logger.Info("configuring namespace for use with mesh")

	// get namespace
	namespaceResource := &unstructured.Unstructured{}
	namespaceResource.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource)
	if err != nil {
		return err
	}

	allErrors := []error{}

	// add network policies

	// add role bindings
	err = r.reconcileRoleBindings(namespace, mesh, logger)
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
		for tries := 0; tries < 5; tries++ {
			common.SetLabel(namespaceResource, common.MemberOfKey, mesh.Namespace)
			common.SetLabel(namespaceResource, common.LegacyMemberOfKey, mesh.Namespace)
			err = r.Client.Update(context.TODO(), namespaceResource)
			if err != nil {
				if errors.IsConflict(err) {
					namespaceResource = &unstructured.Unstructured{}
					namespaceResource.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
					err := r.Client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource)
					if err == nil {
						continue
					}
				}
				allErrors = append(allErrors, err)
			}
			break
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

func (r *ReconcileMemberList) reconcileRoleBindings(namespace string, mesh *v1.ServiceMeshControlPlane, reqLogger logr.Logger) error {
	reqLogger.Info("reconciling RoleBinding resources for namespace")
	meshRoleBindingsList, err := common.FetchOwnedResources(r.Client, rbacv1.SchemeGroupVersion.WithKind("RoleBinding"), mesh.Namespace, mesh.Namespace)
	if err != nil {
		reqLogger.Error(err, "error retrieving RoleBinding resources for mesh")
		return err
	}

	namespaceRoleBindings, err := common.FetchMeshResources(r.Client, rbacv1.SchemeGroupVersion.WithKind("RoleBinding"), mesh.Namespace, namespace)
	if err != nil {
		reqLogger.Error(err, "error retrieving RoleBinding resources for namespace")
		return err
	}

	allErrors := []error{}

	// add required role bindings
	existingRoleBindings := nameSet(namespaceRoleBindings.Items)
	addedRoleBindings := map[string]struct{}{}
	requiredRoleBindings := map[string]struct{}{}
	for _, meshRoleBinding := range meshRoleBindingsList.Items {
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
			common.SetLabel(roleBinding, common.MemberOfKey, mesh.Namespace)
			err = r.Client.Create(context.TODO(), roleBinding)
			if err == nil {
				addedRoleBindings[roleBindingName] = struct{}{}
			} else {
				reqLogger.Error(err, "error creating RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
				allErrors = append(allErrors, err)
			}
		} // XXX: else if existingRoleBinding.annotations[mesh-generation] != meshRoleBinding.annotations[generation] then update?
		requiredRoleBindings[roleBindingName] = struct{}{}
	}

	existingRoleBindings = union(existingRoleBindings, addedRoleBindings)

	// delete obsolete role bindings
	for roleBindingName := range difference(existingRoleBindings, requiredRoleBindings) {
		r.Log.Info("deleting RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
		roleBinding := &unstructured.Unstructured{}
		roleBinding.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
		roleBinding.SetName(roleBindingName)
		roleBinding.SetNamespace(namespace)
		err = r.Client.Delete(context.TODO(), roleBinding, client.PropagationPolicy(metav1.DeletePropagationForeground))
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
