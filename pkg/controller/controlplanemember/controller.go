package controlplanemember

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	istiov1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/v1alpha3"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_controlplane")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ControlPlaneMember Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileControlPlaneMember{ResourceManager: common.ResourceManager{Client: mgr.GetClient(), Log: log}, scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("controlplanemember-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ControlPlaneMember
	err = c.Watch(&source.Kind{Type: &istiov1alpha3.ControlPlaneMember{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileControlPlaneMember{}

// ReconcileControlPlaneMember reconciles a ControlPlaneMember object
type ReconcileControlPlaneMember struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	common.ResourceManager
	scheme *runtime.Scheme
}

const (
	finalizer = "istio-operator-ControlPlaneMember"
)

// Reconcile reads that state of the cluster for a ControlPlaneMember object and makes changes based on the state read
// and what is in the ControlPlaneMember.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileControlPlaneMember) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Processing ControlPlaneMember")

	// Fetch the ControlPlaneMember instance
	instance := &istiov1alpha3.ControlPlaneMember{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	reqLogger = reqLogger.WithValues("mesh", instance.Name)
	if instance.GetGeneration() == instance.Status.ObservedGeneration {
		reqLogger.Info("nothing to reconcile, generations match")
		return reconcile.Result{}, nil
	}

	deleted := instance.GetDeletionTimestamp() != nil
	finalizers := instance.GetFinalizers()
	finalizerIndex := common.IndexOf(finalizers, finalizer)
	if deleted {
		if finalizerIndex < 0 {
			return reconcile.Result{}, nil
		}
		reqLogger.Info("Deleting ControlPlaneMember")
		r.deleteServiceAccountsFromSCC(instance, reqLogger)
		r.deleteRoleBinding(instance, reqLogger)
		// XXX: for now, nuke the resources, regardless of errors
		finalizers = append(finalizers[:finalizerIndex], finalizers[finalizerIndex+1:]...)
		instance.SetFinalizers(finalizers)
		_ = r.Client.Update(context.TODO(), instance)
		return reconcile.Result{}, nil
	} else if finalizerIndex < 0 {
		reqLogger.V(1).Info("Adding finalizer", "finalizer", finalizer)
		finalizers = append(finalizers, finalizer)
		instance.SetFinalizers(finalizers)
		err = r.Client.Update(context.TODO(), instance)
		return reconcile.Result{Requeue: err == nil}, err
	}

	reqLogger.Info("Reconciling ControlPlaneMember")

	meshList := &istiov1alpha3.ControlPlaneList{}
	err = r.Client.List(context.TODO(), client.InNamespace(instance.Name), meshList)
	if err != nil {
		return reconcile.Result{}, err
	}
	if len(meshList.Items) != 1 {
		reqLogger.Error(nil, "cannot reconcile ControlPlaneMember: multiple ControlPlaneMember resources exist in project")
		return reconcile.Result{}, fmt.Errorf("failed to locate single ControlPlane for project %s", instance.Namespace)
	}

	mesh := meshList.Items[0]

	if mesh.GetGeneration() == 0 {
		// wait for the mesh to be installed
		return reconcile.Result{}, nil
	}

	if mesh.GetGeneration() != instance.Status.MeshGeneration {
		// setup mesh
		// update role bindings
		err = r.reconcileRoleBindings(instance, reqLogger)
		if err != nil {
			// bail
			reqLogger.Error(err, "error reconciling RoleBinding for mesh")
			return reconcile.Result{}, err
		}
		instance.Status.MeshGeneration = mesh.GetGeneration()
	}

	// configure ServiceAccounts used by Pods configured for injection
	err = r.reconcilePodServiceAccounts(instance, reqLogger)

	if err != nil {
		r.Client.Status().Update(context.TODO(), instance)
	}

	return reconcile.Result{}, err
}

func (r *ReconcileControlPlaneMember) reconcileRoleBindings(instance *istiov1alpha3.ControlPlaneMember, reqLogger logr.Logger) error {
	meshRoleBindingsList := &unstructured.UnstructuredList{}
	meshRoleBindingsList.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBindingList"))
	err := r.Client.List(context.TODO(), client.InNamespace(instance.Name), meshRoleBindingsList)
	if err != nil {
		reqLogger.Error(err, "could not read RoleBinding resources for mesh")
		return err
	}

	owner := metav1.NewControllerRef(instance, istiov1alpha3.SchemeGroupVersion.WithKind("ControlPlaneMember"))
	ownerRefs := []metav1.OwnerReference{*owner}

	// add required role bindings
	existingRoleBindings := toSet(instance.Status.ManagedRoleBindings)
	addedRoleBindings := map[string]struct{}{}
	requiredRoleBindings := map[string]struct{}{}
	for _, meshRoleBinding := range meshRoleBindingsList.Items {
		name := meshRoleBinding.GetName()
		if _, ok := existingRoleBindings[name]; !ok {
			reqLogger.Info("creating RoleBinding for mesh")
			meshRoleBinding.SetNamespace(instance.Name)
			meshRoleBinding.SetOwnerReferences(ownerRefs)
			err = r.Client.Create(context.TODO(), &meshRoleBinding)
			if err == nil {
				addedRoleBindings[name] = struct{}{}
			} else {
				reqLogger.Error(err, "error creating RoleBinding for mesh")
			}
		}
		requiredRoleBindings[name] = struct{}{}
	}

	existingRoleBindings = merge(existingRoleBindings, addedRoleBindings)

	// delete obsolete role bindings
	for roleBindingName := range difference(existingRoleBindings, requiredRoleBindings) {
		r.Log.Info("deleting RoleBinding for mesh")
		roleBinding := &unstructured.Unstructured{}
		roleBinding.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
		roleBinding.SetName(roleBindingName)
		roleBinding.SetNamespace(instance.Namespace)
		err = r.Client.Delete(context.TODO(), roleBinding, client.PropagationPolicy(metav1.DeletePropagationForeground))
		if err == nil {
			delete(existingRoleBindings, roleBindingName)
		} else {
			reqLogger.Error(err, "error deleting RoleBinding for mesh")
		}
	}

	instance.Status.ManagedRoleBindings = toList(existingRoleBindings)
	sort.Strings(instance.Status.ManagedRoleBindings)

	// if there were errors, we've logged them and there's not really anything we can do, as we're in an uncertain state
	// maybe a following reconcile will add the required role binding that failed.  if it was a delete that failed, we're
	// just leaving behind some cruft.
	return nil
}

func (r *ReconcileControlPlaneMember) reconcilePodServiceAccounts(instance *istiov1alpha3.ControlPlaneMember, reqLogger logr.Logger) error {
	// scan for pods with injection labels
	podList := &unstructured.UnstructuredList{}
	podList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("PodList"))
	err := r.Client.List(context.TODO(), client.InNamespace(instance.Name), podList)
	if err != nil {
		// XXX: is this the right thing to do, or whould we remove SAs from SCC?
		reqLogger.Error(err, "cannot update ServiceAccount SCC settings: error occurred scanning for Pods")
		return err
	}

	// update account privileges used by deployments with injection labels
	serviceAccounts := map[string]struct{}{}
	for _, pod := range podList.Items {
		if val, ok := pod.GetAnnotations()["sidecar.istio.io/inject"]; ok && (val == "y" || val == "yes" || val == "true" || val == "on") {
			// XXX: this is pretty hacky.  we need to recreate the logic that determines whether or not injection is
			// enabled on the pod.  maybe we just have the user add SAs to the ControlPlaneMember spec
			if podSA, ok, err := unstructured.NestedString(pod.UnstructuredContent(), "spec", "serviceAccountName"); ok || err == nil {
				if len(podSA) == 0 {
					podSA = "default"
				}
				serviceAccounts[podSA] = struct{}{}
			}
		}
	}
	currentlyManagedServiceAccounts := toSet(instance.Status.ManagedServiceAccounts)
	removedServiceAccounts := difference(currentlyManagedServiceAccounts, serviceAccounts)
	// XXX: use privileged and anyuid for now
	addedServiceAccounts, err := r.AddUsersToSCC("privileged", toList(serviceAccounts)...)
	//err = r.AddUsersToSCC(fmt.Sprintf("istio-init-%s", instance.Name), keyList(serviceAccounts)...)

	// always update managed list to prevent removal of a user managed SAs
	currentlyManagedServiceAccounts = merge(currentlyManagedServiceAccounts, toSet(addedServiceAccounts))

	// remove unused service accounts that may have been previously configured
	if err := r.RemoveUsersFromSCC("privileged", toList(removedServiceAccounts)...); err != nil {
		reqLogger.Error(err, "error removing unused ServiceAccounts from privileged SecurityContextConstraints")
	} else {
		// remove what we removed
		currentlyManagedServiceAccounts = difference(currentlyManagedServiceAccounts, removedServiceAccounts)
	}

	// update status
	instance.Status.ManagedServiceAccounts = toList(currentlyManagedServiceAccounts)
	sort.Strings(instance.Status.ManagedServiceAccounts)

	if instance.Status.ObservedGeneration == 0 {
		// force recreation of pods to trigger injection
	}
	return err
}

func (r *ReconcileControlPlaneMember) deleteServiceAccountsFromSCC(instance *istiov1alpha3.ControlPlaneMember, reqLogger logr.Logger) error {
	return r.RemoveUsersFromSCC("privileged", instance.Status.ManagedServiceAccounts...)
}

func (r *ReconcileControlPlaneMember) deleteRoleBinding(instance *istiov1alpha3.ControlPlaneMember, reqLogger logr.Logger) error {
	roleBinding := &unstructured.Unstructured{}
	roleBinding.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
	roleBinding.SetName(fmt.Sprintf("istio-mesh-role-binding"))
	roleBinding.SetNamespace(instance.GetNamespace())
	return r.Client.Delete(context.TODO(), roleBinding, client.PropagationPolicy(metav1.DeletePropagationForeground))
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

func merge(set1, set2 map[string]struct{}) map[string]struct{} {
	merged := map[string]struct{}{}
	for val := range set1 {
		merged[val] = struct{}{}
	}
	for val := range set2 {
		merged[val] = struct{}{}
	}
	return merged
}
