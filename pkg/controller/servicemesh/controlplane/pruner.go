package controlplane

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/maistra/istio-operator/pkg/controller/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type pruneConfig struct {
	supportsDeleteCollection bool
}

var (
	// XXX: move this into a ConfigMap so users can override things if they add new types in customized charts
	// ordered by which types should be deleted, first to last
	namespacedResources = map[schema.GroupVersionKind]pruneConfig{
		gvk("kiali.io", "v1alpha1", "Kiali"):                        {supportsDeleteCollection: true},
		gvk("autoscaling", "v2beta1", "HorizontalPodAutoscaler"):    {supportsDeleteCollection: true},
		gvk("policy", "v1beta1", "PodDisruptionBudget"):             {supportsDeleteCollection: true},
		gvk("route.openshift.io", "v1", "Route"):                    {supportsDeleteCollection: true},
		gvk("apps", "v1", "Deployment"):                             {supportsDeleteCollection: true},
		gvk("apps", "v1", "DaemonSet"):                              {supportsDeleteCollection: true},
		gvk("apps", "v1", "StatefulSet"):                            {supportsDeleteCollection: true},
		gvk("extensions", "v1beta1", "Ingress"):                     {supportsDeleteCollection: true},
		gvk("", "v1", "Service"):                                    {supportsDeleteCollection: false},
		gvk("", "v1", "Endpoints"):                                  {supportsDeleteCollection: true},
		gvk("", "v1", "ConfigMap"):                                  {supportsDeleteCollection: true},
		gvk("", "v1", "PersistentVolumeClaim"):                      {supportsDeleteCollection: true},
		gvk("", "v1", "Pod"):                                        {supportsDeleteCollection: true},
		gvk("", "v1", "Secret"):                                     {supportsDeleteCollection: true},
		gvk("", "v1", "ServiceAccount"):                             {supportsDeleteCollection: true},
		gvk("networking.k8s.io", "v1", "NetworkPolicy"):             {supportsDeleteCollection: true},
		gvk("rbac.authorization.k8s.io", "v1beta1", "RoleBinding"):  {supportsDeleteCollection: true},
		gvk("rbac.authorization.k8s.io", "v1", "RoleBinding"):       {supportsDeleteCollection: true},
		gvk("rbac.authorization.k8s.io", "v1beta1", "Role"):         {supportsDeleteCollection: true},
		gvk("rbac.authorization.k8s.io", "v1", "Role"):              {supportsDeleteCollection: true},
		gvk("authentication.istio.io", "v1alpha1", "Policy"):        {supportsDeleteCollection: true},
		gvk("config.istio.io", "v1alpha2", "adapter"):               {supportsDeleteCollection: true},
		gvk("config.istio.io", "v1alpha2", "attributemanifest"):     {supportsDeleteCollection: true},
		gvk("config.istio.io", "v1alpha2", "handler"):               {supportsDeleteCollection: true},
		gvk("config.istio.io", "v1alpha2", "kubernetes"):            {supportsDeleteCollection: true},
		gvk("config.istio.io", "v1alpha2", "logentry"):              {supportsDeleteCollection: true},
		gvk("config.istio.io", "v1alpha2", "metric"):                {supportsDeleteCollection: true},
		gvk("config.istio.io", "v1alpha2", "rule"):                  {supportsDeleteCollection: true},
		gvk("config.istio.io", "v1alpha2", "template"):              {supportsDeleteCollection: true},
		gvk("networking.istio.io", "v1alpha3", "DestinationRule"):   {supportsDeleteCollection: true},
		gvk("networking.istio.io", "v1alpha3", "EnvoyFilter"):       {supportsDeleteCollection: true},
		gvk("networking.istio.io", "v1alpha3", "Gateway"):           {supportsDeleteCollection: true},
		gvk("networking.istio.io", "v1alpha3", "VirtualService"):    {supportsDeleteCollection: true},
		gvk("jaegertracing.io", "v1", "jaeger"):                     {supportsDeleteCollection: true},
		gvk("authentication.maistra.io", "v1", "ServiceMeshPolicy"): {supportsDeleteCollection: true},
	}

	// ordered by which types should be deleted, first to last
	nonNamespacedResources = map[schema.GroupVersionKind]pruneConfig{
		gvk("admissionregistration.k8s.io", "v1beta1", "MutatingWebhookConfiguration"):   {supportsDeleteCollection: true},
		gvk("admissionregistration.k8s.io", "v1beta1", "ValidatingWebhookConfiguration"): {supportsDeleteCollection: true},
		gvk("certmanager.k8s.io", "v1alpha1", "ClusterIssuer"):                           {supportsDeleteCollection: true},
		gvk("rbac.authorization.k8s.io", "v1", "ClusterRole"):                            {supportsDeleteCollection: true},
		gvk("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"):                     {supportsDeleteCollection: true},
	}
)

func (r *controlPlaneInstanceReconciler) prune(ctx context.Context, generation string) error {
	allErrors := []error{}
	err := r.pruneResources(ctx, namespacedResources, generation, r.Instance.Namespace)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	err = r.pruneResources(ctx, namespacedResources, generation, r.OperatorNamespace)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	err = r.pruneResources(ctx, nonNamespacedResources, generation, "")
	if err != nil {
		allErrors = append(allErrors, err)
	}
	return utilerrors.NewAggregate(allErrors)
}

func (r *controlPlaneInstanceReconciler) pruneResources(ctx context.Context, gvks map[schema.GroupVersionKind]pruneConfig, instanceGeneration string, namespace string) error {
	log := common.LogFromContext(ctx)

	allErrors := []error{}
	for gvk, pruneConfig := range gvks {
		log.Info("pruning resources", "type", gvk.String())
		var err error
		if pruneConfig.supportsDeleteCollection {
			err = r.pruneAll(ctx, gvk, instanceGeneration, namespace)
		} else {
			err = r.pruneIndividually(ctx, gvk, instanceGeneration, namespace)
		}
		if err != nil {
			log.Error(err, "Error pruning resources", "type", gvk.String())
			allErrors = append(allErrors, err)
		}
	}
	return utilerrors.NewAggregate(allErrors)
}

func (r *controlPlaneInstanceReconciler) pruneIndividually(ctx context.Context, gvk schema.GroupVersionKind, instanceGeneration string, namespace string) error {
	labelSelector := map[string]string{common.OwnerKey: r.Instance.Namespace}
	objects := &unstructured.UnstructuredList{}
	objects.SetGroupVersionKind(gvk)
	err := r.Client.List(ctx, objects, client.InNamespace(namespace), client.MatchingLabels(labelSelector))
	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("Error retrieving resources to prune: %v", err)
	}
	for _, object := range objects.Items {
		if generation, ok := common.GetAnnotation(&object, common.MeshGenerationKey); ok && generation != instanceGeneration {
			err = r.Client.Delete(ctx, &object, client.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("Error deleting resource: %v", err)
			}
		}
	}
	return nil
}

func (r *controlPlaneInstanceReconciler) pruneAll(ctx context.Context, gvk schema.GroupVersionKind, instanceGeneration string, namespace string) error {
	ownerRequirement, err := labels.NewRequirement(common.OwnerKey, selection.Equals, []string{r.Instance.Namespace})
	if err != nil {
		return err
	}
	generationRequirement, err := labels.NewRequirement(common.KubernetesAppVersionKey, selection.NotEquals, []string{instanceGeneration})
	if err != nil {
		return err
	}
	selector := labels.NewSelector().Add(*ownerRequirement, *generationRequirement)

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)
	err = r.Client.DeleteAllOf(ctx,
		object,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
		client.PropagationPolicy(metav1.DeletePropagationBackground))

	if meta.IsNoMatchError(err) || errors.IsNotFound(err) {
		return nil
	}
	return err
}

func gvk(group, version, kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	}
}
