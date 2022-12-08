package controlplane

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/controllers/common"
	"github.com/maistra/istio-operator/controllers/common/cni"
	"github.com/maistra/istio-operator/controllers/common/test"
)

func TestPrune(t *testing.T) {
	operatorNamespace := "operator-namespace"
	controlPlaneNamespace := "control-plane-namespace"
	unrelatedNamespace := "unrelated-namespace"

	previousMeshGeneration := "test-1"
	currentMeshGeneration := "test-2"

	cases := []struct {
		name              string
		gvk               schema.GroupVersionKind
		createObject      func() client.Object
		namespaced        bool
		pruneIndividually bool
	}{
		{
			name:              "namespaced-pruneAll",
			gvk:               gvk("apps", "v1", "Deployment"),
			createObject:      func() client.Object { return &appsv1.Deployment{} },
			namespaced:        true,
			pruneIndividually: false,
		},
		{
			name:              "namespaced-pruneIndividually",
			gvk:               gvk("apps", "v1", "Deployment"),
			createObject:      func() client.Object { return &appsv1.Deployment{} },
			namespaced:        true,
			pruneIndividually: true,
		},
		{
			name:              "clusterscoped-pruneAll",
			gvk:               gvk("rbac.authorization.k8s.io", "v1", "ClusterRole"),
			createObject:      func() client.Object { return &v1.ClusterRole{} },
			namespaced:        false,
			pruneIndividually: false,
		},
		{
			name:              "clusterscoped-pruneIndividually",
			gvk:               gvk("rbac.authorization.k8s.io", "v1", "ClusterRole"),
			createObject:      func() client.Object { return &v1.ClusterRole{} },
			namespaced:        false,
			pruneIndividually: true,
		},
	}

	subcases := []struct {
		name           string
		ns             string
		owner          string
		version        string
		expectDeletion bool
	}{
		{
			name:           "delete-object-with-previous-version",
			owner:          controlPlaneNamespace,
			version:        previousMeshGeneration,
			expectDeletion: true,
		},
		{
			name:           "preserve-object-with-current-version",
			ns:             controlPlaneNamespace,
			owner:          controlPlaneNamespace,
			version:        currentMeshGeneration,
			expectDeletion: false,
		},
		{
			name:           "preserve-object-with-different-owner",
			owner:          "other-control-plane-namespace",
			version:        currentMeshGeneration,
			expectDeletion: false,
		},
		{
			name:           "preserve-object-with-no-owner",
			ns:             controlPlaneNamespace,
			owner:          "",
			version:        "",
			expectDeletion: false,
		},
	}

	for _, tc := range cases {
		for _, sc := range subcases {
			var namespaces []string
			if tc.namespaced {
				// check if object is pruned regardless of the namespace it belongs to
				namespaces = []string{operatorNamespace, controlPlaneNamespace, unrelatedNamespace}
			} else {
				namespaces = []string{""}
			}
			for _, ns := range namespaces {
				t.Run(tc.name+"-"+sc.name+"-in-"+ns, func(t *testing.T) {
					obj := tc.createObject()
					o := obj.(metav1.Object)
					o.SetName("test")
					o.SetNamespace(ns)
					if sc.owner != "" {
						o.SetLabels(map[string]string{
							common.OwnerKey:                  sc.owner,
							common.KubernetesAppManagedByKey: common.KubernetesAppManagedByValue,
							common.KubernetesAppVersionKey:   sc.version,
						})
						o.SetAnnotations(map[string]string{
							common.MeshGenerationKey: sc.version,
						})
					}

					smcp := newControlPlane()
					smcp.Namespace = controlPlaneNamespace

					cl, tracker := test.CreateClient(smcp, obj)
					fakeEventRecorder := &record.FakeRecorder{}

					r := &controlPlaneInstanceReconciler{
						ControllerResources: common.ControllerResources{
							Client:            cl,
							Scheme:            tracker.Scheme,
							EventRecorder:     fakeEventRecorder,
							OperatorNamespace: operatorNamespace,
						},
						Instance:  smcp,
						Status:    smcp.Status.DeepCopy(),
						cniConfig: cni.Config{Enabled: true},
					}

					var err error
					if tc.pruneIndividually {
						err = r.pruneIndividually(ctx, tc.gvk, currentMeshGeneration)
					} else {
						err = r.pruneAll(ctx, tc.gvk, currentMeshGeneration)
					}
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

					namespacedName := common.ToNamespacedName(obj.(metav1.Object))
					if sc.expectDeletion {
						test.AssertNotFound(ctx, cl, namespacedName, obj, "Expected prune() to delete object, but it didn't", t)
					} else {
						test.AssertObjectExists(ctx, cl, namespacedName, obj, "Expected prune() to preserve object, but it didn't", t)
					}
				})
			}
		}
	}
}
