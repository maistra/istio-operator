package webhooks

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"
	arbeta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/webhookca"
)

// XXX: this entire file can be removed once ValidatingWebhookConfiguration and
// Service definitions are moved into the operator's CSV file.

var webhookFailurePolicy = arbeta1.Fail

const (
	webhookSecretName  = "maistra-operator-serving-cert"
	webhookServiceName = "maistra-admission-controller"
)

func createWebhookResources(mgr manager.Manager, log logr.Logger, operatorNamespace string) error {
	cl, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		return pkgerrors.Wrap(err, "error creating k8s client")
	}

	log.Info("Creating Maistra webhook Service")
	if err := cl.Create(context.TODO(), newWebhookService(operatorNamespace)); err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("Maistra webhook Service already exists")
		} else {
			return pkgerrors.Wrap(err, "error creating Maistra webhook Service")
		}
	}

	log.Info("Creating Maistra ValidatingWebhookConfiguration")
	validatingWebhookConfiguration := newValidatingWebhookConfiguration(operatorNamespace)
	if err := cl.Create(context.TODO(), validatingWebhookConfiguration); err != nil {
		if errors.IsAlreadyExists(err) {
			// the cache is not available until the manager is started, so webhook update needs to be done during startup.
			log.Info("Updating existing Maistra ValidatingWebhookConfiguration")
			existing := &arbeta1.ValidatingWebhookConfiguration{}
			if err := cl.Get(context.TODO(), types.NamespacedName{Name: validatingWebhookConfiguration.GetName()}, existing); err != nil {
				return pkgerrors.Wrap(err, "error retrieving existing Maistra ValidatingWebhookConfiguration")
			}
			validatingWebhookConfiguration.SetResourceVersion(existing.GetResourceVersion())
			if err := cl.Update(context.TODO(), validatingWebhookConfiguration); err != nil {
				return pkgerrors.Wrap(err, "error updating existing Maistra ValidatingWebhookConfiguration")
			}
		} else {
			return err
		}
	}

	log.Info("Registering Maistra ValidatingWebhookConfiguration with CABundle reconciler")
	if err := webhookca.WebhookCABundleManagerInstance.ManageWebhookCABundle(
		validatingWebhookConfiguration,
		types.NamespacedName{
			Namespace: operatorNamespace,
			Name:      webhookSecretName,
		},
		common.ServiceCARootCertKey); err != nil {
		return err
	}

	log.Info("Creating Maistra MutatingWebhookConfiguration")
	mutatingWebhookConfiguration := newMutatingWebhookConfiguration(operatorNamespace)
	if err := cl.Create(context.TODO(), mutatingWebhookConfiguration); err != nil {
		if errors.IsAlreadyExists(err) {
			// the cache is not available until the manager is started, so webhook update needs to be done during startup.
			log.Info("Updating existing Maistra MutatingWebhookConfiguration")
			existing := &arbeta1.MutatingWebhookConfiguration{}
			if err := cl.Get(context.TODO(), types.NamespacedName{Name: mutatingWebhookConfiguration.GetName()}, existing); err != nil {
				return pkgerrors.Wrap(err, "error retrieving existing Maistra MutatingWebhookConfiguration")
			}
			mutatingWebhookConfiguration.SetResourceVersion(existing.GetResourceVersion())
			if err := cl.Update(context.TODO(), mutatingWebhookConfiguration); err != nil {
				return pkgerrors.Wrap(err, "error updating existing Maistra MutatingWebhookConfiguration")
			}
		} else {
			return err
		}
	}

	log.Info("Registering Maistra MutatingWebhookConfiguration with CABundle reconciler")
	if err := webhookca.WebhookCABundleManagerInstance.ManageWebhookCABundle(
		mutatingWebhookConfiguration,
		types.NamespacedName{
			Namespace: operatorNamespace,
			Name:      webhookSecretName,
		},
		common.ServiceCARootCertKey); err != nil {
		return err
	}

	// wait for secret to become available to prevent the operator from bouncing
	// we don't worry about any errors here, as the worst thing that will happen
	// is that the operator might restart.
	coreclient, err := clientcorev1.NewForConfig(mgr.GetConfig())
	if err != nil {
		log.Info("error occured creating client for watching Maistra webhook Secret")
		return nil
	}
	secretwatch, err := coreclient.Secrets(operatorNamespace).Watch(metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", webhookSecretName)})
	if err != nil {
		log.Info("error occured creating watch for Maistra webhook Secret")
		return nil
	}
	func() {
		defer secretwatch.Stop()
        log.Info("Waiting for Maistra webhook Secret to become available")
		select {
		case <-secretwatch.ResultChan():
            log.Info("Maistra webhook Secret is now ready")
		case <-time.After(30 * time.Second):
			log.Info("timed out waiting for Maistra webhook Secret to become available")
		}
	}()

	return nil
}

func newWebhookService(namespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webhookServiceName,
			Namespace: namespace,
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": webhookSecretName,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"name": "istio-operator",
			},
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name:       "validation",
					Port:       443,
					TargetPort: intstr.FromInt(11999),
				},
			},
		},
	}
}

func newValidatingWebhookConfiguration(namespace string) *arbeta1.ValidatingWebhookConfiguration {
	return &arbeta1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.servicemesh-resources.maistra.io", namespace),
		},
		Webhooks: []arbeta1.ValidatingWebhook{
			arbeta1.ValidatingWebhook{
				Name: "smcp.validation.maistra.io",
				Rules: rulesFor("servicemeshcontrolplanes", arbeta1.Create, arbeta1.Update),
				FailurePolicy: &webhookFailurePolicy,
				ClientConfig: arbeta1.WebhookClientConfig{
					Service: &arbeta1.ServiceReference{
						Path:      &smcpValidatorServicePath,
						Name:      webhookServiceName,
						Namespace: namespace,
					},
				},
			},
			arbeta1.ValidatingWebhook{
				Name: "smmr.validation.maistra.io",
				Rules: rulesFor("servicemeshmemberrolls", arbeta1.Create, arbeta1.Update),
				FailurePolicy: &webhookFailurePolicy,
				ClientConfig: arbeta1.WebhookClientConfig{
					Service: &arbeta1.ServiceReference{
						Path:      &smmrValidatorServicePath,
						Name:      webhookServiceName,
						Namespace: namespace,
					},
				},
			},
			arbeta1.ValidatingWebhook{
				Name: "smm.validation.maistra.io",
				Rules: rulesFor("servicemeshmembers", arbeta1.Create, arbeta1.Update),
				FailurePolicy: &webhookFailurePolicy,
				ClientConfig: arbeta1.WebhookClientConfig{
					Service: &arbeta1.ServiceReference{
						Path:      &smmValidatorServicePath,
						Name:      webhookServiceName,
						Namespace: namespace,
					},
				},
			},
		},
	}
}

func newMutatingWebhookConfiguration(namespace string) *arbeta1.MutatingWebhookConfiguration {
	return &arbeta1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.servicemesh-resources.maistra.io", namespace),
		},
		Webhooks: []arbeta1.MutatingWebhook{
			arbeta1.MutatingWebhook{
				Name: "smcp.mutation.maistra.io",
				Rules: rulesFor("servicemeshcontrolplanes", arbeta1.Create, arbeta1.Update),
				FailurePolicy: &webhookFailurePolicy,
				ClientConfig: arbeta1.WebhookClientConfig{
					Service: &arbeta1.ServiceReference{
						Path:      &smcpMutatorServicePath,
						Name:      webhookServiceName,
						Namespace: namespace,
					},
				},
			},
			arbeta1.MutatingWebhook{
				Name: "smmr.mutation.maistra.io",
				Rules: rulesFor("servicemeshmemberrolls", arbeta1.Create, arbeta1.Update),
				FailurePolicy: &webhookFailurePolicy,
				ClientConfig: arbeta1.WebhookClientConfig{
					Service: &arbeta1.ServiceReference{
						Path:      &smmrMutatorServicePath,
						Name:      webhookServiceName,
						Namespace: namespace,
					},
				},
			},
		},
	}
}

func rulesFor(resource string, operations ...arbeta1.OperationType) []arbeta1.RuleWithOperations {
	return []arbeta1.RuleWithOperations{
		{
			Rule: arbeta1.Rule{
				APIGroups:   []string{maistrav1.SchemeGroupVersion.Group},
				APIVersions: []string{maistrav1.SchemeGroupVersion.Version},
				Resources:   []string{resource},
			},
			Operations: operations,
		},
	}
}
