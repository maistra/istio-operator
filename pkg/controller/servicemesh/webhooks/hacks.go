package webhooks

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	webhookSecretName    = "maistra-operator-serving-cert"
	webhookConfigMapName = "maistra-operator-cabundle"
	webhookServiceName   = "maistra-admission-controller"
)

func createWebhookResources(ctx context.Context, mgr manager.Manager, log logr.Logger, operatorNamespace string) error {
	cl, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		return pkgerrors.Wrap(err, "error creating k8s client")
	}

	webhookServiceCreated := false
	log.Info("Creating Maistra webhook Service")
	if err := cl.Create(context.TODO(), newWebhookService(operatorNamespace)); err == nil {
		webhookServiceCreated = true
	} else {
		if errors.IsAlreadyExists(err) {
			log.Info("Maistra webhook Service already exists")
		} else {
			return pkgerrors.Wrap(err, "error creating Maistra webhook Service")
		}
	}

	log.Info("Creating Maistra webhook CA bundle ConfigMap")
	if err := cl.Create(context.TODO(), newCABundleConfigMap(operatorNamespace)); err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("Maistra webhook CA bundle ConfigMap already exists")
		} else {
			return pkgerrors.Wrap(err, "error creating Maistra webhook CA bundle ConfigMap")
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
		&webhookca.ConfigMapCABundleSource{
			Namespace:     operatorNamespace,
			ConfigMapName: webhookConfigMapName,
			Key:           common.ServiceCABundleKey,
		}); err != nil {
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
		&webhookca.ConfigMapCABundleSource{
			Namespace:     operatorNamespace,
			ConfigMapName: webhookConfigMapName,
			Key:           common.ServiceCABundleKey,
		}); err != nil {
		return err
	}

	if err := RegisterConversionWebhook(ctx, cl, log, operatorNamespace, &smcpConverterServicePath, webhookca.ServiceMeshControlPlaneCRDName, true); err != nil {
		return err
	}

	if err := RegisterConversionWebhook(ctx, cl, log, operatorNamespace, &SmeConverterServicePath, webhookca.ServiceMeshExtensionCRDName, false); err != nil {
		return err
	}

	// wait for secret to become available to prevent the operator from bouncing
	// we don't worry about any errors here, as the worst thing that will happen
	// is that the operator might restart.
	coreclient, err := clientcorev1.NewForConfig(mgr.GetConfig())
	if err != nil {
		log.Info("error occurred creating client for watching Maistra webhook Secret")
		return nil
	}
	secretwatch, err := coreclient.Secrets(operatorNamespace).Watch(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", webhookSecretName)})
	if err != nil {
		log.Info("error occurred creating watch for Maistra webhook Secret")
		return nil
	}
	func() {
		defer secretwatch.Stop()
		for {
			select {
			case <-secretwatch.ResultChan():
				return
			case <-time.After(1 * time.Second):
				log.Info("Waiting for Maistra webhook Secret to become available", "Secret", webhookSecretName)
			}
		}
	}()
	log.Info("Maistra webhook Secret is ready", "Secret", webhookSecretName)

	configMapWatch, err := coreclient.ConfigMaps(operatorNamespace).Watch(ctx,
		metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", webhookConfigMapName)})
	if err != nil {
		log.Info("error occurred creating watch for Maistra webhook CA bundle ConfigMap")
		return nil
	}
	func() {
		defer configMapWatch.Stop()
		for {
			select {
			case <-configMapWatch.ResultChan():
				return
			case <-time.After(1 * time.Second):
				log.Info("Waiting for Maistra webhook CA bundle ConfigMap to become available", "ConfigMap", webhookConfigMapName)
			}
		}
	}()
	log.Info("Maistra webhook CA bundle ConfigMap is ready", "ConfigMap", webhookConfigMapName)

	// If we have just created the webhook Service, then the Secret didn't exist
	// when the operator Pod was created and so the Secret couldn't have been
	// mounted. We exit so that the container is restarted. In the next run of
	// the container, the Secret will be mounted.
	if webhookServiceCreated {
		log.Info("Restarting to obtain the Maistra webhook Secret and CA bundle ConfigMap...")
		os.Exit(0)
	}

	return nil
}

func RegisterConversionWebhook(
	ctx context.Context,
	cl client.Client,
	log logr.Logger,
	operatorNamespace string,
	path *string,
	crdName string,
	crdMustExist bool,
) error {
	log.Info(fmt.Sprintf("Adding conversion webhook to %s CRD", crdName))

	crd := &apixv1.CustomResourceDefinition{}
	if err := cl.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
		if !crdMustExist && errors.IsNotFound(err) {
			log.Info(fmt.Sprintf("Not registering conversion webhook for CRD %q as it doesn't exist yet.", crdName))
			return nil
		}

		return err
	}

	newCrd := crd.DeepCopy()
	newCrd.Spec.Conversion = &apixv1.CustomResourceConversion{
		Strategy: apixv1.WebhookConverter,
		Webhook: &apixv1.WebhookConversion{
			ConversionReviewVersions: []string{"v1beta1"},
			ClientConfig: &apixv1.WebhookClientConfig{
				URL: nil,
				Service: &apixv1.ServiceReference{
					Name:      webhookServiceName,
					Namespace: operatorNamespace,
					Path:      path,
				},
			},
		},
	}

	log.Info(fmt.Sprintf("Registering Maistra %s CRD conversion webhook with CABundle reconciler", crdName))
	if err := webhookca.WebhookCABundleManagerInstance.ManageWebhookCABundle(
		newCrd,
		&webhookca.ConfigMapCABundleSource{
			Namespace:     operatorNamespace,
			ConfigMapName: webhookConfigMapName,
			Key:           common.ServiceCABundleKey,
		}); err != nil {
		return fmt.Errorf("error registering %s CRD conversion webhook with CABundle reconciler: %v", crdName, err)
	}

	if err := cl.Patch(ctx, newCrd, client.MergeFrom(crd), client.FieldOwner(common.FinalizerName)); err != nil {
		return fmt.Errorf("error patching CRD %s: %v", crdName, err)
	}

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

func newCABundleConfigMap(namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webhookConfigMapName,
			Namespace: namespace,
			Annotations: map[string]string{
				"service.beta.openshift.io/inject-cabundle": "true",
			},
		},
	}
}

func newValidatingWebhookConfiguration(namespace string) *admissionv1.ValidatingWebhookConfiguration {
	noneSideEffects := admissionv1.SideEffectClassNone
	return &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.servicemesh-resources.maistra.io", namespace),
			Annotations: map[string]string{
				"service.beta.openshift.io/inject-cabundle": "true",
			},
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
			Annotations: map[string]string{
				"service.beta.openshift.io/inject-cabundle": "true",
			},
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
