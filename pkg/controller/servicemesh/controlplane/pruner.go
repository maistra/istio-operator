package controlplane

import (
	"context"
	"fmt"

	"github.com/maistra/istio-operator/pkg/controller/common"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type pruneConfig struct {
	gvk                      schema.GroupVersionKind
	supportsDeleteCollection bool
}

var (
	builtinTypes = []schema.GroupVersionKind{
		gvk("autoscaling", "v2beta1", "HorizontalPodAutoscaler"),
		gvk("policy", "v1beta1", "PodDisruptionBudget"),
		gvk("route.openshift.io", "v1", "Route"),
		gvk("apps", "v1", "Deployment"),
		gvk("apps", "v1", "DaemonSet"),
		gvk("apps", "v1", "StatefulSet"),
		gvk("networking.k8s.io", "v1", "Ingress"),
		gvk("", "v1", "Service"),
		gvk("", "v1", "Endpoints"),
		gvk("", "v1", "ConfigMap"),
		gvk("", "v1", "PersistentVolumeClaim"),
		gvk("", "v1", "Pod"),
		gvk("", "v1", "Secret"),
		gvk("", "v1", "ServiceAccount"),
		gvk("networking.k8s.io", "v1", "NetworkPolicy"),
		gvk("rbac.authorization.k8s.io", "v1", "RoleBinding"),
		gvk("rbac.authorization.k8s.io", "v1", "Role"),
		gvk("admissionregistration.k8s.io", "v1", "MutatingWebhookConfiguration"),
		gvk("admissionregistration.k8s.io", "v1", "ValidatingWebhookConfiguration"),
		gvk("rbac.authorization.k8s.io", "v1", "ClusterRole"),
		gvk("rbac.authorization.k8s.io", "v1", "ClusterRoleBinding"),
	}

	crds = map[schema.GroupKind]struct{}{
		gk("kiali.io", "Kiali"):                              {},
		gk("jaegertracing.io", "Jaeger"):                     {},
		gk("config.istio.io", "adapter"):                     {},
		gk("config.istio.io", "attributemanifest"):           {},
		gk("config.istio.io", "handler"):                     {},
		gk("config.istio.io", "instance"):                    {},
		gk("config.istio.io", "kubernetes"):                  {},
		gk("config.istio.io", "logentry"):                    {},
		gk("config.istio.io", "metric"):                      {},
		gk("config.istio.io", "rule"):                        {},
		gk("config.istio.io", "template"):                    {},
		gk("networking.istio.io", "DestinationRule"):         {},
		gk("networking.istio.io", "EnvoyFilter"):             {},
		gk("networking.istio.io", "Gateway"):                 {},
		gk("networking.istio.io", "ServiceEntry"):            {},
		gk("networking.istio.io", "Sidecar"):                 {},
		gk("networking.istio.io", "VirtualService"):          {},
		gk("networking.istio.io", "WorkloadEntry"):           {},
		gk("authentication.istio.io", "Policy"):              {},
		gk("authentication.maistra.io", "ServiceMeshPolicy"): {},
		gk("security.istio.io", "AuthorizationPolicy"):       {},
		gk("security.istio.io", "PeerAuthentication"):        {},
		gk("security.istio.io", "RequestAuthentication"):     {},
		gk("certmanager.k8s.io", "ClusterIssuer"):            {},
	}
)

func (r *controlPlaneInstanceReconciler) prune(ctx context.Context, generation string) error {
	resourcesToPrune, err := r.findResourcesToPrune(ctx)
	if err != nil {
		return err
	}
	return r.pruneResources(ctx, resourcesToPrune, generation)
}

func (r *controlPlaneInstanceReconciler) findResourcesToPrune(ctx context.Context) ([]pruneConfig, error) {
	resourcesToPrune := []pruneConfig{}
	for _, gvk := range builtinTypes {
		resourcesToPrune = append(resourcesToPrune, pruneConfig{
			gvk:                      gvk,
			supportsDeleteCollection: false,
		})
	}

	crdList := &v1.CustomResourceDefinitionList{}
	err := r.Client.List(ctx, crdList)
	if err != nil {
		return nil, err
	}
	for _, crd := range crdList.Items {
		if _, exists := crds[gk(crd.Spec.Group, crd.Spec.Names.Kind)]; exists {
			version := getVersion(crd)
			if version == "" {
				continue
			}
			resourcesToPrune = append(resourcesToPrune, pruneConfig{
				gvk: schema.GroupVersionKind{
					Group:   crd.Spec.Group,
					Version: version,
					Kind:    crd.Spec.Names.Kind,
				},
				supportsDeleteCollection: true,
			})
		}
	}
	return resourcesToPrune, nil
}

func getVersion(crd v1.CustomResourceDefinition) string {
	for _, version := range crd.Spec.Versions {
		if version.Served {
			return version.Name
		}
	}
	return ""
}

func (r *controlPlaneInstanceReconciler) pruneResources(ctx context.Context, pruneConfigs []pruneConfig, instanceGeneration string) error {
	log := common.LogFromContext(ctx)

	allErrors := []error{}
	for _, pruneConfig := range pruneConfigs {
		gvk := pruneConfig.gvk
		log.Info("pruning resources", "type", gvk.String())
		var err error
		if pruneConfig.supportsDeleteCollection {
			err = r.pruneAll(ctx, gvk, instanceGeneration)
		} else {
			err = r.pruneIndividually(ctx, gvk, instanceGeneration)
		}
		if err != nil {
			log.Error(err, "Error pruning resources", "type", gvk.String())
			allErrors = append(allErrors, err)
		}
	}
	return utilerrors.NewAggregate(allErrors)
}

func (r *controlPlaneInstanceReconciler) pruneIndividually(ctx context.Context, gvk schema.GroupVersionKind, instanceGeneration string) error {
	labelSelector, err := createLabelSelector(r.Instance.Namespace, instanceGeneration)
	if err != nil {
		return err
	}
	objects := &unstructured.UnstructuredList{}
	objects.SetGroupVersionKind(gvk)
	err = r.Client.List(ctx, objects, client.MatchingLabelsSelector{Selector: labelSelector})
	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error retrieving resources to prune: %v", err)
	}
	for _, object := range objects.Items {
		err = r.Client.Delete(ctx, &object, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("error deleting resource: %v", err)
		}
	}
	return nil
}

func (r *controlPlaneInstanceReconciler) pruneAll(ctx context.Context, gvk schema.GroupVersionKind, instanceGeneration string) error {
	labelSelector, err := createLabelSelector(r.Instance.Namespace, instanceGeneration)
	if err != nil {
		return err
	}

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)
	err = r.Client.DeleteAllOf(ctx,
		object,
		client.MatchingLabelsSelector{Selector: labelSelector},
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

func gk(group, kind string) schema.GroupKind {
	return schema.GroupKind{
		Group: group,
		Kind:  kind,
	}
}

func createLabelSelector(meshNamespace, meshGeneration string) (labels.Selector, error) {
	ownerRequirement, err := labels.NewRequirement(common.OwnerKey, selection.Equals, []string{meshNamespace})
	if err != nil {
		return nil, err
	}
	generationRequirement, err := labels.NewRequirement(common.KubernetesAppVersionKey, selection.NotEquals, []string{meshGeneration})
	if err != nil {
		return nil, err
	}
	labelsSelector := labels.NewSelector().Add(*ownerRequirement, *generationRequirement)
	return labelsSelector, nil
}
