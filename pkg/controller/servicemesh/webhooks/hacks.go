package webhooks

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clientapixv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pttypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/json"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/servicemesh/webhookca"
)

// XXX: this entire file can be removed once ValidatingWebhookConfiguration and
// Service definitions are moved into the operator's CSV file.

var webhookFailurePolicy = admissionv1.Fail

const (
	webhookSecretName  = "maistra-operator-serving-cert"
	webhookServiceName = "maistra-admission-controller"
)

func createWebhookResources(ctx context.Context, mgr manager.Manager, log logr.Logger, operatorNamespace string) error {
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
			existing := &admissionv1.ValidatingWebhookConfiguration{}
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
			existing := &admissionv1.MutatingWebhookConfiguration{}
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

	log.Info("Adding conversion webhook to SMCP CRD")
	if apixclient, err := clientapixv1.NewForConfig(mgr.GetConfig()); err == nil {
		if crdPatchBytes, err := json.Marshal(map[string]interface{}{
			"spec": map[string]interface{}{
				"conversion": &apixv1.CustomResourceConversion{
					Strategy: apixv1.WebhookConverter,
					Webhook: &apixv1.WebhookConversion{
						ConversionReviewVersions: []string{"v1beta1"},
						ClientConfig: &apixv1.WebhookClientConfig{
							URL: nil,
							Service: &apixv1.ServiceReference{
								Name:      webhookServiceName,
								Namespace: operatorNamespace,
								Path:      &smcpConverterServicePath,
							},
						},
					},
				},
			},
		}); err == nil {
			if smcpcrd, err := apixclient.CustomResourceDefinitions().Patch(ctx, webhookca.ServiceMeshControlPlaneCRDName,
				pttypes.MergePatchType, crdPatchBytes, metav1.PatchOptions{FieldManager: common.FinalizerName}); err == nil {
				log.Info("Registering Maistra ServiceMeshControlPlane CRD conversion webhook with CABundle reconciler")
				if err := webhookca.WebhookCABundleManagerInstance.ManageWebhookCABundle(
					smcpcrd,
					types.NamespacedName{
						Namespace: operatorNamespace,
						Name:      webhookSecretName,
					}, common.ServiceCARootCertKey); err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			return err
		}
	} else {
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
	secretwatch, err := coreclient.Secrets(operatorNamespace).Watch(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", webhookSecretName)})
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
				{
					Name:       "validation",
					Port:       443,
					TargetPort: intstr.FromInt(11999),
				},
			},
		},
	}
}

func newValidatingWebhookConfiguration(namespace string) *admissionv1.ValidatingWebhookConfiguration {
	noneSideEffects := admissionv1.SideEffectClassNone
	return &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.servicemesh-resources.maistra.io", namespace),
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name: "smcp.validation.maistra.io",
				Rules: rulesFor("servicemeshcontrolplanes",
					[]string{maistrav1.SchemeGroupVersion.Version, maistrav2.SchemeGroupVersion.Version},
					admissionv1.Create, admissionv1.Update),
				FailurePolicy:           &webhookFailurePolicy,
				SideEffects:             &noneSideEffects,
				AdmissionReviewVersions: []string{"v1beta1"},
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Path:      &smcpValidatorServicePath,
						Name:      webhookServiceName,
						Namespace: namespace,
					},
				},
			},
			{
				Name: "smmr.validation.maistra.io",
				Rules: rulesFor("servicemeshmemberrolls",
					[]string{maistrav1.SchemeGroupVersion.Version}, admissionv1.Create, admissionv1.Update),
				FailurePolicy:           &webhookFailurePolicy,
				SideEffects:             &noneSideEffects,
				AdmissionReviewVersions: []string{"v1beta1"},
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Path:      &smmrValidatorServicePath,
						Name:      webhookServiceName,
						Namespace: namespace,
					},
				},
			},
			{
				Name: "smm.validation.maistra.io",
				Rules: rulesFor("servicemeshmembers",
					[]string{maistrav1.SchemeGroupVersion.Version}, admissionv1.Create, admissionv1.Update),
				FailurePolicy:           &webhookFailurePolicy,
				SideEffects:             &noneSideEffects,
				AdmissionReviewVersions: []string{"v1beta1"},
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Path:      &smmValidatorServicePath,
						Name:      webhookServiceName,
						Namespace: namespace,
					},
				},
			},
		},
	}
}

func newMutatingWebhookConfiguration(namespace string) *admissionv1.MutatingWebhookConfiguration {
	noneOnDryRunSideEffects := admissionv1.SideEffectClassNoneOnDryRun
	return &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.servicemesh-resources.maistra.io", namespace),
		},
		Webhooks: []admissionv1.MutatingWebhook{
			{
				Name: "smcp.mutation.maistra.io",
				Rules: rulesFor("servicemeshcontrolplanes",
					[]string{maistrav1.SchemeGroupVersion.Version, maistrav2.SchemeGroupVersion.Version},
					admissionv1.Create, admissionv1.Update),
				FailurePolicy:           &webhookFailurePolicy,
				SideEffects:             &noneOnDryRunSideEffects,
				AdmissionReviewVersions: []string{"v1beta1"},
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Path:      &smcpMutatorServicePath,
						Name:      webhookServiceName,
						Namespace: namespace,
					},
				},
			},
			{
				Name: "smmr.mutation.maistra.io",
				Rules: rulesFor("servicemeshmemberrolls",
					[]string{maistrav1.SchemeGroupVersion.Version}, admissionv1.Create, admissionv1.Update),
				FailurePolicy:           &webhookFailurePolicy,
				SideEffects:             &noneOnDryRunSideEffects,
				AdmissionReviewVersions: []string{"v1beta1"},
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Path:      &smmrMutatorServicePath,
						Name:      webhookServiceName,
						Namespace: namespace,
					},
				},
			},
		},
	}
}

func rulesFor(resource string, versions []string, operations ...admissionv1.OperationType) []admissionv1.RuleWithOperations {
	return []admissionv1.RuleWithOperations{
		{
			Rule: admissionv1.Rule{
				APIGroups:   []string{maistrav1.SchemeGroupVersion.Group},
				APIVersions: versions,
				Resources:   []string{resource},
			},
			Operations: operations,
		},
	}
}
