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
		testNamespace                     = "test-namespace-1"
		test2Namespace                    = "test-namespace-2"
		galleyWebhookNamePrefix           = "istio-galley"
		sidecarInjectorWebhookNamePrefix  = "istio-sidecar-injector"
		istiodMutatingWebhookNamePrefix   = "istiod-foo"
		istiodValidatingWebhookNamePrefix = "istiod-foo"
		v11GalleySecretName               = "istio.istio-galley-service-account"
		v11SidecarInjectorSecretName      = "istio.istio-sidecar-injector-service-account"
		v20SelfSignedSecretName           = "istio-ca-secret"
		v20PrivateKeySecretName           = "cacerts"
	)

	var eventTimeout = 10 * time.Second
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
						return mgr.GetClient().Create(context.TODO(), create1xValidatingWebhook(galleyWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11GalleySecretName).In(testNamespace).IsSeen(),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-galley-secret",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), createWebhookSecret(galleySecretName, testNamespace, "root-cert.pem"))
					},
					Verifier: VerifyActions(
						Verify("get").On("validatingwebhookconfigurations").Named(webhookName(galleyWebhookNamePrefix, testNamespace)).IsSeen(),
						Verify("get").On("secrets").Named(v11GalleySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(webhookName(galleyWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(galleySecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-injector-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create1xMutatingWebhook(sidecarInjectorWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11SidecarInjectorSecretName).In(testNamespace).IsSeen(),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-injector-secret",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), createWebhookSecret(sidecarInjectorSecretName, testNamespace, "root-cert.pem"))
					},
					Verifier: VerifyActions(
						Verify("get").On("mutatingwebhookconfigurations").Named(webhookName(sidecarInjectorWebhookNamePrefix, testNamespace)).IsSeen(),
						Verify("get").On("secrets").Named(v11SidecarInjectorSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(webhookName(sidecarInjectorWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(sidecarInjectorSecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
			},
		},
		{
			name:        "default.v2.0.mutating",
			description: "testing mutating webhook update by webhook controller using a default installation",
			events: []ControllerTestEvent{
				{
					Name: "create-mutating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create2xMutatingWebhook(istiodMutatingWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-istio-ca-secret",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), createWebhookSecret(v20SelfSignedSecretName, testNamespace, "ca-cert.pem"))
					},
					Verifier: VerifyActions(
						Verify("get").On("mutatingwebhookconfigurations").Named(webhookName(istiodMutatingWebhookNamePrefix, testNamespace)).IsSeen(),
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(webhookName(istiodMutatingWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(v20SelfSignedSecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
			},
		},
		{
			name:        "default.v2.0.validating",
			description: "testing validating webhook update by webhook controller using a default installation",
			events: []ControllerTestEvent{
				{
					Name: "create-validating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create2xValidatingWebhook(istiodValidatingWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-istio-ca-secret",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), createWebhookSecret(v20SelfSignedSecretName, testNamespace, "ca-cert.pem"))
					},
					Verifier: VerifyActions(
						Verify("get").On("validatingwebhookconfigurations").Named(webhookName(istiodValidatingWebhookNamePrefix, testNamespace)).IsSeen(),
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(webhookName(istiodValidatingWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(v20SelfSignedSecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
			},
		},
		{
			name:        "preexisting_secret.v1.1",
			description: "testing webhook controller with a pre-existing secret",
			resources: []runtime.Object{
				createWebhookSecret(galleySecretName, testNamespace, "root-cert.pem"),
				createWebhookSecret(sidecarInjectorSecretName, testNamespace, "root-cert.pem"),
			},
			events: []ControllerTestEvent{
				{
					Name: "create-galley-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create1xValidatingWebhook(galleyWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11GalleySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(webhookName(galleyWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(galleySecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-injector-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create1xMutatingWebhook(sidecarInjectorWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11SidecarInjectorSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(webhookName(sidecarInjectorWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(sidecarInjectorSecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
			},
		},
		{
			name:        "preexisting_secret.v2.0.self-signed",
			description: "testing webhook controller with a pre-existing self-signed secret",
			resources: []runtime.Object{
				createWebhookSecret(v20SelfSignedSecretName, testNamespace, "ca-cert.pem"),
			},
			events: []ControllerTestEvent{
				{
					Name: "create-mutating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create2xMutatingWebhook(istiodMutatingWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(webhookName(istiodMutatingWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(v20SelfSignedSecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-validating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create2xValidatingWebhook(istiodValidatingWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("get").On("secrets").Named(v20SelfSignedSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(webhookName(istiodValidatingWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(v20SelfSignedSecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
			},
		},
		{
			name:        "preexisting_secret.v2.0.private-key",
			description: "testing webhook controller with a pre-existing private-key secret",
			resources: []runtime.Object{
				createWebhookSecret(v20PrivateKeySecretName, testNamespace, "ca-cert.pem"),
			},
			events: []ControllerTestEvent{
				{
					Name: "create-mutating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create2xMutatingWebhook(istiodMutatingWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(webhookName(istiodMutatingWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(v20PrivateKeySecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-validating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create2xValidatingWebhook(istiodValidatingWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(webhookName(istiodValidatingWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(v20PrivateKeySecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
			},
		},
		{
			name:        "preexisting_secret.v2.0.both",
			description: "testing webhook controller with a pre-existing self-signed and private-key secret",
			resources: []runtime.Object{
				createWebhookSecret(v20SelfSignedSecretName, testNamespace, "ca-cert.pem"),
				createWebhookSecret(v20PrivateKeySecretName, testNamespace, "ca-cert.pem"),
			},
			events: []ControllerTestEvent{
				{
					Name: "create-mutating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create2xMutatingWebhook(istiodMutatingWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(webhookName(istiodMutatingWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(v20PrivateKeySecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-validating-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create2xValidatingWebhook(istiodValidatingWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v20PrivateKeySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(webhookName(istiodValidatingWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(v20PrivateKeySecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
			},
		},
		{
			name:        "maistra-2053",
			description: "multiple installs should use the correct secret for their install",
			resources:   []runtime.Object{&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: test2Namespace}}},
			events: []ControllerTestEvent{
				{
					Name: "create-galley-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create1xValidatingWebhook(galleyWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11GalleySecretName).In(testNamespace).IsSeen(),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-galley-secret",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), createWebhookSecret(galleySecretName, testNamespace, "root-cert.pem"))
					},
					Verifier: VerifyActions(
						Verify("get").On("validatingwebhookconfigurations").Named(webhookName(galleyWebhookNamePrefix, testNamespace)).IsSeen(),
						Verify("get").On("secrets").Named(v11GalleySecretName).In(testNamespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(webhookName(galleyWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(galleySecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-injector-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create1xMutatingWebhook(sidecarInjectorWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11SidecarInjectorSecretName).In(testNamespace).IsSeen(),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-injector-secret",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), createWebhookSecret(sidecarInjectorSecretName, testNamespace, "root-cert.pem"))
					},
					Verifier: VerifyActions(
						Verify("get").On("mutatingwebhookconfigurations").Named(webhookName(sidecarInjectorWebhookNamePrefix, testNamespace)).IsSeen(),
						Verify("get").On("secrets").Named(v11SidecarInjectorSecretName).In(testNamespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(webhookName(sidecarInjectorWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(sidecarInjectorSecretName, testNamespace))),
					),
					Timeout: eventTimeout,
				},
				{
					Name: "create-galley-webhook-ns2",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create1xValidatingWebhook(galleyWebhookNamePrefix, test2Namespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11GalleySecretName).In(test2Namespace).IsSeen(),
					),
					Assertions: []ActionAssertion{
						Assert("get").On("secrets").Named(v11GalleySecretName).In(testNamespace).IsNotSeen(),
						Assert("update").On("validatingwebhookconfigurations").Named(webhookName(galleyWebhookNamePrefix, test2Namespace)).IsNotSeen(),
					},
					Timeout: eventTimeout,
				},
				{
					Name: "create-galley-secret-ns2",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), createWebhookSecret(galleySecretName, test2Namespace, "root-cert.pem"))
					},
					Verifier: VerifyActions(
						Verify("get").On("validatingwebhookconfigurations").Named(webhookName(galleyWebhookNamePrefix, test2Namespace)).IsSeen(),
						Verify("get").On("secrets").Named(v11GalleySecretName).In(test2Namespace).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(webhookName(galleyWebhookNamePrefix, test2Namespace)).Passes(verifyCABundle(certForSecret(galleySecretName, test2Namespace))),
					),
					Assertions: []ActionAssertion{
						Assert("get").On("secrets").Named(v11GalleySecretName).In(testNamespace).IsNotSeen(),
					},
					Timeout: eventTimeout,
				},
				{
					Name: "create-injector-webhook-ns2",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), create1xMutatingWebhook(sidecarInjectorWebhookNamePrefix, test2Namespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("secrets").Named(v11SidecarInjectorSecretName).In(test2Namespace).IsSeen(),
					),
					Assertions: []ActionAssertion{
						Assert("get").On("secrets").Named(v11SidecarInjectorSecretName).In(testNamespace).IsNotSeen(),
						Assert("update").On("mutatingwebhookconfigurations").Named(webhookName(sidecarInjectorWebhookNamePrefix, test2Namespace)).IsNotSeen(),
					},
					Timeout: eventTimeout,
				},
				{
					Name: "create-injector-secret-ns2",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Create(context.TODO(), createWebhookSecret(sidecarInjectorSecretName, test2Namespace, "root-cert.pem"))
					},
					Verifier: VerifyActions(
						Verify("get").On("mutatingwebhookconfigurations").Named(webhookName(sidecarInjectorWebhookNamePrefix, test2Namespace)).IsSeen(),
						Verify("get").On("secrets").Named(v11SidecarInjectorSecretName).In(test2Namespace).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(webhookName(sidecarInjectorWebhookNamePrefix, test2Namespace)).Passes(verifyCABundle(certForSecret(sidecarInjectorSecretName, test2Namespace))),
					),
					Assertions: []ActionAssertion{
						Assert("get").On("secrets").Named(v11SidecarInjectorSecretName).In(testNamespace).IsNotSeen(),
					},
					Timeout: eventTimeout,
				},
				{
					Name: "update-galley-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Update(context.TODO(), create1xValidatingWebhook(galleyWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("validatingwebhookconfigurations").Named(webhookName(galleyWebhookNamePrefix, testNamespace)).IsSeen(),
						Verify("update").On("validatingwebhookconfigurations").Named(webhookName(galleyWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(galleySecretName, testNamespace))),
					),
					Assertions: []ActionAssertion{
						Assert("get").On("secrets").Named(v11GalleySecretName).In(test2Namespace).IsNotSeen(),
					},
					Timeout: eventTimeout,
				},
				{
					Name: "update-injector-webhook",
					Execute: func(mgr *FakeManager, tracker *EnhancedTracker) error {
						return mgr.GetClient().Update(context.TODO(), create1xMutatingWebhook(sidecarInjectorWebhookNamePrefix, testNamespace))
					},
					Verifier: VerifyActions(
						Verify("get").On("mutatingwebhookconfigurations").Named(webhookName(sidecarInjectorWebhookNamePrefix, testNamespace)).IsSeen(),
						Verify("update").On("mutatingwebhookconfigurations").Named(webhookName(sidecarInjectorWebhookNamePrefix, testNamespace)).Passes(verifyCABundle(certForSecret(sidecarInjectorSecretName, testNamespace))),
					),
					Assertions: []ActionAssertion{
						Assert("get").On("secrets").Named(v11SidecarInjectorSecretName).In(test2Namespace).IsNotSeen(),
					},
					Timeout: eventTimeout,
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

func webhookName(prefix, namespace string) string {
	return fmt.Sprintf("%s-%s", prefix, namespace)
}

func create1xValidatingWebhook(prefix, namespace string) *arv1beta1.ValidatingWebhookConfiguration {
	return &arv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookName(prefix, namespace),
		},
		Webhooks: []arv1beta1.ValidatingWebhook{
			{
				Name: "pilot.validation.istio.io",
				ClientConfig: arv1beta1.WebhookClientConfig{
					Service: &arv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      "istio-galley",
					},
				},
			},
			{
				Name: "mixer.validation.istio.io",
				ClientConfig: arv1beta1.WebhookClientConfig{
					Service: &arv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      "istio-galley",
					},
				},
			},
		},
	}
}

func create1xMutatingWebhook(prefix, namespace string) *arv1beta1.MutatingWebhookConfiguration {
	return &arv1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookName(prefix, namespace),
		},
		Webhooks: []arv1beta1.MutatingWebhook{
			{
				Name: "sidecar-injector.istio.io",
				ClientConfig: arv1beta1.WebhookClientConfig{
					Service: &arv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      "istio-sidecar-injector",
					},
				},
			},
		},
	}
}

func create2xValidatingWebhook(prefix, namespace string) *arv1beta1.ValidatingWebhookConfiguration {
	return &arv1beta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookName(prefix, namespace),
		},
		Webhooks: []arv1beta1.ValidatingWebhook{
			{
				Name: "validation.istio.io",
				ClientConfig: arv1beta1.WebhookClientConfig{
					Service: &arv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      "istiod-foo",
					},
				},
			},
		},
	}
}

func create2xMutatingWebhook(prefix, namespace string) *arv1beta1.MutatingWebhookConfiguration {
	return &arv1beta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookName(prefix, namespace),
		},
		Webhooks: []arv1beta1.MutatingWebhook{
			{
				Name: "sidecar-injector.istio.io",
				ClientConfig: arv1beta1.WebhookClientConfig{
					Service: &arv1beta1.ServiceReference{
						Namespace: namespace,
						Name:      "istiod-foo",
					},
				},
			},
		},
	}
}

func createWebhookSecret(name, namespace, key string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			key: []byte(certForSecret(name, namespace)),
		},
	}
}

func certForSecret(name, namespace string) string {
	return fmt.Sprintf("%s-%s", name, namespace)
}
