package controllers

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1 "maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var testConfig = common.OperatorConfig{}

const (
	istioVersion = "latest"
	pilotImage   = "maistra.io/test:latest"
)

var _ = ginkgo.Describe("IstioController", ginkgo.Ordered, func() {
	const istioName = "test-istio"
	const istioNamespace = "test"

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioNamespace,
		},
	}

	istioObjectKey := client.ObjectKey{Name: istioName, Namespace: istioNamespace}
	deploymentObjectKey := client.ObjectKey{Name: "istiod", Namespace: istioNamespace}
	cniObjectKey := client.ObjectKey{Name: "istio-cni-node", Namespace: operatorNamespace}
	webhookObjectKey := client.ObjectKey{Name: "istio-sidecar-injector-" + istioNamespace}

	common.Config = testConfig

	ginkgo.BeforeAll(func() {
		ginkgo.By("Creating the Namespace to perform the tests")
		err := k8sClient.Create(ctx, namespace)
		gomega.Expect(err).To(gomega.Not(gomega.HaveOccurred()))
	})

	ginkgo.AfterAll(func() {
		// TODO(user): Attention if you improve this code by adding other context test you MUST
		// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		ginkgo.By("Deleting the Namespace to perform the tests")
		_ = k8sClient.Delete(ctx, namespace)
	})

	istio := &v1.Istio{}

	ginkgo.It("successfully reconciles the resource", func() {
		ginkgo.By("Creating the custom resource")
		err := k8sClient.Get(ctx, istioObjectKey, istio)
		if err != nil && errors.IsNotFound(err) {
			istio = &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name:      istioName,
					Namespace: istioNamespace,
				},
				Spec: v1.IstioSpec{
					Version: istioVersion,
					Values:  []byte(`{"pilot":{"image":"` + pilotImage + `"}}`),
				},
			}

			err = k8sClient.Create(ctx, istio)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}

		ginkgo.By("Checking if the resource was successfully created")
		gomega.Eventually(func() error {
			found := &v1.Istio{}
			return k8sClient.Get(ctx, istioObjectKey, found)
		}, time.Minute, time.Second).Should(gomega.Succeed())

		istiodDeployment := &appsv1.Deployment{}
		ginkgo.By("Checking if Deployment was successfully created in the reconciliation")
		gomega.Eventually(func() error {
			return k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
		}, time.Minute, time.Second).Should(gomega.Succeed())
		gomega.Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal(pilotImage))
		gomega.Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(gomega.ContainElement(expectedOwnerReference(istio)))

		ginkgo.By("Checking if the status is updated")
		gomega.Eventually(func() int64 {
			err := k8sClient.Get(ctx, istioObjectKey, istio)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			return istio.Status.ObservedGeneration
		}, time.Minute, time.Second).Should(gomega.Equal(istio.ObjectMeta.Generation))

		ginkgo.By("Checking if the appliedValues are written properly")
		gomega.Eventually(func() string {
			err := k8sClient.Get(ctx, istioObjectKey, istio)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			vals := istio.Status.GetAppliedValues()
			imageName, _, err := unstructured.NestedString(vals, "pilot", "image")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			return imageName
		}, time.Minute, time.Second).Should(gomega.Equal(pilotImage))
	})

	ginkgo.When("istiod and istio-cni-node readiness changes", func() {
		ginkgo.It("marks updates the status of the istio resource", func() {
			ginkgo.By("setting the Ready condition status to true when both are ready", func() {
				istiodDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				istiodDeployment.Status.Replicas = 1
				istiodDeployment.Status.ReadyReplicas = 1
				err = k8sClient.Status().Update(ctx, istiodDeployment)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				cniDaemonSet := &appsv1.DaemonSet{}
				err = k8sClient.Get(ctx, cniObjectKey, cniDaemonSet)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				cniDaemonSet.Status.CurrentNumberScheduled = 3
				cniDaemonSet.Status.NumberReady = 3
				err = k8sClient.Status().Update(ctx, cniDaemonSet)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				gomega.Eventually(func() metav1.ConditionStatus {
					err := k8sClient.Get(ctx, istioObjectKey, istio)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					return istio.Status.GetCondition(v1.ConditionTypeReady).Status
				}, time.Minute, time.Second).Should(gomega.Equal(metav1.ConditionTrue))
			})

			ginkgo.By("setting the Ready condition status to false when istiod isn't ready", func() {
				istiodDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				istiodDeployment.Status.ReadyReplicas = 0
				err = k8sClient.Status().Update(ctx, istiodDeployment)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				gomega.Eventually(func() metav1.ConditionStatus {
					err := k8sClient.Get(ctx, istioObjectKey, istio)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					return istio.Status.GetCondition(v1.ConditionTypeReady).Status
				}, time.Minute, time.Second).Should(gomega.Equal(metav1.ConditionFalse))
			})
		})
	})

	ginkgo.When("an owned namespaced resource is deleted", func() {
		ginkgo.It("recreates the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istiod",
					Namespace: istioNamespace,
				},
			}
			err := k8sClient.Delete(ctx, istiodDeployment, client.PropagationPolicy(metav1.DeletePropagationForeground))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func() error {
				return k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
			}, time.Minute, time.Second).Should(gomega.Succeed())

			gomega.Expect(istiodDeployment.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal(pilotImage))
			gomega.Expect(istiodDeployment.ObjectMeta.OwnerReferences).To(gomega.ContainElement(expectedOwnerReference(istio)))
		})
	})

	ginkgo.When("an owned cluster-scoped resource is deleted", func() {
		ginkgo.It("recreates the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: webhookObjectKey.Name,
				},
			}
			err := k8sClient.Delete(ctx, webhook, client.PropagationPolicy(metav1.DeletePropagationForeground))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func() error {
				err := k8sClient.Get(ctx, webhookObjectKey, webhook)
				return err
			}, time.Minute, time.Second).Should(gomega.Succeed())
		})
	})

	ginkgo.When("an owned namespaced resource is modified", func() {
		ginkgo.It("reverts the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			originalImage := istiodDeployment.Spec.Template.Spec.Containers[0].Image
			istiodDeployment.Spec.Template.Spec.Containers[0].Image = "user-supplied-image"
			err = k8sClient.Update(ctx, istiodDeployment)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func() string {
				err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				return istiodDeployment.Spec.Template.Spec.Containers[0].Image
			}, time.Minute, time.Second).Should(gomega.Equal(originalImage))
		})
	})

	ginkgo.When("an owned cluster-scoped resource is modified", func() {
		ginkgo.It("reverts the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{}
			err := k8sClient.Get(ctx, webhookObjectKey, webhook)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			origWebhooks := webhook.Webhooks
			webhook.Webhooks = []admissionv1.MutatingWebhook{}
			err = k8sClient.Update(ctx, webhook)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func() []admissionv1.MutatingWebhook {
				err := k8sClient.Get(ctx, webhookObjectKey, webhook)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				return webhook.Webhooks
			}, time.Minute, time.Second).Should(gomega.Equal(origWebhooks))
		})
	})
})

func expectedOwnerReference(istio *v1.Istio) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioKind,
		Name:               istio.Name,
		UID:                istio.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}
}

func TestDeriveState(t *testing.T) {
	testCases := []struct {
		name                string
		reconciledCondition v1.IstioCondition
		readyCondition      v1.IstioCondition
		expectedState       v1.IstioConditionReason
	}{
		{
			name:                "healthy",
			reconciledCondition: newCondition(v1.ConditionTypeReconciled, true, ""),
			readyCondition:      newCondition(v1.ConditionTypeReady, true, ""),
			expectedState:       v1.ConditionReasonHealthy,
		},
		{
			name:                "not reconciled",
			reconciledCondition: newCondition(v1.ConditionTypeReconciled, false, v1.ConditionReasonReconcileError),
			readyCondition:      newCondition(v1.ConditionTypeReady, true, ""),
			expectedState:       v1.ConditionReasonReconcileError,
		},
		{
			name:                "not ready",
			reconciledCondition: newCondition(v1.ConditionTypeReconciled, true, ""),
			readyCondition:      newCondition(v1.ConditionTypeReady, false, v1.ConditionReasonIstiodNotReady),
			expectedState:       v1.ConditionReasonIstiodNotReady,
		},
		{
			name:                "not reconciled nor ready",
			reconciledCondition: newCondition(v1.ConditionTypeReconciled, false, v1.ConditionReasonReconcileError),
			readyCondition:      newCondition(v1.ConditionTypeReady, false, v1.ConditionReasonIstiodNotReady),
			expectedState:       v1.ConditionReasonReconcileError, // reconcile reason takes precedence over ready reason
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

func newCondition(conditionType v1.IstioConditionType, status bool, reason v1.IstioConditionReason) v1.IstioCondition {
	st := metav1.ConditionFalse
	if status {
		st = metav1.ConditionTrue
	}
	return v1.IstioCondition{
		Type:   conditionType,
		Status: st,
		Reason: reason,
	}
}

func TestApplyProfile(t *testing.T) {
	const version = "my-version"
	resourceDir := t.TempDir()
	profilesDir := path.Join(resourceDir, version, "profiles")
	Must(t, os.MkdirAll(profilesDir, 0o755))

	writeProfileFile := func(t *testing.T, path string, values ...string) {
		yaml := `
apiVersion: operator.istio.io/v1alpha1
kind: Istio
spec:
  values:`
		for i, val := range values {
			yaml += fmt.Sprintf(`
    value%d: %s`, i+1, val)
		}
		Must(t, os.WriteFile(path, []byte(yaml), 0o644))
	}

	writeProfileFile(t, path.Join(resourceDir, version, "profiles", "default.yaml"), "1-from-default", "2-from-default")
	writeProfileFile(t, path.Join(resourceDir, version, "profiles", "custom.yaml"), "1-from-custom")
	writeProfileFile(t, path.Join(resourceDir, version, "not-in-profiles-dir.yaml"), "should-not-be-accessible")

	tests := []struct {
		name       string
		inputSpec  v1.IstioSpec
		expectSpec v1.IstioSpec
		expectErr  bool
	}{
		{
			name: "no profile",
			inputSpec: v1.IstioSpec{
				Version: version,
				// no profile is specified; default profile should be used
			},
			expectSpec: v1.IstioSpec{
				Version: version,
				Values:  []byte(`{"value1":"1-from-default","value2":"2-from-default"}`),
			},
		},
		{
			name: "custom profile",
			inputSpec: v1.IstioSpec{
				Version: version,
				Profile: "custom", // both custom and default profile should be used (with custom taking precedence)
			},
			expectSpec: v1.IstioSpec{
				Profile: "custom",
				Version: version,
				Values:  []byte(`{"value1":"1-from-custom","value2":"2-from-default"}`),
			},
		},
		{
			name: "profile not found",
			inputSpec: v1.IstioSpec{
				Version: version,
				Profile: "invalid",
			},
			expectErr: true,
		},
		{
			name: "path-traversal-attack",
			inputSpec: v1.IstioSpec{
				Version: version,
				Profile: "../not-in-profiles-dir",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "istio-system"},
				Spec:       tt.inputSpec,
			}

			expected := &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "istio-system"},
				Spec:       tt.expectSpec,
			}

			err := applyProfile(actual, resourceDir)
			if (err != nil) != tt.expectErr {
				t.Errorf("applyProfile() error = %v, expectErr %v", err, tt.expectErr)
			}

			if err == nil {
				if diff := cmp.Diff(expected, actual); diff != "" {
					t.Errorf("profile wasn't applied properly; diff (-expected, +actual):\n%v", diff)
				}
			}
		})
	}
}

func TestMergeValues(t *testing.T) {
	testCases := []struct {
		name                  string
		main, profile, expect map[string]interface{}
	}{
		{
			name:    "both empty",
			main:    make(map[string]interface{}),
			profile: make(map[string]interface{}),
			expect:  make(map[string]interface{}),
		},
		{
			name:    "nil main",
			main:    nil,
			profile: map[string]interface{}{"key1": 42, "key2": "value"},
			expect:  map[string]interface{}{"key1": 42, "key2": "value"},
		},
		{
			name:    "nil profile",
			main:    map[string]interface{}{"key1": 42, "key2": "value"},
			profile: nil,
			expect:  map[string]interface{}{"key1": 42, "key2": "value"},
		},
		{
			name: "adds toplevel keys",
			main: map[string]interface{}{
				"key1": "from main",
			},
			profile: map[string]interface{}{
				"key2": "from profile",
			},
			expect: map[string]interface{}{
				"key1": "from main",
				"key2": "from profile",
			},
		},
		{
			name: "adds nested keys",
			main: map[string]interface{}{
				"key1": map[string]interface{}{
					"nested1": "from main",
				},
			},
			profile: map[string]interface{}{
				"key1": map[string]interface{}{
					"nested2": "from profile",
				},
			},
			expect: map[string]interface{}{
				"key1": map[string]interface{}{
					"nested1": "from main",
					"nested2": "from profile",
				},
			},
		},
		{
			name: "doesn't overwrite",
			main: map[string]interface{}{
				"key1": "from main",
				"key2": map[string]interface{}{
					"nested1": "from main",
				},
			},
			profile: map[string]interface{}{
				"key1": "from profile",
				"key2": map[string]interface{}{
					"nested1": "from profile",
				},
			},
			expect: map[string]interface{}{
				"key1": "from main",
				"key2": map[string]interface{}{
					"nested1": "from main",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mergeValues(tc.main, tc.profile)
			if diff := cmp.Diff(tc.expect, result); diff != "" {
				t.Errorf("unexpected merge result; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
