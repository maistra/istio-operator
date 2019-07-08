package memberroll

import (
	"context"

	"github.com/go-logr/logr"

    "github.com/maistra/istio-operator/pkg/controller/common"

    networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type networkPolicyStrategy struct {
	client                  client.Client
	logger                  logr.Logger
	meshNamespace           string
	requiredNetworkPolicies map[string]struct{}
	networkPoliciesList     *unstructured.UnstructuredList
}

var _ networkingStrategy = (*networkPolicyStrategy)(nil)

func newNetworkPolicyStrategy(r *namespaceReconciler) (*networkPolicyStrategy, error) {
	var err error
	strategy := &networkPolicyStrategy{
		client:                  r.client,
		logger:                  r.logger.WithValues("NetworkStrategy", "NetworkPolicy"),
		meshNamespace:           r.meshNamespace,
		requiredNetworkPolicies: map[string]struct{}{},
	}
	strategy.networkPoliciesList, err = common.FetchOwnedResources(r.client, networkingv1.SchemeGroupVersion.WithKind("NetworkPolicy"), strategy.meshNamespace, strategy.meshNamespace)
	if err != nil {
		strategy.logger.Error(err, "error retrieving NetworkPolicy resources for mesh")
		return nil, err
	}
	for _, np := range strategy.networkPoliciesList.Items {
		if _, ok := common.GetAnnotation(&np, common.InternalKey); ok {
			continue
		}
		strategy.requiredNetworkPolicies[np.GetName()] = struct{}{}
	}
	return strategy, nil
}

func (s *networkPolicyStrategy) reconcileNamespaceInMesh(namespace string) error {
	logger := s.logger.WithValues("Namespace", namespace)
	namespaceNetworkPolicies, err := common.FetchMeshResources(s.client, networkingv1.SchemeGroupVersion.WithKind("NetworkPolicy"), s.meshNamespace, namespace)
	if err != nil {
		logger.Error(err, "error retrieving NetworkPolicy resources for namespace")
		return err
	}

	allErrors := []error{}

	// add required network policies
	existingNetworkPolicies := nameSet(namespaceNetworkPolicies.Items)
	addedNetworkPolicies := map[string]struct{}{}
	for _, meshNetworkPolicy := range s.networkPoliciesList.Items {
		networkPolicyName := meshNetworkPolicy.GetName()
		if _, ok := s.requiredNetworkPolicies[networkPolicyName]; !ok {
			// this is not required for members
			continue
		}
		if _, ok := existingNetworkPolicies[networkPolicyName]; !ok {
			logger.Info("creating NetworkPolicy", "NetworkPolicy", networkPolicyName)
			networkPolicy := &unstructured.Unstructured{}
			networkPolicy.SetGroupVersionKind(networkingv1.SchemeGroupVersion.WithKind("NetworkPolicy"))
			networkPolicy.SetNamespace(namespace)
			networkPolicy.SetName(networkPolicyName)
			networkPolicy.SetLabels(meshNetworkPolicy.GetLabels())
			networkPolicy.SetAnnotations(meshNetworkPolicy.GetAnnotations())
			if ingress, ok, _ := unstructured.NestedSlice(meshNetworkPolicy.UnstructuredContent(), "spec", "ingress"); ok {
				unstructured.SetNestedSlice(networkPolicy.UnstructuredContent(), ingress, "spec", "ingress")
			}
			if egress, ok, _ := unstructured.NestedSlice(meshNetworkPolicy.UnstructuredContent(), "spec", "egress"); ok {
				unstructured.SetNestedSlice(networkPolicy.UnstructuredContent(), egress, "spec", "egress")
			}
			common.SetLabel(networkPolicy, common.MemberOfKey, s.meshNamespace)
			err = s.client.Create(context.TODO(), networkPolicy)
			if err == nil {
				addedNetworkPolicies[networkPolicyName] = struct{}{}
			} else {
				logger.Error(err, "error creating NetworkPolicy", "NetworkPolicy", networkPolicyName)
				allErrors = append(allErrors, err)
			}
		} // XXX: else if existingNetworkPolicy.annotations[mesh-generation] != meshNetworkPolicy.annotations[generation] then update?
	}

	existingNetworkPolicies = union(existingNetworkPolicies, addedNetworkPolicies)

	// delete obsolete network policies
	for networkPolicyName := range difference(existingNetworkPolicies, s.requiredNetworkPolicies) {
		logger.Info("deleting NetworkPolicy", "NetworkPolicy", networkPolicyName)
		networkPolicy := &unstructured.Unstructured{}
		networkPolicy.SetGroupVersionKind(networkingv1.SchemeGroupVersion.WithKind("NetworkPolicy"))
		networkPolicy.SetName(networkPolicyName)
		networkPolicy.SetNamespace(namespace)
		err = s.client.Delete(context.TODO(), networkPolicy, client.PropagationPolicy(metav1.DeletePropagationForeground))
		if err != nil && !(errors.IsNotFound(err) || errors.IsGone(err)) {
			logger.Error(err, "error deleting NetworkPolicy", "NetworkPolicy", networkPolicyName)
			allErrors = append(allErrors, err)
		}
	}

	// if there were errors, we've logged them and there's not really anything we can do, as we're in an uncertain state
	// maybe a following reconcile will add the required network policy that failed.  if it was a delete that failed, we're
	// just leaving behind some cruft.
	return utilerrors.NewAggregate(allErrors)
}

func (s *networkPolicyStrategy) removeNamespaceFromMesh(namespace string) error {
	allErrors := []error{}
	logger := s.logger.WithValues("Namespace", namespace)
	npList, err := common.FetchMeshResources(s.client, networkingv1.SchemeGroupVersion.WithKind("NetworkPolicy"), s.meshNamespace, namespace)
	if err == nil {
		for _, np := range npList.Items {
			logger.Info("deleting NetworkPolicy for mesh", "NetworkPolicy", np.GetName())
			err = s.client.Delete(context.TODO(), &np)
			if err != nil {
				logger.Error(err, "error removing NetworkPolicy associated with mesh", "NetworkPolicy", np.GetName())
				allErrors = append(allErrors, err)
			}
		}
	} else {
		logger.Error(err, "error could not retrieve NetworkPolicy resources associated with mesh")
		allErrors = append(allErrors, err)
	}
	return utilerrors.NewAggregate(allErrors)
}
