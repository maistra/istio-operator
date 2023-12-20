// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package istiorevision

import (
	"context"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/scheme"
	v1 "maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/common"
	"maistra.io/istio-operator/pkg/test"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"istio.io/istio/pkg/ptr"
)

var testConfig = common.OperatorConfig{}

const (
	istioVersion = "latest"
	pilotImage   = "maistra.io/test:latest"
)

var _ = Describe("IstioRevisionController", Ordered, func() {
	const istioName = "test-istio"
	const istioNamespace = "test"

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioNamespace,
		},
	}

	istioObjectKey := client.ObjectKey{Name: istioName}
	deploymentObjectKey := client.ObjectKey{Name: "istiod-" + istioName, Namespace: istioNamespace}
	cniObjectKey := client.ObjectKey{Name: "istio-cni-node", Namespace: operatorNamespace}
	webhookObjectKey := client.ObjectKey{Name: "istio-sidecar-injector-" + istioName + "-" + istioNamespace}

	common.Config = testConfig

	BeforeAll(func() {
		By("Creating the Namespace to perform the tests")
		err := k8sClient.Create(ctx, namespace)
		Expect(err).To(Not(HaveOccurred()))
	})

	AfterAll(func() {
		// TODO(user): Attention if you improve this code by adding other context test you MUST
		// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		By("Deleting the Namespace to perform the tests")
		_ = k8sClient.Delete(ctx, namespace)
	})

	istio := &v1.IstioRevision{}

	It("successfully reconciles the resource", func() {
		By("Creating the custom resource")
		err := k8sClient.Get(ctx, istioObjectKey, istio)
		if err != nil && errors.IsNotFound(err) {
			istio = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioVersion,
					Namespace: istioNamespace,
					Values: []byte(`{
						"global":{"istioNamespace":"` + istioNamespace + `"},
						"revision":"` + istioName + `",
						"pilot":{"image":"` + pilotImage + `"},
						"istio_cni":{"enabled":true}
					}`),
				},
			}

			ExpectSuccess(k8sClient.Create(ctx, istio))
		}

		By("Checking if the resource was successfully created")
		Eventually(func() error {
			found := &v1.IstioRevision{}
			return k8sClient.Get(ctx, istioObjectKey, found)
		}, time.Minute, time.Second).Should(Succeed())

		istiodDeployment := &appsv1.Deployment{}
		By("Checking if Deployment was successfully created in the reconciliation")
		Eventually(func() error {
			return k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
		}, time.Minute, time.Second).Should(Succeed())
		Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
		Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(istio)))

		By("Checking if the status is updated")
		Eventually(func() int64 {
			ExpectSuccess(k8sClient.Get(ctx, istioObjectKey, istio))
			return istio.Status.ObservedGeneration
		}, time.Minute, time.Second).Should(Equal(istio.ObjectMeta.Generation))
	})

	When("istiod and istio-cni-node readiness changes", func() {
		It("marks updates the status of the istio resource", func() {
			By("setting the Ready condition status to true when both are ready", func() {
				istiodDeployment := &appsv1.Deployment{}
				ExpectSuccess(k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment))
				istiodDeployment.Status.Replicas = 1
				istiodDeployment.Status.ReadyReplicas = 1
				ExpectSuccess(k8sClient.Status().Update(ctx, istiodDeployment))

				cniDaemonSet := &appsv1.DaemonSet{}
				ExpectSuccess(k8sClient.Get(ctx, cniObjectKey, cniDaemonSet))
				cniDaemonSet.Status.CurrentNumberScheduled = 3
				cniDaemonSet.Status.NumberReady = 3
				ExpectSuccess(k8sClient.Status().Update(ctx, cniDaemonSet))

				Eventually(func() metav1.ConditionStatus {
					ExpectSuccess(k8sClient.Get(ctx, istioObjectKey, istio))
					return istio.Status.GetCondition(v1.IstioRevisionConditionTypeReady).Status
				}, time.Minute, time.Second).Should(Equal(metav1.ConditionTrue))
			})

			By("setting the Ready condition status to false when istiod isn't ready", func() {
				istiodDeployment := &appsv1.Deployment{}
				ExpectSuccess(k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment))

				istiodDeployment.Status.ReadyReplicas = 0
				ExpectSuccess(k8sClient.Status().Update(ctx, istiodDeployment))

				Eventually(func() metav1.ConditionStatus {
					ExpectSuccess(k8sClient.Get(ctx, istioObjectKey, istio))
					return istio.Status.GetCondition(v1.IstioRevisionConditionTypeReady).Status
				}, time.Minute, time.Second).Should(Equal(metav1.ConditionFalse))
			})
		})
	})

	When("an owned namespaced resource is deleted", func() {
		It("recreates the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentObjectKey.Name,
					Namespace: deploymentObjectKey.Namespace,
				},
			}
			ExpectSuccess(k8sClient.Delete(ctx, istiodDeployment, client.PropagationPolicy(metav1.DeletePropagationForeground)))

			Eventually(func() error {
				return k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
			}, time.Minute, time.Second).Should(Succeed())

			Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
			Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(istio)))
		})
	})

	When("an owned cluster-scoped resource is deleted", func() {
		It("recreates the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: webhookObjectKey.Name,
				},
			}
			ExpectSuccess(k8sClient.Delete(ctx, webhook, client.PropagationPolicy(metav1.DeletePropagationForeground)))

			Eventually(func() error {
				err := k8sClient.Get(ctx, webhookObjectKey, webhook)
				return err
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	When("an owned namespaced resource is modified", func() {
		It("reverts the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{}
			ExpectSuccess(k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment))

			originalImage := istiodDeployment.Spec.Template.Spec.Containers[0].Image
			istiodDeployment.Spec.Template.Spec.Containers[0].Image = "user-supplied-image"
			ExpectSuccess(k8sClient.Update(ctx, istiodDeployment))

			Eventually(func() string {
				ExpectSuccess(k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment))
				return istiodDeployment.Spec.Template.Spec.Containers[0].Image
			}, time.Minute, time.Second).Should(Equal(originalImage))
		})
	})

	When("an owned cluster-scoped resource is modified", func() {
		It("reverts the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{}
			ExpectSuccess(k8sClient.Get(ctx, webhookObjectKey, webhook))

			origWebhooks := webhook.Webhooks
			webhook.Webhooks = []admissionv1.MutatingWebhook{}
			ExpectSuccess(k8sClient.Update(ctx, webhook))

			Eventually(func() []admissionv1.MutatingWebhook {
				ExpectSuccess(k8sClient.Get(ctx, webhookObjectKey, webhook))
				return webhook.Webhooks
			}, time.Minute, time.Second).Should(Equal(origWebhooks))
		})
	})

	It("supports concurrent deployment of two control planes", func() {
		rev2Name := istioName + "2"
		rev2ObjectKey := client.ObjectKey{Name: rev2Name}
		deployment2ObjectKey := client.ObjectKey{Name: "istiod-" + rev2Name, Namespace: istioNamespace}

		rev2 := &v1.IstioRevision{}

		By("Creating the second IstioRevision instance")
		err := k8sClient.Get(ctx, rev2ObjectKey, rev2)
		if err != nil && errors.IsNotFound(err) {
			rev2 = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: rev2ObjectKey.Name,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioVersion,
					Namespace: istioNamespace,
					Values: []byte(`{
						"global":{"istioNamespace":"` + istioNamespace + `"},
						"revision": "` + rev2ObjectKey.Name + `",
						"pilot":{"image":"` + pilotImage + `"}
					}`),
				},
			}

			ExpectSuccess(k8sClient.Create(ctx, rev2))
		}

		By("Checking if the resource was successfully created")
		Eventually(func() error {
			return k8sClient.Get(ctx, rev2ObjectKey, &v1.IstioRevision{})
		}, time.Minute, time.Second).Should(Succeed())

		By("Checking if the status is updated")
		Eventually(func() int64 {
			ExpectSuccess(k8sClient.Get(ctx, rev2ObjectKey, rev2))
			return rev2.Status.ObservedGeneration
		}, time.Minute, time.Second).Should(Equal(rev2.ObjectMeta.Generation))

		istiodDeployment := &appsv1.Deployment{}
		By("Checking if Deployment was successfully created in the reconciliation")
		Eventually(func() error {
			return k8sClient.Get(ctx, deployment2ObjectKey, istiodDeployment)
		}, time.Minute, time.Second).Should(Succeed())
		Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
		Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference(rev2)))
	})
})

func expectedOwnerReference(istio *v1.IstioRevision) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioRevisionKind,
		Name:               istio.Name,
		UID:                istio.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}
}

func TestDeriveState(t *testing.T) {
	testCases := []struct {
		name                string
		reconciledCondition v1.IstioRevisionCondition
		readyCondition      v1.IstioRevisionCondition
		expectedState       v1.IstioRevisionConditionReason
	}{
		{
			name:                "healthy",
			reconciledCondition: newCondition(v1.IstioRevisionConditionTypeReconciled, true, ""),
			readyCondition:      newCondition(v1.IstioRevisionConditionTypeReady, true, ""),
			expectedState:       v1.IstioRevisionConditionReasonHealthy,
		},
		{
			name:                "not reconciled",
			reconciledCondition: newCondition(v1.IstioRevisionConditionTypeReconciled, false, v1.IstioRevisionConditionReasonReconcileError),
			readyCondition:      newCondition(v1.IstioRevisionConditionTypeReady, true, ""),
			expectedState:       v1.IstioRevisionConditionReasonReconcileError,
		},
		{
			name:                "not ready",
			reconciledCondition: newCondition(v1.IstioRevisionConditionTypeReconciled, true, ""),
			readyCondition:      newCondition(v1.IstioRevisionConditionTypeReady, false, v1.IstioRevisionConditionReasonIstiodNotReady),
			expectedState:       v1.IstioRevisionConditionReasonIstiodNotReady,
		},
		{
			name:                "not reconciled nor ready",
			reconciledCondition: newCondition(v1.IstioRevisionConditionTypeReconciled, false, v1.IstioRevisionConditionReasonReconcileError),
			readyCondition:      newCondition(v1.IstioRevisionConditionTypeReady, false, v1.IstioRevisionConditionReasonIstiodNotReady),
			expectedState:       v1.IstioRevisionConditionReasonReconcileError, // reconcile reason takes precedence over ready reason
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := deriveState(tc.reconciledCondition, tc.readyCondition)
			if result != tc.expectedState {
				t.Errorf("Expected reason %s, but got %s", tc.expectedState, result)
			}
		})
	}
}

func newCondition(conditionType v1.IstioRevisionConditionType, status bool, reason v1.IstioRevisionConditionReason) v1.IstioRevisionCondition {
	st := metav1.ConditionFalse
	if status {
		st = metav1.ConditionTrue
	}
	return v1.IstioRevisionCondition{
		Type:   conditionType,
		Status: st,
		Reason: reason,
	}
}

func TestDetermineReadyCondition(t *testing.T) {
	testCases := []struct {
		name          string
		cniEnabled    bool
		values        string
		clientObjects []client.Object
		expected      v1.IstioRevisionCondition
	}{
		{
			name:   "Istiod ready",
			values: "",
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:   v1.IstioRevisionConditionTypeReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name:   "Istiod not ready",
			values: "",
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     1,
						AvailableReplicas: 1,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionConditionReasonIstiodNotReady,
				Message: "not all istiod pods are ready",
			},
		},
		{
			name:   "Istiod scaled to zero",
			values: "",
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          0,
						ReadyReplicas:     0,
						AvailableReplicas: 0,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionConditionReasonIstiodNotReady,
				Message: "istiod Deployment is scaled to zero replicas",
			},
		},
		{
			name:          "Istiod not found",
			values:        ``,
			clientObjects: []client.Object{},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionConditionReasonIstiodNotReady,
				Message: "istiod Deployment not found",
			},
		},
		{
			name: "Istiod and CNI ready",
			values: `
istio_cni:
  enabled: true
`,
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: operatorNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 3,
						NumberReady:            3,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:   v1.IstioRevisionConditionTypeReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "CNI not ready",
			values: `
istio_cni:
  enabled: true
`,
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: operatorNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 1,
						NumberReady:            0,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionConditionReasonCNINotReady,
				Message: "not all istio-cni-node pods are ready",
			},
		},
		{
			name: "CNI pods not scheduled",
			values: `
istio_cni:
  enabled: true
`,
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: operatorNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 0,
						NumberReady:            0,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionConditionReasonCNINotReady,
				Message: "no istio-cni-node pods are currently scheduled",
			},
		},
		{
			name: "CNI not found",
			values: `
istio_cni:
  enabled: true
`,
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionTypeReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionConditionReasonCNINotReady,
				Message: "istio-cni-node DaemonSet not found",
			},
		},
		{
			name:   "Non-default revision",
			values: "revision: my-revision",
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod-my-revision",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:   v1.IstioRevisionConditionTypeReady,
				Status: metav1.ConditionTrue,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tt.clientObjects...).Build()

			r := NewIstioRevisionReconciler(cl, scheme.Scheme, nil, operatorNamespace)

			var values map[string]any
			Must(t, yaml.Unmarshal([]byte(tt.values), &values))

			rev := &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-istio",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
				},
			}

			result, err := r.determineReadyCondition(context.TODO(), rev, values)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Type != tt.expected.Type || result.Status != tt.expected.Status ||
				result.Reason != tt.expected.Reason || result.Message != tt.expected.Message {
				t.Errorf("Unexpected result.\nGot:\n    %+v\nexpected:\n    %+v", result, tt.expected)
			}
		})
	}
}

func TestDetermineInUseCondition(t *testing.T) {
	test.SetupScheme()

	testCases := []struct {
		podLabels           map[string]string
		podAnnotations      map[string]string
		nsLabels            map[string]string
		enableAllNamespaces bool
		matchesRevision     string
	}{
		// no labels on namespace or pod
		{
			nsLabels:        map[string]string{},
			podLabels:       map[string]string{},
			matchesRevision: "",
		},

		// namespace labels only
		{
			nsLabels:        map[string]string{"istio-injection": "enabled"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			matchesRevision: "default",
		},

		// pod labels only
		{
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},
		{
			podLabels:       map[string]string{"sidecar.istio.io/inject": "true"},
			matchesRevision: "default",
		},
		{
			podLabels:       map[string]string{"sidecar.istio.io/inject": "true", "istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},

		// ns and pod labels
		{
			nsLabels:        map[string]string{"istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "my-rev",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},

		// special case: when Values.sidecarInjectorWebhook.enableNamespacesByDefault is true, all pods should match the default revision
		// unless they are in one of the system namespaces ("kube-system","kube-public","kube-node-lease","local-path-storage")
		{
			enableAllNamespaces: true,
			matchesRevision:     "default",
		},
	}

	for _, revName := range []string{"default", "my-rev"} {
		for _, tc := range testCases {
			nameBuilder := strings.Builder{}
			nameBuilder.WriteString(revName + ":")
			if len(tc.nsLabels) == 0 && len(tc.podLabels) == 0 {
				nameBuilder.WriteString("no labels")
			}
			if len(tc.nsLabels) > 0 {
				nameBuilder.WriteString("NS:")
				for k, v := range tc.nsLabels {
					nameBuilder.WriteString(k + ":" + v + ",")
				}
			}
			if len(tc.podLabels) > 0 {
				nameBuilder.WriteString("POD:")
				for k, v := range tc.podLabels {
					nameBuilder.WriteString(k + ":" + v + ",")
				}
			}
			name := strings.TrimSuffix(nameBuilder.String(), ",")

			t.Run(name, func(t *testing.T) {
				rev := &v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: revName,
					},
					Spec: v1.IstioRevisionSpec{
						Namespace: "istio-system",
						Version:   "my-version",
					},
				}
				if tc.enableAllNamespaces {
					rev.Spec.Values = []byte(`{"sidecarInjectorWebhook":{"enableNamespacesByDefault":true}}`)
				}

				namespace := "bookinfo"
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   namespace,
						Labels: tc.nsLabels,
					},
				}

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "some-pod",
						Namespace:   namespace,
						Labels:      tc.podLabels,
						Annotations: tc.podAnnotations,
					},
				}

				cl := fake.NewClientBuilder().
					WithScheme(scheme.Scheme).
					WithObjects(rev, ns, pod).
					Build()

				r := NewIstioRevisionReconciler(cl, scheme.Scheme, nil, operatorNamespace)

				result, err := r.determineInUseCondition(context.TODO(), rev)
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if result.Type != v1.IstioRevisionConditionTypeInUse {
					t.Errorf("unexpected condition type: %v", result.Type)
				}

				expectedStatus := metav1.ConditionFalse
				if revName == tc.matchesRevision {
					expectedStatus = metav1.ConditionTrue
				}

				if result.Status != expectedStatus {
					t.Errorf("Unexpected status. Revision %s reports being in use, but shouldn't be\n"+
						"revision: %s\nexpected revision: %s\nnamespace labels: %+v\npod labels: %+v",
						revName, revName, tc.matchesRevision, tc.nsLabels, tc.podLabels)
				}
			})
		}
	}
}

func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func ExpectSuccess(err error) {
	GinkgoHelper()
	Expect(err).NotTo(HaveOccurred())
}
