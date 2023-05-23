package member

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	multusv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	configv1 "github.com/openshift/api/config/v1"
	networkv1 "github.com/openshift/api/network/v1"
	pkgerrors "github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

const (
	networkTypeOpenShiftSDN  = "OpenShiftSDN"
	networkTypeCalico        = "Calico"
	networkTypeOVNKubernetes = "OVNKubernetes"
)

type namespaceReconciler struct {
	common.ControllerResources
	meshNamespace        string
	meshVersion          versions.Version
	clusterWideMode      bool
	isCNIEnabled         bool
	networkingStrategy   NamespaceReconciler
	roleBindingsList     rbac.RoleBindingList
	requiredRoleBindings sets.String
}

func newNamespaceReconciler(ctx context.Context, cl client.Client,
	meshNamespace string, meshVersion versions.Version, clusterWideMode, isCNIEnabled bool,
) (NamespaceReconciler, error) {
	reconciler := &namespaceReconciler{
		ControllerResources: common.ControllerResources{
			Client: cl,
		},
		meshNamespace:        meshNamespace,
		meshVersion:          meshVersion,
		clusterWideMode:      clusterWideMode,
		isCNIEnabled:         isCNIEnabled,
		roleBindingsList:     rbac.RoleBindingList{},
		requiredRoleBindings: sets.NewString(),
	}
	logger := reconciler.getLogger(ctx)

	err := reconciler.initializeNetworkingStrategy(ctx)
	if err != nil {
		return nil, err
	}

	labelSelector := map[string]string{common.OwnerKey: meshNamespace}
	err = cl.List(ctx, &reconciler.roleBindingsList, client.InNamespace(meshNamespace), client.MatchingLabels(labelSelector))
	if err != nil {
		logger.Error(err, "error retrieving RoleBinding resources for mesh")
		return nil, pkgerrors.Wrap(err, "error retrieving RoleBinding resources for mesh")
	}
	for _, rb := range reconciler.roleBindingsList.Items {
		reconciler.requiredRoleBindings.Insert(rb.GetName())
	}
	return reconciler, nil
}

func (r *namespaceReconciler) initializeNetworkingStrategy(ctx context.Context) error {
	log := common.LogFromContext(ctx)
	// configure networks
	r.networkingStrategy = &subnetStrategy{}
	network := &configv1.Network{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: "cluster"}, network)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("network configuration not defined, skipping")
			return nil
		} else if meta.IsNoMatchError(err) {
			log.Info("kind Network in config.openshift.io/v1 doesn't exist; operator seems to be running in Kubernetes, not OpenShift; defaulting to NetworkPolicy")
			r.networkingStrategy, err = newNetworkPolicyStrategy(ctx, r.Client, r.meshNamespace)
		}
		return err
	}

	switch strings.ToLower(network.Spec.NetworkType) {
	case strings.ToLower(networkTypeOpenShiftSDN):
		clusterNetwork := &networkv1.ClusterNetwork{}
		err = r.Client.Get(ctx, client.ObjectKey{Name: "default"}, clusterNetwork)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("default cluster network not defined, skipping network configuration")
				return nil
			}
			return pkgerrors.Wrap(err, "error retrieving ClusterNetwork resource")
		}
		switch clusterNetwork.PluginName {
		case "redhat/openshift-ovs-subnet":
			log.Info("Network Strategy OpenShiftSDN:subnet")
			// nothing to do
		case "redhat/openshift-ovs-networkpolicy":
			log.Info("Network Strategy OpenShiftSDN:NetworkPolicy")
			r.networkingStrategy, err = newNetworkPolicyStrategy(ctx, r.Client, r.meshNamespace)
		case "redhat/openshift-ovs-multitenant":
			log.Info("Network Strategy OpenShiftSDN:MultiTenant")
			r.networkingStrategy = newMultitenantStrategy(r.Client, r.meshNamespace)
		case "":
			log.Info(fmt.Sprintf("cluster network plugin not defined, defaulting to NetworkPolicy for Network Type: %s", network.Spec.NetworkType))
			r.networkingStrategy, err = newNetworkPolicyStrategy(ctx, r.Client, r.meshNamespace)
		default:
			return fmt.Errorf("unsupported cluster network plugin: %s", clusterNetwork.PluginName)
		}
	case strings.ToLower(networkTypeCalico):
		log.Info("Network Strategy Calico:NetworkPolicy")
		r.networkingStrategy, err = newNetworkPolicyStrategy(ctx, r.Client, r.meshNamespace)
	case strings.ToLower(networkTypeOVNKubernetes):
		log.Info("Network Strategy OVNKubernetes:NetworkPolicy")
		r.networkingStrategy, err = newNetworkPolicyStrategy(ctx, r.Client, r.meshNamespace)
	default:
		log.Info(fmt.Sprintf("Unknown Network Type %s, defaulting to NetworkPolicy", network.Spec.NetworkType))
		r.networkingStrategy, err = newNetworkPolicyStrategy(ctx, r.Client, r.meshNamespace)
	}
	return pkgerrors.Wrap(err, "error initializing network strategy")
}

func (r *namespaceReconciler) removeNamespaceFromMesh(ctx context.Context, namespace string) error {
	logger := common.LogFromContext(ctx).WithValues("namespace", namespace)
	ctx = common.NewContextWithLog(ctx, logger)

	// get namespace
	namespaceResource := &core.Namespace{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: namespace}, namespaceResource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("namespace to remove from mesh is missing")
			return nil
		}
		logger.Error(err, "error retrieving namespace to remove from mesh")
		return err
	}

	if namespaceResource.DeletionTimestamp != nil {
		logger.Info("not removing mesh resources from namespace because it is being deleted")
		return &NamespaceTerminatingError{}
	}

	logger.Info("cleaning up resources in namespace removed from mesh")

	allErrors := []error{}

	// delete role bindings
	rbList := &rbac.RoleBindingList{}
	labelSelector := map[string]string{common.MemberOfKey: r.meshNamespace}
	err = r.Client.List(ctx, rbList, client.InNamespace(namespace), client.MatchingLabels(labelSelector))
	if err == nil {
		for _, rb := range rbList.Items {
			logger.Info("deleting RoleBinding for mesh ServiceAccount", "RoleBinding", rb.GetName())
			err = r.Client.Delete(ctx, &rb)
			if err != nil && !apierrors.IsNotFound(err) {
				logger.Error(err, "error removing RoleBinding associated with mesh", "RoleBinding", rb.GetName())
				allErrors = append(allErrors, err)
			}
		}
	} else {
		logger.Error(err, "error could not retrieve RoleBindings associated with mesh")
		allErrors = append(allErrors, err)
	}

	// remove NetworkAttachmentDefinition so that Multus CNI no longer invokes Istio CNI for pods in this namespace
	err = r.removeNetworkAttachmentDefinition(ctx, namespace)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// delete network policies
	err = r.networkingStrategy.removeNamespaceFromMesh(ctx, namespace)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// remove mesh labels
	// get fresh Namespace from cache to minimize the chance of a conflict during update
	// (the Namespace might have been updated during the execution of removeNamespaceFromMesh())
	namespaceResource = &core.Namespace{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: namespace}, namespaceResource); err == nil {
		common.DeleteLabel(namespaceResource, common.MemberOfKey)
		if err := r.Client.Update(ctx, namespaceResource); err == nil {
			logger.Info("Removed member-of label from namespace")
		} else if !apierrors.IsNotFound(err) {
			allErrors = append(allErrors, pkgerrors.Wrapf(err, "error removing member-of label from namespace %s", namespace))
			return utilerrors.NewAggregate(allErrors)
		}
	} else if !apierrors.IsNotFound(err) {
		allErrors = append(allErrors, err)
	}

	return utilerrors.NewAggregate(allErrors)
}

func (r *namespaceReconciler) reconcileNamespaceInMesh(ctx context.Context, namespace string) error {
	logger := common.LogFromContext(ctx).WithValues("namespace", namespace)
	logger.V(2).Info("configuring namespace for use with mesh")
	ctx = common.NewContextWithLog(ctx, logger)

	// get namespace
	namespaceResource := &core.Namespace{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: namespace}, namespaceResource)
	if err != nil {
		return err
	}

	if namespaceResource.DeletionTimestamp != nil {
		logger.Info("not reconciling member namespace, because it is being deleted")
		return &NamespaceTerminatingError{}
	}

	memberOf := ""
	if namespaceResource.Labels != nil {
		memberOf = namespaceResource.Labels[common.MemberOfKey]
	}
	isMemberOfDifferentMesh := memberOf != "" && memberOf != r.meshNamespace
	if isMemberOfDifferentMesh {
		return fmt.Errorf("cannot reconcile namespace %s in mesh %s, as it is already a member of %s", namespace, r.meshNamespace, memberOf)
	}

	// configure networking
	err = r.networkingStrategy.reconcileNamespaceInMesh(ctx, namespace)
	if err != nil {
		return err
	}

	allErrors := []error{}

	if err := r.reconcileRoleBindings(ctx, namespace); err != nil {
		allErrors = append(allErrors, err)
	}

	if r.isCNIEnabled {
		// add NetworkAttachmentDefinition to tell Multus to invoke Istio CNI for pods in this namespace
		netAttachDefName := r.meshVersion.GetCNINetworkName()
		err = r.addNetworkAttachmentDefinition(ctx, namespace, netAttachDefName)
	} else {
		err = r.removeNetworkAttachmentDefinition(ctx, namespace)
	}
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// add mesh labels
	if !common.HasLabel(namespaceResource, common.MemberOfKey) {
		// get fresh Namespace from cache to minimize the chance of a conflict
		// during update (the Namespace might have been updated during
		// the execution of reconcileNamespaceInMesh())
		namespaceResource = &core.Namespace{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: namespace}, namespaceResource); err == nil {
			common.SetLabel(namespaceResource, common.MemberOfKey, r.meshNamespace)
			if err := r.Client.Update(ctx, namespaceResource); err == nil {
				logger.Info("Added member-of label to namespace")
			} else {
				allErrors = append(allErrors, pkgerrors.Wrapf(err, "error adding member-of label to namespace %s", namespace))
			}
		} else {
			allErrors = append(allErrors, err)
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

func (r *namespaceReconciler) reconcileRoleBindings(ctx context.Context, namespace string) error {
	reqLogger := common.LogFromContext(ctx)

	namespaceRoleBindings := rbac.RoleBindingList{}
	labelSelector := map[string]string{common.MemberOfKey: r.meshNamespace}
	err := r.Client.List(ctx, &namespaceRoleBindings, client.MatchingLabels(labelSelector), client.InNamespace(namespace))
	if err != nil {
		reqLogger.Error(err, "error retrieving RoleBinding resources for namespace")
		return err
	}

	allErrors := []error{}

	// add required role bindings
	existingRoleBindings := common.NameSet(&namespaceRoleBindings)
	addedRoleBindings := sets.NewString()
	for _, meshRoleBinding := range r.roleBindingsList.Items {
		// since we don't copy Roles from istio-system to member namespace, there's no sense in copying RoleBindings
		// that reference them; instead, we only need to copy RoleBindings that reference ClusterRoles
		if meshRoleBinding.RoleRef.Kind == "Role" {
			continue
		}
		// if cluster-wide mode is enabled, istiod is given cluster-wide permissions via ClusterRoleBindings,
		// so there's no need to create RoleBindings in member namespaces, but we still have to create RoleBinding
		// for Prometheus, otherwise it won't be able to scrape metrics from workloads in member namespaces.
		if r.clusterWideMode && strings.HasPrefix(meshRoleBinding.Name, "istiod-") {
			continue
		}
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
			removeLabelsUsedByPruner(roleBinding)

			err = r.Client.Create(ctx, roleBinding)
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
		err = r.Client.Delete(ctx, roleBinding, client.PropagationPolicy(metav1.DeletePropagationForeground))
		if err != nil && !apierrors.IsNotFound(err) {
			reqLogger.Error(err, "error deleting RoleBinding for mesh ServiceAccount", "RoleBinding", roleBindingName)
			allErrors = append(allErrors, err)
		}
	}

	// if there were errors, we've logged them and there's not really anything we can do, as we're in an uncertain state
	// maybe a following reconcile will add the required role binding that failed.  if it was a delete that failed, we're
	// just leaving behind some cruft.
	return utilerrors.NewAggregate(allErrors)
}

// Removes the labels that the pruner uses to determine which objects to prune.
// These labels need to be removed when copying objects from the mesh namespace
// to the member namespace if those objects are managed by the member controller
// and should thus never be deleted by the pruner.
func removeLabelsUsedByPruner(obj metav1.Object) {
	common.DeleteLabel(obj, common.OwnerKey)
	common.DeleteLabel(obj, common.OwnerNameKey)
	common.DeleteLabel(obj, common.KubernetesAppVersionKey)
}

func (r *namespaceReconciler) addNetworkAttachmentDefinition(ctx context.Context, namespace string, netAttachDefName string) error {
	log := common.LogFromContext(ctx)
	nadList := &multusv1.NetworkAttachmentDefinitionList{}
	if err := r.Client.List(ctx, nadList, client.InNamespace(namespace), client.MatchingLabels{common.MemberOfKey: r.meshNamespace}); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("skipping creation of NetworkAttachmentDefinition, because this cluster doesn't support them")
			return nil
		}
		return pkgerrors.Wrapf(err, "could not list NetworkAttachmentDefinition resources in member namespace %s", namespace)
	}

	found := false
	var allErrors []error
	for _, nad := range nadList.Items {
		if nad.GetName() == netAttachDefName {
			found = true
		} else if err := r.Client.Delete(ctx, &nad, client.PropagationPolicy(metav1.DeletePropagationOrphan)); err != nil && !apierrors.IsNotFound(err) {
			allErrors = append(allErrors, err)
		}
	}
	if found {
		// TODO: update resource if its state isn't what we want
		return utilerrors.NewAggregate(allErrors)
	}

	log.Info("creating NetworkAttachmentDefinition", "NetworkAttachmentDefinition", netAttachDefName)
	netAttachDef := &multusv1.NetworkAttachmentDefinition{}
	netAttachDef.SetNamespace(namespace)
	netAttachDef.SetName(netAttachDefName)
	common.SetLabel(netAttachDef, common.MemberOfKey, r.meshNamespace)
	if err := r.Client.Create(ctx, netAttachDef); err != nil {
		allErrors = append(allErrors, pkgerrors.Wrapf(err, "could not create NetworkAttachmentDefinition %s/%s", namespace, netAttachDefName))
	}
	return utilerrors.NewAggregate(allErrors)
}

func (r *namespaceReconciler) removeNetworkAttachmentDefinition(ctx context.Context, namespace string) error {
	log := common.LogFromContext(ctx)
	nadList := &multusv1.NetworkAttachmentDefinitionList{}
	if err := r.Client.List(ctx, nadList, client.InNamespace(namespace), client.MatchingLabels{common.MemberOfKey: r.meshNamespace}); err != nil {
		if meta.IsNoMatchError(err) {
			// if the NetworkAttachmentDefinition kind doesn't exist in this cluster, we don't need to remove it
			return nil
		}
		return pkgerrors.Wrapf(err, "could not list NetworkAttachmentDefinition resources in member namespace %s", namespace)
	}

	var allErrors []error
	for _, nad := range nadList.Items {
		log.Info("deleting NetworkAttachmentDefinition", "NetworkAttachmentDefinition", nad.GetName())
		if err := r.Client.Delete(ctx, &nad, client.PropagationPolicy(metav1.DeletePropagationOrphan)); err != nil && !apierrors.IsNotFound(err) {
			allErrors = append(allErrors, err)
		}
	}
	return utilerrors.NewAggregate(allErrors)
}

func (r *namespaceReconciler) getLogger(ctx context.Context) logr.Logger {
	return common.LogFromContext(ctx).WithValues("MeshNamespace", r.meshNamespace)
}

type NamespaceTerminatingError struct{}

func (e *NamespaceTerminatingError) Error() string {
	return "namespace is terminating"
}

func IsTerminating(err error) bool {
	_, ok := err.(*NamespaceTerminatingError)
	return ok
}
