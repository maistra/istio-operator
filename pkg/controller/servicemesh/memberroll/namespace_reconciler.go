package memberroll

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/maistra/istio-operator/pkg/controller/common"

	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type namespaceReconciler struct {
	common.ControllerResources
	meshNamespace        string
	meshVersion          string
	isCNIEnabled         bool
	networkingStrategy   NamespaceReconciler
	roleBindingsList     rbac.RoleBindingList
	requiredRoleBindings sets.String
}

func newNamespaceReconciler(cl client.Client, logger logr.Logger, meshNamespace string, meshVersion string, isCNIEnabled bool) (NamespaceReconciler, error) {
	reconciler := &namespaceReconciler{
		ControllerResources: common.ControllerResources{
			Client: cl,
			Log:    logger.WithValues("MeshNamespace", meshNamespace),
		},
		meshNamespace:        meshNamespace,
		meshVersion:          meshVersion,
		isCNIEnabled:         isCNIEnabled,
		roleBindingsList:     rbac.RoleBindingList{},
		requiredRoleBindings: sets.NewString(),
	}
	err := reconciler.initializeNetworkingStrategy()
	if err != nil {
		return nil, err
	}

	labelSelector := map[string]string{common.OwnerKey: meshNamespace}
	err = cl.List(context.TODO(), client.MatchingLabels(labelSelector).InNamespace(meshNamespace), &reconciler.roleBindingsList)
	if err != nil {
		reconciler.Log.Error(err, "error retrieving RoleBinding resources for mesh")
		return nil, pkgerrors.Wrap(err, "error retrieving RoleBinding resources for mesh")
	}
	for _, rb := range reconciler.roleBindingsList.Items {
		reconciler.requiredRoleBindings.Insert(rb.GetName())
	}
	return reconciler, nil
}

func (r *namespaceReconciler) initializeNetworkingStrategy() error {
	// configure networks
	clusterNetwork := &unstructured.Unstructured{}
	clusterNetwork.SetAPIVersion("network.openshift.io/v1")
	clusterNetwork.SetKind("ClusterNetwork")
	r.networkingStrategy = &subnetStrategy{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: "default"}, clusterNetwork)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.Log.Info("default cluster network not defined, skipping network configuration")
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
			r.networkingStrategy, err = newNetworkPolicyStrategy(r.Client, r.Log, r.meshNamespace)
		case "redhat/openshift-ovs-multitenant":
			r.networkingStrategy, err = newMultitenantStrategy(r.Client, r.Log, r.meshNamespace)
		default:
			return fmt.Errorf("unsupported cluster network plugin: %s", networkPlugin)
		}
	} else {
		r.Log.Info("cluster network plugin not defined, skipping network configuration")
	}
	return err
}

func (r *namespaceReconciler) removeNamespaceFromMesh(namespace string) error {
	logger := r.Log.WithValues("namespace", namespace)
	logger.Info("cleaning up resources in namespace removed from mesh")

	// get namespace
	namespaceResource := &core.Namespace{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource)
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsGone(err) {
			logger.Info("namespace to remove from mesh is missing")
			return nil
		}
		logger.Error(err, "error retrieving namespace to remove from mesh")
		return err
	}

	allErrors := []error{}

	// delete role bindings
	rbList := &rbac.RoleBindingList{}
	labelSelector := map[string]string{common.OwnerKey: r.meshNamespace}
	err = r.Client.List(context.TODO(), client.MatchingLabels(labelSelector).InNamespace(namespace), rbList)
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

	// remove NetworkAttachmentDefinition so that Multus CNI no longer invokes Istio CNI for pods in this namespace
	err = r.removeNetworkAttachmentDefinition(namespace, logger)
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
	namespaceResource = &core.Namespace{}
	if err := r.Client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource); err == nil {
		common.DeleteLabel(namespaceResource, common.MemberOfKey)
		if err := r.Client.Update(context.TODO(), namespaceResource); err == nil {
			logger.Info("Removed member-of label from namespace")
		} else if !(apierrors.IsGone(err) || apierrors.IsNotFound(err)) {
			allErrors = append(allErrors, fmt.Errorf("Error removing member-of label from namespace %s: %v", namespace, err))
			return utilerrors.NewAggregate(allErrors)
		}
	} else if !(apierrors.IsGone(err) || apierrors.IsNotFound(err)) {
		allErrors = append(allErrors, fmt.Errorf("Error getting namespace %s prior to removing member-of label: %v", namespace, err))
	}

	return utilerrors.NewAggregate(allErrors)
}

func (r *namespaceReconciler) reconcileNamespaceInMesh(namespace string) error {
	logger := r.Log.WithValues("namespace", namespace)
	logger.Info("configuring namespace for use with mesh")

	// get namespace
	namespaceResource := &core.Namespace{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource)
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
	err = r.reconcileRoleBindings(namespace, r.Log)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	if r.isCNIEnabled {
		// add NetworkAttachmentDefinition to tell Multus to invoke Istio CNI for pods in this namespace
		err = r.addNetworkAttachmentDefinition(namespace, logger)
	} else {
		err = r.removeNetworkAttachmentDefinition(namespace, logger)
	}
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// add mesh labels
	if !common.HasLabel(namespaceResource, common.MemberOfKey) {
		// get fresh Namespace from cache to minimize the chance of a conflict during update (the Namespace might have been updated during the execution of reconcileNamespaceInMesh())
		namespaceResource = &core.Namespace{}
		if err := r.Client.Get(context.TODO(), client.ObjectKey{Name: namespace}, namespaceResource); err == nil {
			common.SetLabel(namespaceResource, common.MemberOfKey, r.meshNamespace)
			if err := r.Client.Update(context.TODO(), namespaceResource); err == nil {
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
	namespaceRoleBindings := rbac.RoleBindingList{}
	labelSelector := map[string]string{common.MemberOfKey: r.meshNamespace}
	err := r.Client.List(context.TODO(), client.InNamespace(namespace).MatchingLabels(labelSelector), &namespaceRoleBindings)
	if err != nil {
		reqLogger.Error(err, "error retrieving RoleBinding resources for namespace")
		return err
	}

	allErrors := []error{}

	// add required role bindings
	existingRoleBindings := nameSet(&namespaceRoleBindings)
	addedRoleBindings := sets.NewString()
	for _, meshRoleBinding := range r.roleBindingsList.Items {
		roleBindingName := meshRoleBinding.GetName()
		if !existingRoleBindings.Has(roleBindingName) {
			reqLogger.Info("creating RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
			roleBinding := meshRoleBinding.DeepCopy()
			roleBinding.ObjectMeta = metav1.ObjectMeta{
				Name:        meshRoleBinding.Name,
				Namespace:   namespace,
				Labels:      roleBinding.Labels,
				Annotations: roleBinding.Annotations,
			}
			common.SetLabel(roleBinding, common.MemberOfKey, r.meshNamespace)
			err = r.Client.Create(context.TODO(), roleBinding)
			if err == nil {
				addedRoleBindings.Insert(roleBindingName)
			} else {
				reqLogger.Error(err, "error creating RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
				allErrors = append(allErrors, err)
			}
		} // XXX: else if existingRoleBinding.annotations[mesh-generation] != meshRoleBinding.annotations[generation] then update?
	}

	existingRoleBindings = existingRoleBindings.Union(addedRoleBindings)

	// delete obsolete role bindings
	for roleBindingName := range existingRoleBindings.Difference(r.requiredRoleBindings) {
		reqLogger.Info("deleting RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
		roleBinding := &rbac.RoleBinding{}
		roleBinding.SetName(roleBindingName)
		roleBinding.SetNamespace(namespace)
		err = r.Client.Delete(context.TODO(), roleBinding, client.PropagationPolicy(metav1.DeletePropagationForeground))
		if err != nil && !(apierrors.IsNotFound(err) || apierrors.IsGone(err)) {
			reqLogger.Error(err, "error deleting RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
			allErrors = append(allErrors, err)
		}
	}

	// if there were errors, we've logged them and there's not really anything we can do, as we're in an uncertain state
	// maybe a following reconcile will add the required role binding that failed.  if it was a delete that failed, we're
	// just leaving behind some cruft.
	return utilerrors.NewAggregate(allErrors)
}

func (r *namespaceReconciler) addNetworkAttachmentDefinition(namespace string, reqLogger logr.Logger) error {
	netAttachDefName, ok := common.GetCNINetworkName(r.meshVersion)
	if !ok {
		return fmt.Errorf("unknown maistra version: %s", r.meshVersion)
	}

	nadList, err := common.FetchMeshResources(r.Client, schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinitionList",
	}, r.meshNamespace, namespace)
	if err != nil {
		return fmt.Errorf("Could not list NetworkAttachmentDefinition resources in member namespace %s: %v", namespace, err)
	}

	found := false
	var allErrors []error
	for _, nad := range nadList.Items {
		if nad.GetName() == netAttachDefName {
			found = true
		} else if err := r.Client.Delete(context.TODO(), &nad, client.PropagationPolicy(metav1.DeletePropagationOrphan)); err != nil {
			allErrors = append(allErrors, err)
		}
	}
	if found {
		// TODO: update resource if its state isn't what we want
		return utilerrors.NewAggregate(allErrors)
	}

	netAttachDef := &unstructured.Unstructured{}
	netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})
	netAttachDef.SetNamespace(namespace)
	netAttachDef.SetName(netAttachDefName)
	common.SetLabel(netAttachDef, common.MemberOfKey, r.meshNamespace)
	err = r.Client.Create(context.TODO(), netAttachDef)
	if err != nil {
		allErrors = append(allErrors, fmt.Errorf("Could not create NetworkAttachmentDefinition %s/%s: %v", namespace, netAttachDefName, err))
	}
	return utilerrors.NewAggregate(allErrors)
}

func (r *namespaceReconciler) removeNetworkAttachmentDefinition(namespace string, reqLogger logr.Logger) error {
	nadList, err := common.FetchMeshResources(r.Client, schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinitionList",
	}, r.meshNamespace, namespace)
	if err != nil {
		return fmt.Errorf("Could not list NetworkAttachmentDefinition resources in member namespace %s: %v", namespace, err)
	}

	var allErrors []error
	for _, nad := range nadList.Items {
		if err := r.Client.Delete(context.TODO(), &nad, client.PropagationPolicy(metav1.DeletePropagationOrphan)); err != nil {
			allErrors = append(allErrors, err)
		}
	}
	return utilerrors.NewAggregate(allErrors)
}
