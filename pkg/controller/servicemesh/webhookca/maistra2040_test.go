package webhookca

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
	arv1 "k8s.io/api/admissionregistration/v1"
	arv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	. "github.com/maistra/istio-operator/pkg/controller/common/test"
)

func TestMAISTRA_2040(t *testing.T) {
	const (
		testNamespace                = "test-namespace"
		galleyWebhookName            = "istio-galley-" + testNamespace
		sidecarInjectorWebhookName   = "istio-sidecar-injector-" + testNamespace
		istiodMutatingWebhookName    = "istiod-foo-" + testNamespace
		istiodValidatingWebhookName  = "istiod-foo-" + testNamespace
		validatingGeneratedCert      = "generated-validating"
		mutatingGeneratedCert        = "generated-mutating"
		istiodGeneratedCert          = "custom-validating"
		istiodCustomCert             = "custom-cert"
		v11GalleySecretName          = "istio.istio-galley-service-account"
		v11SidecarInjectorSecretName = "istio.istio-sidecar-injector-service-account"
		v20SelfSignedSecretName      = "istio-ca-secret"
		v20PrivateKeySecretName      = "cacerts"
	)

	var (
		galleyWebhook = &arv1beta1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: galleyWebhookName,
			},
			Webhooks: []arv1beta1.ValidatingWebhook{
				{
					Name: "pilot.validation.istio.io",
					ClientConfig: arv1beta1.WebhookClientConfig{
						Service: &arv1beta1.ServiceReference{
							Namespace: testNamespace,
							Name:      "istio-galley",
						},
					},
				},
				{
					Name: "mixer.validation.istio.io",
					ClientConfig: arv1beta1.WebhookClientConfig{
						Service: &arv1beta1.ServiceReference{
							Namespace: testNamespace,
							Name:      "istio-galley",
						},
					},
				},
			},
		}
		galleySecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v11GalleySecretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"root-cert.pem": []byte(validatingGeneratedCert),
			},
		}
		sidecarInjectorWebhook = &arv1beta1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: sidecarInjectorWebhookName,
			},
			Webhooks: []arv1beta1.MutatingWebhook{
				{
					Name: "sidecar-injector.istio.io",
					ClientConfig: arv1beta1.WebhookClientConfig{
						Service: &arv1beta1.ServiceReference{
							Namespace: testNamespace,
							Name:      "istio-sidecar-injector",
						},
					},
				},
			},
		}
		sidecarInjectorSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v11SidecarInjectorSecretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"root-cert.pem": []byte(mutatingGeneratedCert),
			},
		}
		istiodMutatingWebhook = &arv1beta1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: istiodMutatingWebhookName,
			},
			Webhooks: []arv1beta1.MutatingWebhook{
				{
					Name: "sidecar-injector.istio.io",
					ClientConfig: arv1beta1.WebhookClientConfig{
						Service: &arv1beta1.ServiceReference{
							Namespace: testNamespace,
							Name:      "istiod-foo",
						},
					},
				},
			},
		}
		istiodValidatingWebhook = &arv1beta1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: istiodValidatingWebhookName,
			},
			Webhooks: []arv1beta1.ValidatingWebhook{
				{
					Name: "validation.istio.io",
					ClientConfig: arv1beta1.WebhookClientConfig{
						Service: &arv1beta1.ServiceReference{
							Namespace: testNamespace,
							Name:      "istiod-foo",
						},
					},
				},
			},
		}
		istiodSelfSignedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v20SelfSignedSecretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"ca-cert.pem": []byte(istiodGeneratedCert),
			},
		}
		istiodPrivateKeySecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v20PrivateKeySecretName,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"ca-cert.pem": []byte(istiodCustomCert),
			},
		}
	)
	var testCases = []struct {
		name        string
		description string
		resources   []runtime.Object
		events      []ControllerTestEvent
	}{
		{
			name:        "default.v1.1",
			description: "testing webhook controller using a default installation",
			events: []ControllerTestEvent{
				{
					Name: "create-galley-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), galleyWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11GalleySecretName).In(testNamespace).IsSeen(),
					),
					Timeout: 2 * time.Second,
				},
				{
					Name: "create-galley-secret",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), galleySecret.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("validatingwebhookconfigurations").Named(galleyWebhookName).IsSeen(),
						Verify("get").On("secrets").Named(v11GalleySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(galleyWebhookName).Passes(verifyCABundle(validatingGeneratedCert)),
					),
					Timeout: 2 * time.Second,
				},
				{
					Name: "create-injector-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), sidecarInjectorWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11SidecarInjectorSecretName).In(testNamespace).IsSeen(),
					),
					Timeout: 2 * time.Second,
				},
				{
					Name: "create-injector-secret",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), sidecarInjectorSecret.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("mutatingwebhookconfigurations").Named(sidecarInjectorWebhookName).IsSeen(),
						Verify("get").On("secrets").Named(v11SidecarInjectorSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(sidecarInjectorWebhookName).Passes(verifyCABundle(mutatingGeneratedCert)),
					),
					Timeout: 2 * time.Second,
				},
			},
		},
		{
			name:        "default.v2.0",
			description: "testing webhook controller using a default installation",
			events: []ControllerTestEvent{
				{
					Name: "create-mutating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), istiodMutatingWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
					),
					Timeout: 2 * time.Second,
				},
				{
					Name: "create-validating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), istiodValidatingWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
					),
					Timeout: 2 * time.Second,
				},
				{
					Name: "create-istio-ca-secret",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), istiodSelfSignedSecret.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("mutatingwebhookconfigurations").Named(istiodMutatingWebhookName).IsSeen(),
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(istiodMutatingWebhookName).Passes(verifyCABundle(istiodGeneratedCert)),
						Verify("get").On("validatingwebhookconfigurations").Named(istiodValidatingWebhookName).IsSeen(),
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(istiodValidatingWebhookName).Passes(verifyCABundle(istiodGeneratedCert)),
					),
					Timeout: 2 * time.Second,
				},
			},
		},
		{
			name:        "preexisting_secret.v1.1",
			description: "testing webhook controller with a pre-existing secret",
			resources: []runtime.Object{
				galleySecret.DeepCopy(),
				sidecarInjectorSecret.DeepCopy(),
			},
			events: []ControllerTestEvent{
				{
					Name: "create-galley-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), galleyWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11GalleySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(galleyWebhookName).Passes(verifyCABundle(validatingGeneratedCert)),
					),
					Timeout: 2 * time.Second,
				},
				{
					Name: "create-injector-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), sidecarInjectorWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11SidecarInjectorSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(sidecarInjectorWebhookName).Passes(verifyCABundle(mutatingGeneratedCert)),
					),
					Timeout: 2 * time.Second,
				},
			},
		},
		{
			name:        "preexisting_secret.v2.0.self-signed",
			description: "testing webhook controller with a pre-existing self-signed secret",
			resources: []runtime.Object{
				istiodSelfSignedSecret.DeepCopy(),
			},
			events: []ControllerTestEvent{
				{
					Name: "create-mutating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), istiodMutatingWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(istiodMutatingWebhookName).Passes(verifyCABundle(istiodGeneratedCert)),
					),
					Timeout: 2 * time.Second,
				},
				{
					Name: "create-validating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), istiodValidatingWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(istiodValidatingWebhookName).Passes(verifyCABundle(istiodGeneratedCert)),
					),
					Timeout: 2 * time.Second,
				},
			},
		},
		{
			name:        "preexisting_secret.v2.0.private-key",
			description: "testing webhook controller with a pre-existing private-key secret",
			resources: []runtime.Object{
				istiodPrivateKeySecret.DeepCopy(),
			},
			events: []ControllerTestEvent{
				{
					Name: "create-mutating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), istiodMutatingWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(istiodMutatingWebhookName).Passes(verifyCABundle(istiodCustomCert)),
					),
					Timeout: 2 * time.Second,
				},
				{
					Name: "create-validating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), istiodValidatingWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(istiodValidatingWebhookName).Passes(verifyCABundle(istiodCustomCert)),
					),
					Timeout: 2 * time.Second,
				},
			},
		},
		{
			name:        "preexisting_secret.v2.0.both",
			description: "testing webhook controller with a pre-existing self-signed and private-key secret",
			resources: []runtime.Object{
				istiodSelfSignedSecret.DeepCopy(),
				istiodPrivateKeySecret.DeepCopy(),
			},
			events: []ControllerTestEvent{
				{
					Name: "create-mutating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), istiodMutatingWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(istiodMutatingWebhookName).Passes(verifyCABundle(istiodCustomCert)),
					),
					Timeout: 2 * time.Second,
				},
				{
					Name: "create-validating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), istiodValidatingWebhook.DeepCopy())
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(istiodValidatingWebhookName).Passes(verifyCABundle(istiodCustomCert)),
					),
					Timeout: 2 * time.Second,
				},
			},
		},
	}

	if testing.Verbose() {
		logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(os.Stderr), zap.Level(zapcore.Level(-5))))
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
            t.Log(tc.description)
            // this is global singleton, so we need to reset it each time
            WebhookCABundleManagerInstance = newWebhookCABundleManager()
			RunControllerTestCase(t, ControllerTestCase{
				Name:            tc.name,
				AddControllers:  []AddControllerFunc{Add},
				StorageVersions: []schema.GroupVersion{arv1.SchemeGroupVersion},
				Resources:       append([]runtime.Object{&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}}, tc.resources...),
				Events:          tc.events,
			})
		})
	}
}

func verifyCABundle(caBundle string) func(action clienttesting.Action) error {
	return func(action clienttesting.Action) error {
		updateAction := action.(clienttesting.UpdateAction)
		obj := updateAction.GetObject()
		switch webhook := obj.(type) {
		case *arv1.MutatingWebhookConfiguration:
			for _, webhook := range webhook.Webhooks {
				if string(webhook.ClientConfig.CABundle) != caBundle {
					return fmt.Errorf("unexpected CABundle: expected %s, got %s", caBundle, string(webhook.ClientConfig.CABundle))
				}
			}
		case *arv1.ValidatingWebhookConfiguration:
			for _, webhook := range webhook.Webhooks {
				if string(webhook.ClientConfig.CABundle) != caBundle {
					return fmt.Errorf("unexpected CABundle: expected %s, got %s", caBundle, string(webhook.ClientConfig.CABundle))
				}
			}
		default:
			return fmt.Errorf("unexpected webhook type: expected v1beta1, got %T", obj)
		}
		return nil
	}
}
