package controlplane

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

const operatorNamespace = "openshift-operators"

func TestCNIPrune(t *testing.T) {
	maistra := map[string]string{
		common.KubernetesAppManagedByKey: common.KubernetesAppManagedByValue,
	}

	sail := map[string]string{
		common.KubernetesAppManagedByKey: "sail-operator",
	}

	tests := []struct {
		name              string
		smcpVersion       versions.Version
		smcpDeleted       bool
		otherSmcpVersions []versions.Version
		objectsToDelete   []runtime.Object
		objectsToPreserve []runtime.Object
	}{
		{
			name:        "v2.6",
			smcpVersion: versions.V2_6,
			smcpDeleted: true,
			objectsToDelete: []runtime.Object{
				clusterRole("ossm-cni", maistra),
				clusterRoleBinding("ossm-cni", maistra),
				configMap("ossm-cni-config-v2-6", maistra),
				daemonSet("istio-cni-node-v2-6", maistra),
				serviceAccount("ossm-cni", maistra),
			},
			objectsToPreserve: []runtime.Object{},
		},
		{
			name:        "v2.5",
			smcpVersion: versions.V2_5,
			smcpDeleted: true,
			objectsToDelete: []runtime.Object{
				clusterRole("ossm-cni", maistra),
				clusterRoleBinding("ossm-cni", maistra),
				configMap("ossm-cni-config-v2-5", maistra),
				daemonSet("istio-cni-node-v2-5", maistra),
				serviceAccount("ossm-cni", maistra),
			},
			objectsToPreserve: []runtime.Object{},
		},
		{
			name:        "v2.4",
			smcpVersion: versions.V2_4,
			smcpDeleted: true,
			objectsToDelete: []runtime.Object{
				clusterRole("ossm-cni", maistra),
				clusterRoleBinding("ossm-cni", maistra),
				configMap("ossm-cni-config-v2-4", maistra),
				daemonSet("istio-cni-node-v2-4", maistra),
				serviceAccount("ossm-cni", maistra),
			},
			objectsToPreserve: []runtime.Object{},
		},
		{
			name:        "v2.3",
			smcpVersion: versions.V2_3,
			smcpDeleted: true,
			objectsToDelete: []runtime.Object{
				clusterRole("istio-cni", maistra),
				clusterRoleBinding("istio-cni", maistra),
				configMap("istio-cni-config-v2-3", maistra),
				daemonSet("istio-cni-node-v2-3", maistra),
				serviceAccount("ossm-cni", maistra),
			},
			objectsToPreserve: []runtime.Object{},
		},
		{
			name:        "v2.2",
			smcpVersion: versions.V2_2,
			smcpDeleted: true,
			objectsToDelete: []runtime.Object{
				clusterRole("istio-cni", maistra),
				clusterRoleBinding("istio-cni", maistra),
				configMap("istio-cni-config", maistra),
				daemonSet("istio-cni-node", maistra),
				serviceAccount("istio-cni", maistra),
			},
			objectsToPreserve: []runtime.Object{},
		},
		{
			name:        "v2.1",
			smcpVersion: versions.V2_1,
			smcpDeleted: true,
			objectsToDelete: []runtime.Object{
				clusterRole("istio-cni", maistra),
				clusterRoleBinding("istio-cni", maistra),
				configMap("istio-cni-config", maistra),
				daemonSet("istio-cni-node", maistra),
				serviceAccount("istio-cni", maistra),
			},
			objectsToPreserve: []runtime.Object{},
		},
		{
			name:        "v2.0",
			smcpVersion: versions.V2_0,
			smcpDeleted: true,
			objectsToDelete: []runtime.Object{
				clusterRole("istio-cni", maistra),
				clusterRoleBinding("istio-cni", maistra),
				configMap("istio-cni-config", maistra),
				daemonSet("istio-cni-node", maistra),
				serviceAccount("istio-cni", maistra),
			},
			objectsToPreserve: []runtime.Object{},
		},
		{
			name:            "preserves non-maistra resources",
			smcpVersion:     versions.V2_6,
			smcpDeleted:     true,
			objectsToDelete: []runtime.Object{},
			objectsToPreserve: []runtime.Object{
				// this must be preserved, because it wasn't deployed by the Maistra istio operator (but by e.g. the Sail operator)
				configMap("ossm-cni-config-v2-6", sail),
			},
		},
		{
			name:            "preserves resources when v2.6 SMCP is not deleted",
			smcpVersion:     versions.V2_6,
			smcpDeleted:     false,
			objectsToDelete: []runtime.Object{},
			objectsToPreserve: []runtime.Object{
				// these must be preserved, because the SMCP isn't deleted
				clusterRole("ossm-cni", maistra),
				clusterRoleBinding("ossm-cni", maistra),
				configMap("ossm-cni-config-v2-6", maistra),
				daemonSet("istio-cni-node-v2-6", maistra),
				serviceAccount("ossm-cni", maistra),
			},
		},
		{
			name:              "preserves resources when another v2.6 SMCP is present",
			smcpVersion:       versions.V2_6,
			smcpDeleted:       true,
			otherSmcpVersions: []versions.Version{versions.V2_6},
			objectsToDelete:   []runtime.Object{},
			objectsToPreserve: []runtime.Object{
				// these must be preserved, because only one v2.6 is being deleted, while the other v2.6 still needs them
				clusterRole("ossm-cni", maistra),
				clusterRoleBinding("ossm-cni", maistra),
				configMap("ossm-cni-config-v2-6", maistra),
				daemonSet("istio-cni-node-v2-6", maistra),
				serviceAccount("ossm-cni", maistra),
			},
		},
		{
			name:              "preserves resources shared between v2.6 and v2.5",
			smcpVersion:       versions.V2_6,
			smcpDeleted:       true,
			otherSmcpVersions: []versions.Version{versions.V2_5},
			objectsToDelete: []runtime.Object{
				configMap("ossm-cni-config-v2-6", maistra),
				daemonSet("istio-cni-node-v2-6", maistra),
			},
			objectsToPreserve: []runtime.Object{
				// these three must be preserved, because they are shared between v2.6 and v2.5
				clusterRole("ossm-cni", maistra),
				clusterRoleBinding("ossm-cni", maistra),
				serviceAccount("ossm-cni", maistra),
				// these two must be preserved, because they are not associated with v2.6
				configMap("ossm-cni-config-v2-5", maistra),
				daemonSet("istio-cni-node-v2-5", maistra),
			},
		},
		{
			name:              "preserves resources shared between v2.6 and v2.4",
			smcpVersion:       versions.V2_6,
			smcpDeleted:       true,
			otherSmcpVersions: []versions.Version{versions.V2_4},
			objectsToDelete: []runtime.Object{
				configMap("ossm-cni-config-v2-6", maistra),
				daemonSet("istio-cni-node-v2-6", maistra),
			},
			objectsToPreserve: []runtime.Object{
				// these three must be preserved, because they are shared between v2.6 and v2.4
				clusterRole("ossm-cni", maistra),
				clusterRoleBinding("ossm-cni", maistra),
				serviceAccount("ossm-cni", maistra),
				// these two must be preserved, because they are not associated with v2.6
				configMap("ossm-cni-config-v2-4", maistra),
				daemonSet("istio-cni-node-v2-4", maistra),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			smcp := &v2.ServiceMeshControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-smcp",
					Namespace: "istio-system",
					UID:       "123",
				},
				Spec: v2.ControlPlaneSpec{Version: tc.smcpVersion.String()},
			}
			if tc.smcpDeleted {
				smcp.DeletionTimestamp = &oneMinuteAgo
			}

			otherSMCPs := []runtime.Object{}
			for i, v := range tc.otherSmcpVersions {
				otherSMCPs = append(otherSMCPs, &v2.ServiceMeshControlPlane{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("other-smcp-%d", i),
						Namespace: fmt.Sprintf("istio-system-%d", i),
						UID:       types.UID(strconv.Itoa(1000 + i)),
					},
					Spec: v2.ControlPlaneSpec{Version: v.String()},
				})
			}

			objects := append(tc.objectsToDelete, tc.objectsToPreserve...)
			objects = append(objects, smcp)
			objects = append(objects, otherSMCPs...)

			cl, _ := test.CreateClient(objects...)

			c := &CNIPruner{
				Client:            cl,
				OperatorNamespace: operatorNamespace,
			}

			if err := c.Prune(ctx, smcp); err != nil {
				t.Errorf("Prune() error = %v", err)
			}

			// Verify that the objects to preserve are still there
			for _, obj := range tc.objectsToPreserve {
				key, err := client.ObjectKeyFromObject(obj)
				if err != nil {
					t.Fatalf("error getting key from object: %v", err.Error())
				}
				if err = cl.Get(ctx, key, obj); err != nil {
					t.Errorf("Prune() failed to preserve object %v %v: %v", reflect.TypeOf(obj), key, err)
				}
			}

			// Verify that the objects to delete are gone
			for _, obj := range tc.objectsToDelete {
				key, err := client.ObjectKeyFromObject(obj)
				if err != nil {
					t.Fatalf("error getting key from object: %v", err.Error())
				}
				if err = cl.Get(ctx, key, obj); err == nil {
					t.Errorf("Prune() failed to delete object %v %v", reflect.TypeOf(obj), key)
				}
			}
		})
	}
}

func Must(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func configMap(name string, labels map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: operatorNamespace,
			Labels:    labels,
		},
	}
}

func daemonSet(name string, labels map[string]string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: operatorNamespace,
			Labels:    labels,
		},
	}
}

func serviceAccount(name string, labels map[string]string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: operatorNamespace,
			Labels:    labels,
		},
	}
}

func clusterRole(name string, labels map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func clusterRoleBinding(name string, labels map[string]string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}
