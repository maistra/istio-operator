package controlplane

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

type CNIPruner struct {
	Client            client.Client
	OperatorNamespace string
}

type resource struct {
	obj            runtime.Object
	namespaced     bool
	name           string
	usedInVersions []string
}

var cniResources = []resource{
	toResource(&rbacv1.ClusterRole{}, false, "istio-cni", versions.V2_0, versions.V2_1, versions.V2_2, versions.V2_3),
	toResource(&rbacv1.ClusterRole{}, false, "ossm-cni", versions.V2_4, versions.V2_5, versions.V2_6),
	toResource(&rbacv1.ClusterRoleBinding{}, false, "istio-cni", versions.V2_0, versions.V2_1, versions.V2_2, versions.V2_3),
	toResource(&rbacv1.ClusterRoleBinding{}, false, "ossm-cni", versions.V2_4, versions.V2_5, versions.V2_6),
	toResource(&corev1.ConfigMap{}, true, "istio-cni-config", versions.V2_0, versions.V2_1, versions.V2_2),
	toResource(&corev1.ConfigMap{}, true, "istio-cni-config-v2-3", versions.V2_3),
	toResource(&corev1.ConfigMap{}, true, "ossm-cni-config-v2-4", versions.V2_4),
	toResource(&corev1.ConfigMap{}, true, "ossm-cni-config-v2-5", versions.V2_5),
	toResource(&corev1.ConfigMap{}, true, "ossm-cni-config-v2-6", versions.V2_6),
	toResource(&appsv1.DaemonSet{}, true, "istio-cni-node", versions.V2_0, versions.V2_1, versions.V2_2),
	toResource(&appsv1.DaemonSet{}, true, "istio-cni-node-v2-3", versions.V2_3),
	toResource(&appsv1.DaemonSet{}, true, "istio-cni-node-v2-4", versions.V2_4),
	toResource(&appsv1.DaemonSet{}, true, "istio-cni-node-v2-5", versions.V2_5),
	toResource(&appsv1.DaemonSet{}, true, "istio-cni-node-v2-6", versions.V2_6),
	toResource(&corev1.ServiceAccount{}, true, "istio-cni", versions.V2_0, versions.V2_1, versions.V2_2, versions.V2_3),
	toResource(&corev1.ServiceAccount{}, true, "ossm-cni", versions.V2_4, versions.V2_5, versions.V2_6),
}

func toResource(obj runtime.Object, namespaced bool, name string, versions ...versions.Version) resource {
	r := resource{
		obj:        obj,
		namespaced: namespaced,
		name:       name,
	}
	for _, v := range versions {
		r.usedInVersions = append(r.usedInVersions, v.String())
	}
	return r
}

func (c *CNIPruner) Prune(ctx context.Context, smcp *maistrav2.ServiceMeshControlPlane) error {
	smcpList := maistrav2.ServiceMeshControlPlaneList{}
	err := c.Client.List(ctx, &smcpList)
	if err != nil {
		return err
	}

	deployedVersions := sets.NewString()
	for _, s := range smcpList.Items {
		if s.DeletionTimestamp == nil || s.UID != smcp.UID {
			deployedVersions.Insert(s.Spec.Version)
		}
	}

	for _, res := range cniResources {
		inUse := deployedVersions.HasAny(res.usedInVersions...)
		if inUse {
			continue
		}

		key := types.NamespacedName{Name: res.name}
		if res.namespaced {
			key.Namespace = c.OperatorNamespace
		}

		obj := res.obj.DeepCopyObject()

		if err = c.Client.Get(ctx, key, obj); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return err
		}

		// ignore resources not created by this operator
		if objMeta, ok := obj.(v1.ObjectMetaAccessor); !ok {
			return fmt.Errorf("failed to get object metadata for %s %s", obj.GetObjectKind().GroupVersionKind(), key)
		} else {
			labels := objMeta.GetObjectMeta().GetLabels()
			if labels[common.KubernetesAppManagedByKey] != common.KubernetesAppManagedByValue {
				continue
			}
		}

		if err = c.Client.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
