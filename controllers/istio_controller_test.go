package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/common"
	"maistra.io/istio-operator/pkg/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var testConfig = common.OperatorConfig{}

const (
	istioVersion = "latest"
	pilotImage   = "maistra.io/test:latest"
)

var _ = Describe("IstioController", Ordered, func() {
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

	istio := &v1.Istio{}

	It("successfully reconciles the resource", func() {
		By("Creating the custom resource")
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
			Expect(err).NotTo(HaveOccurred())
		}

		By("Checking if the resource was successfully created")
		Eventually(func() error {
			found := &v1.Istio{}
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
			err := k8sClient.Get(ctx, istioObjectKey, istio)
			Expect(err).NotTo(HaveOccurred())
			return istio.Status.ObservedGeneration
		}, time.Minute, time.Second).Should(Equal(istio.ObjectMeta.Generation))

		By("Checking if the appliedValues are written properly")
		Eventually(func() string {
			err := k8sClient.Get(ctx, istioObjectKey, istio)
			Expect(err).NotTo(HaveOccurred())

			imageName, _, err := istio.Status.GetAppliedValues().GetString("pilot.image")
			Expect(err).NotTo(HaveOccurred())
			return imageName
		}, time.Minute, time.Second).Should(Equal(pilotImage))
	})

	When("istiod and istio-cni-node readiness changes", func() {
		It("marks updates the status of the istio resource", func() {
			By("setting the Ready condition status to true when both are ready", func() {
				istiodDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())
				istiodDeployment.Status.Replicas = 1
				istiodDeployment.Status.ReadyReplicas = 1
				err = k8sClient.Status().Update(ctx, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())

				cniDaemonSet := &appsv1.DaemonSet{}
				err = k8sClient.Get(ctx, cniObjectKey, cniDaemonSet)
				Expect(err).NotTo(HaveOccurred())
				cniDaemonSet.Status.CurrentNumberScheduled = 3
				cniDaemonSet.Status.NumberReady = 3
				err = k8sClient.Status().Update(ctx, cniDaemonSet)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() metav1.ConditionStatus {
					err := k8sClient.Get(ctx, istioObjectKey, istio)
					Expect(err).NotTo(HaveOccurred())
					return istio.Status.GetCondition(v1.ConditionTypeReady).Status
				}, time.Minute, time.Second).Should(Equal(metav1.ConditionTrue))
			})

			By("setting the Ready condition status to false when istiod isn't ready", func() {
				istiodDeployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())

				istiodDeployment.Status.ReadyReplicas = 0
				err = k8sClient.Status().Update(ctx, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() metav1.ConditionStatus {
					err := k8sClient.Get(ctx, istioObjectKey, istio)
					Expect(err).NotTo(HaveOccurred())
					return istio.Status.GetCondition(v1.ConditionTypeReady).Status
				}, time.Minute, time.Second).Should(Equal(metav1.ConditionFalse))
			})
		})
	})

	When("an owned namespaced resource is deleted", func() {
		It("recreates the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istiod",
					Namespace: istioNamespace,
				},
			}
			err := k8sClient.Delete(ctx, istiodDeployment, client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(err).NotTo(HaveOccurred())

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
			err := k8sClient.Delete(ctx, webhook, client.PropagationPolicy(metav1.DeletePropagationForeground))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				err := k8sClient.Get(ctx, webhookObjectKey, webhook)
				return err
			}, time.Minute, time.Second).Should(Succeed())
		})
	})

	When("an owned namespaced resource is modified", func() {
		It("reverts the owned resource", func() {
			istiodDeployment := &appsv1.Deployment{}
			err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
			Expect(err).NotTo(HaveOccurred())

			originalImage := istiodDeployment.Spec.Template.Spec.Containers[0].Image
			istiodDeployment.Spec.Template.Spec.Containers[0].Image = "user-supplied-image"
			err = k8sClient.Update(ctx, istiodDeployment)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() string {
				err := k8sClient.Get(ctx, deploymentObjectKey, istiodDeployment)
				Expect(err).NotTo(HaveOccurred())
				return istiodDeployment.Spec.Template.Spec.Containers[0].Image
			}, time.Minute, time.Second).Should(Equal(originalImage))
		})
	})

	When("an owned cluster-scoped resource is modified", func() {
		It("reverts the owned resource", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{}
			err := k8sClient.Get(ctx, webhookObjectKey, webhook)
			Expect(err).NotTo(HaveOccurred())

			origWebhooks := webhook.Webhooks
			webhook.Webhooks = []admissionv1.MutatingWebhook{}
			err = k8sClient.Update(ctx, webhook)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() []admissionv1.MutatingWebhook {
				err := k8sClient.Get(ctx, webhookObjectKey, webhook)
				Expect(err).NotTo(HaveOccurred())
				return webhook.Webhooks
			}, time.Minute, time.Second).Should(Equal(origWebhooks))
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

// TestGetAggregatedValues tests that the values are sourced from the following sources
// (with each source overriding the values from the previous sources):
//   - default profile(s)
//   - profile selected in Istio.spec.profile
//   - Istio.spec.values
//   - Istio.spec.rawValues
//   - other (non-value) fields in the Istio resource (e.g. the value global.istioNamespace is set from Istio.metadata.namespace)
func TestGetAggregatedValues(t *testing.T) {
	const version = "my-version"
	resourceDir := t.TempDir()
	profilesDir := path.Join(resourceDir, version, "profiles")
	Must(t, os.MkdirAll(profilesDir, 0o755))

	Must(t, os.WriteFile(path.Join(profilesDir, "default.yaml"), []byte((`
apiVersion: operator.istio.io/v1alpha1
kind: Istio
spec:
  values:
    key1: from-default-profile
    key2: from-default-profile  # this gets overridden in my-profile
    key3: from-default-profile  # this gets overridden in my-profile and values
    key4: from-default-profile  # this gets overridden in my-profile, values, and rawValues`)), 0o644))

	Must(t, os.WriteFile(path.Join(profilesDir, "my-profile.yaml"), []byte((`
apiVersion: operator.istio.io/v1alpha1
kind: Istio
spec:
  values:
    key2: overridden-in-my-profile
    key3: overridden-in-my-profile  # this gets overridden in values
    key4: overridden-in-my-profile  # this gets overridden in rawValues`)), 0o644))

	istio := v1.Istio{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-istio",
			Namespace: "my-istio-namespace",
		},
		Spec: v1.IstioSpec{
			Version: version,
			Profile: "my-profile",
			Values: toJSON(helm.HelmValues{
				"key3": "overridden-in-values",
				"key4": "overridden-in-values", // this gets overridden in rawValues
			}),
			RawValues: toJSON(helm.HelmValues{
				"key4": "overridden-in-raw-values",
			}),
		},
	}

	result, err := getAggregatedValues(istio, []string{"default"}, resourceDir)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}

	expected := helm.HelmValues{
		"key1": "from-default-profile",
		"key2": "overridden-in-my-profile",
		"key3": "overridden-in-values",
		"key4": "overridden-in-raw-values",
		"global": map[string]any{
			"istioNamespace": "my-istio-namespace", // this value is always added/overridden based on Istio.metadata.namespace
		},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Result does not match the expected HelmValues.\nExpected: %v\nActual: %v", expected, result)
	}
}

func toJSON(values helm.HelmValues) json.RawMessage {
	jsonVals, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return jsonVals
}

func TestGetValuesFromProfiles(t *testing.T) {
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
			if val != "" {
				yaml += fmt.Sprintf(`
    value%d: %s`, i+1, val)
			}
		}
		Must(t, os.WriteFile(path, []byte(yaml), 0o644))
	}

	writeProfileFile(t, path.Join(profilesDir, "default.yaml"), "1-from-default", "2-from-default")
	writeProfileFile(t, path.Join(profilesDir, "overlay.yaml"), "", "2-from-overlay")
	writeProfileFile(t, path.Join(profilesDir, "custom.yaml"), "1-from-custom")
	writeProfileFile(t, path.Join(resourceDir, version, "not-in-profiles-dir.yaml"), "should-not-be-accessible")

	tests := []struct {
		name         string
		profiles     []string
		expectValues helm.HelmValues
		expectErr    bool
	}{
		{
			name:         "nil default profiles",
			profiles:     nil,
			expectValues: helm.HelmValues{},
		},
		{
			name:     "default profile only",
			profiles: []string{"default"},
			expectValues: helm.HelmValues{
				"value1": "1-from-default",
				"value2": "2-from-default",
			},
		},
		{
			name:     "default and overlay",
			profiles: []string{"default", "overlay"},
			expectValues: helm.HelmValues{
				"value1": "1-from-default",
				"value2": "2-from-overlay",
			},
		},
		{
			name:     "default and overlay and custom",
			profiles: []string{"default", "overlay", "custom"},
			expectValues: helm.HelmValues{
				"value1": "1-from-custom",
				"value2": "2-from-overlay",
			},
		},
		{
			name:      "default profile empty",
			profiles:  []string{""},
			expectErr: true,
		},
		{
			name:      "profile not found",
			profiles:  []string{"invalid"},
			expectErr: true,
		},
		{
			name:      "path-traversal-attack",
			profiles:  []string{"../not-in-profiles-dir"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := getValuesFromProfiles(profilesDir, tt.profiles)
			if (err != nil) != tt.expectErr {
				t.Errorf("applyProfile() error = %v, expectErr %v", err, tt.expectErr)
			}

			if err == nil {
				if diff := cmp.Diff(tt.expectValues, actual); diff != "" {
					t.Errorf("profile wasn't applied properly; diff (-expected, +actual):\n%v", diff)
				}
			}
		})
	}
}

func TestMergeOverwrite(t *testing.T) {
	testCases := []struct {
		name                    string
		overrides, base, expect map[string]any
	}{
		{
			name:      "both empty",
			base:      make(map[string]any),
			overrides: make(map[string]any),
			expect:    make(map[string]any),
		},
		{
			name:      "nil overrides",
			base:      map[string]any{"key1": 42, "key2": "value"},
			overrides: nil,
			expect:    map[string]any{"key1": 42, "key2": "value"},
		},
		{
			name:      "nil base",
			base:      nil,
			overrides: map[string]any{"key1": 42, "key2": "value"},
			expect:    map[string]any{"key1": 42, "key2": "value"},
		},
		{
			name: "adds toplevel keys",
			base: map[string]any{
				"key2": "from base",
			},
			overrides: map[string]any{
				"key1": "from overrides",
			},
			expect: map[string]any{
				"key1": "from overrides",
				"key2": "from base",
			},
		},
		{
			name: "adds nested keys",
			base: map[string]any{
				"key1": map[string]any{
					"nested2": "from base",
				},
			},
			overrides: map[string]any{
				"key1": map[string]any{
					"nested1": "from overrides",
				},
			},
			expect: map[string]any{
				"key1": map[string]any{
					"nested1": "from overrides",
					"nested2": "from base",
				},
			},
		},
		{
			name: "overrides overrides base",
			base: map[string]any{
				"key1": "from base",
				"key2": map[string]any{
					"nested1": "from base",
				},
			},
			overrides: map[string]any{
				"key1": "from overrides",
				"key2": map[string]any{
					"nested1": "from overrides",
				},
			},
			expect: map[string]any{
				"key1": "from overrides",
				"key2": map[string]any{
					"nested1": "from overrides",
				},
			},
		},
		{
			name: "mismatched types",
			base: map[string]any{
				"key1": map[string]any{
					"desc": "key1 is a map in base",
				},
				"key2": "key2 is a string in base",
			},
			overrides: map[string]any{
				"key1": "key1 is a string in overrides",
				"key2": map[string]any{
					"desc": "key2 is a map in overrides",
				},
			},
			expect: map[string]any{
				"key1": "key1 is a string in overrides",
				"key2": map[string]any{
					"desc": "key2 is a map in overrides",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mergeOverwrite(tc.base, tc.overrides)
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
