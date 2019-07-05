package controlplane

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ControlPlaneReconciler) processNewComponent(name string, status *v1.ComponentStatus) error {
	switch name {
	case "istio/charts/galley":
		r.waitForDeployments(status)
		if name == "istio/charts/galley" {
			for _, status := range status.FindResourcesOfKind("ValidatingWebhookConfiguration") {
				if installCondition := status.GetCondition(v1.ConditionTypeInstalled); installCondition.Status == v1.ConditionStatusTrue {
					webhookKey := v1.ResourceKey(status.Resource)
					r.waitForWebhookCABundleInitialization(webhookKey.ToUnstructured())
				}
			}
		}
	case "istio/charts/sidecarInjectorWebhook":
		for _, status := range status.FindResourcesOfKind("MutatingWebhookConfiguration") {
			if installCondition := status.GetCondition(v1.ConditionTypeInstalled); installCondition.Status == v1.ConditionStatusTrue {
				webhookKey := v1.ResourceKey(status.Resource)
				r.waitForWebhookCABundleInitialization(webhookKey.ToUnstructured())
			}
		}
		r.waitForDeployments(status)
	default:
		r.waitForDeployments(status)
	}
	return nil
}

func (r *ControlPlaneReconciler) processDeletedComponent(name string, status *v1.ComponentStatus) error {
	// nop
	return nil
}

func (r *ControlPlaneReconciler) preprocessObject(object *unstructured.Unstructured) error {
	switch object.GetKind() {
	case "Kiali":
		return r.patchKialiConfig(object)
	case "Jaeger":
		return r.generateElasticsearchSecrets(object)
	}
	return nil
}

func (r *ControlPlaneReconciler) processNewObject(object *unstructured.Unstructured) error {
	return nil
}

func (r *ControlPlaneReconciler) processDeletedObject(object *unstructured.Unstructured) error {
	return nil
}

func (r *ControlPlaneReconciler) patchKialiConfig(object *unstructured.Unstructured) error {
	r.Log.Info("patching kiali CR", object.GetKind(), object.GetName())

	// get jaeger URL and enabled flags from Kiali CR
	jaegerURL, found, err := unstructured.NestedString(object.UnstructuredContent(), "spec", "external_services", "tracing", "url")
	if !found || err != nil {
		jaegerURL = ""
	}
	jaegerEnabled, found, err := unstructured.NestedBool(object.UnstructuredContent(), "spec", "external_services", "tracing", "enabled")
	if !found || err != nil {
		jaegerEnabled = true // if not set, assume we want it - we'll turn this off if we can't auto-detect
	}

	// if the user has not yet configured this, let's try to auto-detect it now.
	if len(jaegerURL) == 0 && jaegerEnabled {
		r.Log.Info("attempting to auto-detect jaeger for kiali")
		jaegerRoute := &unstructured.Unstructured{}
		jaegerRoute.SetAPIVersion("route.openshift.io/v1")
		jaegerRoute.SetKind("Route")
		// jaeger is the name of the Jaeger CR
		err = r.Client.Get(context.TODO(), client.ObjectKey{Name: "jaeger", Namespace: object.GetNamespace()}, jaegerRoute)
		if err != nil {
			if !errors.IsNotFound(err) {
				r.Log.Error(err, "error retrieving jaeger route - will disable it in Kiali")
				// we aren't going to return here - Jaeger is optional for Kiali; Kiali can still run without it
			}
			jaegerEnabled = false
		} else {
			jaegerURL, _, _ = unstructured.NestedString(jaegerRoute.UnstructuredContent(), "spec", "host")
			jaegerScheme := "http"
			if jaegerTLSTermination, ok, _ := unstructured.NestedString(jaegerRoute.UnstructuredContent(), "spec", "tls", "termination"); ok && len(jaegerTLSTermination) > 0 {
				jaegerScheme = "https"
			}
			if len(jaegerURL) > 0 {
				jaegerURL = fmt.Sprintf("%s://%s", jaegerScheme, jaegerURL)
			} else {
				jaegerEnabled = false // there is no host on this route - disable it in kiali
			}
		}
	}

	// get grafana URL and enabled flags from Kiali CR
	grafanaURL, found, err := unstructured.NestedString(object.UnstructuredContent(), "spec", "external_services", "grafana", "url")
	if !found || err != nil {
		grafanaURL = ""
	}
	grafanaEnabled, found, err := unstructured.NestedBool(object.UnstructuredContent(), "spec", "external_services", "grafana", "enabled")
	if !found || err != nil {
		grafanaEnabled = true // if not set, assume we want it - we'll turn this off if we can't auto-detect
	}

	// if the user has not yet configured this, let's try to auto-detect it now.
	if len(grafanaURL) == 0 && grafanaEnabled {
		r.Log.Info("attempting to auto-detect grafana for kiali")
		grafanaRoute := &unstructured.Unstructured{}
		grafanaRoute.SetAPIVersion("route.openshift.io/v1")
		grafanaRoute.SetKind("Route")
		err = r.Client.Get(context.TODO(), client.ObjectKey{Name: "grafana", Namespace: object.GetNamespace()}, grafanaRoute)
		if err != nil {
			if !errors.IsNotFound(err) {
				r.Log.Error(err, "error retrieving grafana route - will disable it in Kiali")
				// we aren't going to return here - Grafana is optional for Kiali; Kiali can still run without it
			}
			grafanaEnabled = false
		} else {
			grafanaURL, _, _ = unstructured.NestedString(grafanaRoute.UnstructuredContent(), "spec", "host")
			grafanaScheme := "http"
			if grafanaTLSTermination, ok, _ := unstructured.NestedString(grafanaRoute.UnstructuredContent(), "spec", "tls", "termination"); ok && len(grafanaTLSTermination) > 0 {
				grafanaScheme = "https"
			}
			if len(grafanaURL) > 0 {
				grafanaURL = fmt.Sprintf("%s://%s", grafanaScheme, grafanaURL)
			} else {
				grafanaEnabled = false // there is no host on this route - disable it in kiali
			}
		}
	}

	r.Log.Info("new kiali jaeger settings", jaegerURL, jaegerEnabled)
	r.Log.Info("new kiali grafana setting", grafanaURL, grafanaEnabled)

	err = unstructured.SetNestedField(object.UnstructuredContent(), jaegerURL, "spec", "external_services", "tracing", "url")
	if err != nil {
		return fmt.Errorf("could not set jaeger url in kiali CR: %s", err)
	}

	err = unstructured.SetNestedField(object.UnstructuredContent(), jaegerEnabled, "spec", "external_services", "tracing", "enabled")
	if err != nil {
		return fmt.Errorf("could not set jaeger enabled flag in kiali CR: %s", err)
	}

	err = unstructured.SetNestedField(object.UnstructuredContent(), grafanaURL, "spec", "external_services", "grafana", "url")
	if err != nil {
		return fmt.Errorf("could not set grafana url in kiali CR: %s", err)
	}

	err = unstructured.SetNestedField(object.UnstructuredContent(), grafanaEnabled, "spec", "external_services", "grafana", "enabled")
	if err != nil {
		return fmt.Errorf("could not set grafana enabled flag in kiali CR: %s", err)
	}

	return nil
}

func (r *ControlPlaneReconciler) generateElasticsearchSecrets(object *unstructured.Unstructured) error {
	// if we are using Elasticsearch, we need to check whether the secrets are there already
	// and create if they are absent
	jaegerWithElasticsearch, found, err := unstructured.NestedString(object.UnstructuredContent(), "spec", "external_services", "tracing", "jaeger", "template")
	if !found || err != nil {
		jaegerWithElasticsearch = "production-elasticsearch" // this is the default when omitted
	}

	if "production-elasticsearch" == jaegerWithElasticsearch {
		// do we have the secrets already?
		// list of secrets we expect to exist: jaeger-curator, elasticsearch, jaeger-elasticsearch, jaeger-master-certs
		// if any one of them are missing, we have to (re-)create them all
		for _, secret := range []string{"jaeger-curator", "elasticsearch", "jaeger-elasticsearch", "jaeger-master-certs"} {
			missing, err := r.secretMissing(secret, object.GetNamespace())
			if err != nil {
				return fmt.Errorf("failed to check if the secret '%s' is missing", secret)
			}

			if missing {
				// generate new certs
				if err = r.generateCerts(object.GetNamespace()); err != nil {
					return fmt.Errorf("failed to generate new certs: %s", err)
				}

				// generate new secrets
				if err = r.populateCertValues(object); err != nil {
					// if one is missing, we need to generate it again, but we might get cert mismatches, so,
					// we just generate them all again -- in that case, no need to check if the next secret is missing
					break
				}
			}
		}
	}

	return nil
}

func (r *ControlPlaneReconciler) secretMissing(secret string, namespace string) (bool, error) {
	s := &unstructured.Unstructured{}
	s.SetAPIVersion("v1")
	s.SetKind("Secret")
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: secret, Namespace: namespace}, s)
	if err != nil {
		if !errors.IsNotFound(err) {
			r.Log.Error(err, "error retrieving secret")
			return false, err // err
		}
		return true, nil // not found
	}

	return false, nil // found
}

func (r *ControlPlaneReconciler) generateCerts(namespace string) error {
	script := "/usr/local/bin/cert_generation.sh" // should match the place specified in the Dockerfile
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "NAMESPACE="+namespace)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.Log.Error(err, "failed to create certificates")
		r.Log.V(100).Info("output from the cert_generation.sh script", "output", out)
		return fmt.Errorf("error running script %s: %v", script, err)
	}
	return nil
}

func (r *ControlPlaneReconciler) populateCertValues(object *unstructured.Unstructured) error {
	mapping := map[string]string{
		"spec.external_services.tracing.jaeger.es.certs.ca":                              "ca.crt",
		"spec.external_services.tracing.jaeger.es.certs.ca-key":                          "ca.key",
		"spec.external_services.tracing.jaeger.es.certs.curator.cert":                    "system.logging.curator.crt",
		"spec.external_services.tracing.jaeger.es.certs.curator.key":                     "system.logging.curator.key",
		"spec.external_services.tracing.jaeger.es.certs.elasticsearch.admin-cert":        "system.admin.crt",
		"spec.external_services.tracing.jaeger.es.certs.elasticsearch.admin-key":         "system.admin.key",
		"spec.external_services.tracing.jaeger.es.certs.elasticsearch.elasticsearch.crt": "elasticsearch.crt",
		"spec.external_services.tracing.jaeger.es.certs.elasticsearch.elasticsearch.key": "elasticsearch.key",
		"spec.external_services.tracing.jaeger.es.certs.elasticsearch.logging-es.crt":    "logging-es.crt",
		"spec.external_services.tracing.jaeger.es.certs.elasticsearch.logging-es.key":    "logging-es.key",
		"spec.external_services.tracing.jaeger.es.certs.client.cert":                     "user.jaeger.crt",
		"spec.external_services.tracing.jaeger.es.certs.client.key":                      "user.jaeger.key",
	}

	for k, v := range mapping {
		field := strings.Split(k, ".")
		contents, err := getFileContents(v)
		if err != nil {
			return fmt.Errorf("could not get the cert contents for the key '%s'", k)
		}
		encoded := base64.StdEncoding.EncodeToString(contents)

		err = unstructured.SetNestedField(object.UnstructuredContent(), encoded, field...)
		if err != nil {
			return fmt.Errorf("could not populate the cert value for the key '%s': %v", k, err)
		}

	}

	return nil
}

func getFileContents(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("path to file is empty")
	}
	contents, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	return contents, nil
}

func (r *ControlPlaneReconciler) waitForDeployments(status *v1.ComponentStatus) error {
	for _, status := range status.FindResourcesOfKind("StatefulSet") {
		if installCondition := status.GetCondition(v1.ConditionTypeInstalled); installCondition.Status == v1.ConditionStatusTrue {
			deploymentKey := v1.ResourceKey(status.Resource)
			r.waitForDeployment(deploymentKey.ToUnstructured())
		}
	}
	for _, status := range status.FindResourcesOfKind("Deployment") {
		if installCondition := status.GetCondition(v1.ConditionTypeInstalled); installCondition.Status == v1.ConditionStatusTrue {
			deploymentKey := v1.ResourceKey(status.Resource)
			r.waitForDeployment(deploymentKey.ToUnstructured())
		}
	}
	for _, status := range status.FindResourcesOfKind("DeploymentConfig") {
		if installCondition := status.GetCondition(v1.ConditionTypeInstalled); installCondition.Status == v1.ConditionStatusTrue {
			deploymentKey := v1.ResourceKey(status.Resource)
			r.waitForDeployment(deploymentKey.ToUnstructured())
		}
	}
	return nil
}

// XXX: configure wait period
func (r *ControlPlaneReconciler) waitForDeployment(object *unstructured.Unstructured) error {
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

func (r *ControlPlaneReconciler) waitForWebhookCABundleInitialization(object *unstructured.Unstructured) error {
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
