package controlplane

import (
	"context"
	"fmt"
	"regexp"
	"time"

	istiov1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/v1alpha3"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/apiserver/pkg/authentication/serviceaccount"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var launcherProjectName = "devex"

// XXX: should call this from a hook, e.g. preprocessNewComponent()
func (r *controlPlaneReconciler) createLauncherProject() error {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: launcherProjectName}}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: launcherProjectName}, namespace)
	if err == nil {
		// project exists
		return nil
	} else if !errors.IsNotFound(err) {
		return err
	}
	r.Log.Info("creating launcher project")
	projectRequest := &unstructured.Unstructured{}
	projectRequest.SetAPIVersion("project.openshift.io/v1")
	projectRequest.SetKind("ProjectRequest")
	projectRequest.SetName(launcherProjectName)
	projectRequest.SetOwnerReferences(r.ownerRefs)
	projectRequest.SetLabels(map[string]string{
		"app":                                  "fabric8-launcher",
		"istio.openshift.com/ignore-namespace": "ignore",
	})
	unstructured.SetNestedField(projectRequest.UnstructuredContent(), "this project provides launcher capabilities and is administered by the istio-operator", "description")
	return r.Client.Create(context.TODO(), projectRequest)
}

func (r *controlPlaneReconciler) processNewComponent(name string, status *istiov1alpha3.ComponentStatus) error {
	switch name {
	case "istio/charts/galley":
		r.waitForDeployments(status)
		if name == "istio/charts/galley" {
			for _, status := range status.FindResourcesOfKind("ValidatingWebhookConfiguration") {
				if installCondition := status.GetCondition(istiov1alpha3.ConditionTypeInstalled); installCondition.Status == istiov1alpha3.ConditionStatusTrue {
					webhookKey := istiov1alpha3.ResourceKey(status.Resource)
					r.waitForWebhookCABundleInitialization(webhookKey.ToUnstructured())
				}
			}
		}
	case "istio/charts/sidecarInjectorWebhook":
		for _, status := range status.FindResourcesOfKind("MutatingWebhookConfiguration") {
			if installCondition := status.GetCondition(istiov1alpha3.ConditionTypeInstalled); installCondition.Status == istiov1alpha3.ConditionStatusTrue {
				webhookKey := istiov1alpha3.ResourceKey(status.Resource)
				r.waitForWebhookCABundleInitialization(webhookKey.ToUnstructured())
			}
		}
		r.waitForDeployments(status)
	default:
		r.waitForDeployments(status)
	}
	return nil
}

func (r *controlPlaneReconciler) processDeletedComponent(name string, status *istiov1alpha3.ComponentStatus) error {
	switch name {
	case "maistra-launcher":
		project := &unstructured.Unstructured{}
		project.SetAPIVersion("project.openshift.io/v1")
		project.SetKind("Project")
		project.SetName(launcherProjectName)
		return r.Client.Delete(context.TODO(), project)
	}
	return nil
}

func (r *controlPlaneReconciler) patchObject(object *unstructured.Unstructured) error {
	switch object.GetKind() {
	case "ConfigMap":
		if object.GetName() == "kiali" {
			return r.patchKialiConfig(object)
		}
	case "OAuthClient":
		if object.GetName() == "kiali" {
			return r.patchKialiOAuthClient(object)
		}
	}
	return nil
}

func (r *controlPlaneReconciler) processNewObject(object *unstructured.Unstructured) error {
	switch object.GetKind() {
	case "ServiceAccount":
		return r.processNewServiceAccount(object)
	}
	return nil
}

func (r *controlPlaneReconciler) processDeletedObject(object *unstructured.Unstructured) error {
	switch object.GetKind() {
	case "ServiceAccount":
		return r.processDeletedServiceAccount(object)
	}
	return nil
}

var (
	grafanaRegexp = regexp.MustCompile("(grafana:\\s*url:).*?\n")
	jaegerRegexp  = regexp.MustCompile("(jaeger:\\s*url:).*?\n")
)

func (r *controlPlaneReconciler) patchKialiConfig(object *unstructured.Unstructured) error {
	r.Log.Info("patching kiali ConfigMap", object.GetKind(), object.GetName())
	configYaml, found, err := unstructured.NestedString(object.UnstructuredContent(), "data", "config.yaml")
	if err != nil {
		// This shouldn't occur if it's really a ConfigMap, but...
		r.Log.Error(err, "could not parse kiali ConfigMap")
		return err
	} else if !found {
		return nil
	}

	// get jaeger route host
	jaegerRoute := &unstructured.Unstructured{}
	jaegerRoute.SetAPIVersion("route.openshift.io/v1")
	jaegerRoute.SetKind("Route")
	err = r.Client.Get(context.TODO(), client.ObjectKey{Name: "jaeger-query", Namespace: object.GetNamespace()}, jaegerRoute)
	if err != nil && !errors.IsNotFound(err) {
		r.Log.Error(err, "error retrieving jaeger route")
		return fmt.Errorf("could not retrieve jaeger route: %s", err)
	}

	// get grafana route host
	grafanaRoute := &unstructured.Unstructured{}
	grafanaRoute.SetAPIVersion("route.openshift.io/v1")
	grafanaRoute.SetKind("Route")
	err = r.Client.Get(context.TODO(), client.ObjectKey{Name: "grafana", Namespace: object.GetNamespace()}, grafanaRoute)
	if err != nil && !errors.IsNotFound(err) {
		r.Log.Error(err, "error retrieving grafana route")
		return fmt.Errorf("could not retrieve grafana route: %s", err)
	}

	// update config.yaml.external_services.grafana.url
	grafanaURL, _, _ := unstructured.NestedString(grafanaRoute.UnstructuredContent(), "spec", "host")
	configYaml = string(grafanaRegexp.ReplaceAll([]byte(configYaml), []byte(fmt.Sprintf("${1} http://%s\n", grafanaURL))))
	// update config.yaml.external_services.jaeger.url
	jaegerURL, _, _ := unstructured.NestedString(jaegerRoute.UnstructuredContent(), "spec", "host")
	configYaml = string(jaegerRegexp.ReplaceAll([]byte(configYaml), []byte(fmt.Sprintf("${1} https://%s\n", jaegerURL))))

	return unstructured.SetNestedField(object.UnstructuredContent(), configYaml, "data", "config.yaml")
}

func (r *controlPlaneReconciler) patchKialiOAuthClient(object *unstructured.Unstructured) error {
	r.Log.Info("patching kiali OAuthClient", object.GetKind(), object.GetName())
	redirectURIs, found, err := unstructured.NestedStringSlice(object.UnstructuredContent(), "redirectURIs")
	if err != nil {
		// This shouldn't occur if it's really a OAuthClient, but...
		r.Log.Error(err, "could not parse kiali OAuthClient")
		return err
	} else if !found {
		return nil
	}

	// get kiali route host
	kialiRoute := &unstructured.Unstructured{}
	kialiRoute.SetAPIVersion("route.openshift.io/v1")
	kialiRoute.SetKind("Route")
	err = r.Client.Get(context.TODO(), client.ObjectKey{Name: "kiali", Namespace: r.instance.GetNamespace()}, kialiRoute)
	if err != nil && !errors.IsNotFound(err) {
		r.Log.Error(err, "error retrieving kiali route")
		return fmt.Errorf("could not retrieve kiali route: %s", err)
	}

	kialiURL, found, err := unstructured.NestedString(kialiRoute.UnstructuredContent(), "spec", "host")
	if err != nil {
		r.Log.Error(err, "error retrieving kiali route host name")
		return err
	} else if !found {
		err = fmt.Errorf("host field not found in kiali route")
		r.Log.Error(err, "error retrieving kiali route host name")
		return err
	}
	if termination, found, _ := unstructured.NestedString(kialiRoute.UnstructuredContent(), "spec", "tls", "termination"); found && len(termination) > 0 {
		kialiURL = "https://" + kialiURL
	} else {
		kialiURL = "http://" + kialiURL
	}
	// update redirectURIs
	redirectURIs = append([]string{kialiURL}, redirectURIs...)

	return unstructured.SetNestedStringSlice(object.UnstructuredContent(), redirectURIs, "redirectURIs")
}

// add-scc-to-user anyuid to service accounts: citadel, egressgateway, galley, ingressgateway, mixer, pilot, sidecar-injector
// plus: grafana, prometheus

// add-scc-to-user privileged service accounts: jaeger
func (r *controlPlaneReconciler) processNewServiceAccount(object *unstructured.Unstructured) error {
	switch object.GetName() {
	case
		"istio-citadel-service-account",
		"istio-egressgateway-service-account",
		"istio-galley-service-account",
		"istio-ingressgateway-service-account",
		"istio-mixer-service-account",
		"istio-pilot-service-account",
		"istio-sidecar-injector-service-account",
		"grafana",
		"prometheus":
		_, err := r.AddUsersToSCC("anyuid", serviceaccount.MakeUsername(object.GetNamespace(), object.GetName()))
		return err
	case "jaeger":
		_, err := r.AddUsersToSCC("privileged", serviceaccount.MakeUsername(object.GetNamespace(), object.GetName()))
		return err
	}
	return nil
}

func (r *controlPlaneReconciler) processDeletedServiceAccount(object *unstructured.Unstructured) error {
	switch object.GetName() {
	case
		"istio-citadel-service-account",
		"istio-egressgateway-service-account",
		"istio-galley-service-account",
		"istio-ingressgateway-service-account",
		"istio-mixer-service-account",
		"istio-pilot-service-account",
		"istio-sidecar-injector-service-account",
		"grafana",
		"prometheus":
		return r.RemoveUsersFromSCC("anyuid", serviceaccount.MakeUsername(object.GetNamespace(), object.GetName()))
	case "jaeger":
		return r.RemoveUsersFromSCC("privileged", serviceaccount.MakeUsername(object.GetNamespace(), object.GetName()))
	}
	return nil
}

func (r *controlPlaneReconciler) waitForDeployments(status *istiov1alpha3.ComponentStatus) error {
	for _, status := range status.FindResourcesOfKind("StatefulSet") {
		if installCondition := status.GetCondition(istiov1alpha3.ConditionTypeInstalled); installCondition.Status == istiov1alpha3.ConditionStatusTrue {
			deploymentKey := istiov1alpha3.ResourceKey(status.Resource)
			r.waitForDeployment(deploymentKey.ToUnstructured())
		}
	}
	for _, status := range status.FindResourcesOfKind("Deployment") {
		if installCondition := status.GetCondition(istiov1alpha3.ConditionTypeInstalled); installCondition.Status == istiov1alpha3.ConditionStatusTrue {
			deploymentKey := istiov1alpha3.ResourceKey(status.Resource)
			r.waitForDeployment(deploymentKey.ToUnstructured())
		}
	}
	for _, status := range status.FindResourcesOfKind("DeploymentConfig") {
		if installCondition := status.GetCondition(istiov1alpha3.ConditionTypeInstalled); installCondition.Status == istiov1alpha3.ConditionStatusTrue {
			deploymentKey := istiov1alpha3.ResourceKey(status.Resource)
			r.waitForDeployment(deploymentKey.ToUnstructured())
		}
	}
	return nil
}

// XXX: configure wait period
func (r *controlPlaneReconciler) waitForDeployment(object *unstructured.Unstructured) error {
	name := object.GetName()
	// wait for deployment replicas >= 1
	r.Log.Info("waiting for deployment to become ready", object.GetKind(), name)
	err := wait.ExponentialBackoff(wait.Backoff{Duration: 6 * time.Second, Steps: 10, Factor: 1.1}, func() (bool, error) {
		err := r.Client.Get(context.TODO(), client.ObjectKey{Namespace: object.GetNamespace(), Name: name}, object)
		if err == nil {
			val, _, _ := unstructured.NestedInt64(object.UnstructuredContent(), "status", "readyReplicas")
			return val > 0, nil
		} else if errors.IsNotFound(err) {
			r.Log.Error(nil, "attempting to wait on unknown deployment", object.GetKind(), name)
			return true, nil
		}
		r.Log.Error(err, "unexpected error occurred waiting for deployment to become ready", object.GetKind(), name)
		return false, err
	})
	if err != nil {
		r.Log.Error(nil, "deployment failed to become ready in a timely manner", object.GetKind(), name)
	}
	return nil
}

func (r *controlPlaneReconciler) waitForWebhookCABundleInitialization(object *unstructured.Unstructured) error {
	name := object.GetName()
	kind := object.GetKind()
	r.Log.Info("waiting for webhook CABundle initialization", kind, name)
	err := wait.ExponentialBackoff(wait.Backoff{Duration: 6 * time.Second, Steps: 10, Factor: 1.1}, func() (bool, error) {
		err := r.Client.Get(context.TODO(), client.ObjectKey{Name: name}, object)
		if err == nil {
			webhooks, found, _ := unstructured.NestedSlice(object.UnstructuredContent(), "webhooks")
			if !found || len(webhooks) == 0 {
				return true, nil
			}
			for _, webhook := range webhooks {
				typedWebhook, _ := webhook.(map[string]interface{})
				if caBundle, found, _ := unstructured.NestedString(typedWebhook, "clientConfig", "caBundle"); !found || len(caBundle) == 0 {
					return false, nil
				}
			}
			return true, nil
		} else if errors.IsNotFound(err) {
			r.Log.Error(nil, "attempting to wait on unknown webhook", kind, name)
			return true, nil
		}
		r.Log.Error(err, "unexpected error occurred waiting for webhook CABundle to become initialized", object.GetKind(), name)
		return false, err
	})
	if err != nil {
		r.Log.Error(nil, "webhook CABundle failed to become initialized in a timely manner", kind, name)
	}
	return nil
}
