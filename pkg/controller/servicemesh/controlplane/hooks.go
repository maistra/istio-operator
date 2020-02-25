package controlplane

import (
	"context"
	"fmt"
	"time"

	"github.com/maistra/istio-operator/pkg/controller/common"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *controlPlaneInstanceReconciler) processNewComponent(name string, status *v1.ComponentStatus) error {
	return nil
}

func (r *controlPlaneInstanceReconciler) processDeletedComponent(name string, status *v1.ComponentStatus) error {
	// nop
	return nil
}

func (r *controlPlaneInstanceReconciler) preprocessObject(ctx context.Context, object *unstructured.Unstructured) error {
	// Add owner ref
	if object.GetNamespace() == r.Instance.GetNamespace() {
		object.SetOwnerReferences(r.ownerRefs)
	} else {
		// XXX: can't set owner reference on cross-namespace or cluster resources
	}

	// add generation annotation
	common.SetAnnotation(object, common.MeshGenerationKey, r.meshGeneration)

	switch object.GetKind() {
	case "Kiali":
		return r.patchKialiConfig(ctx, object)
	case "ConfigMap":
		if object.GetName() == "istio-grafana" {
			return r.patchGrafanaConfig(ctx, object)
		}
	case "Secret":
		switch object.GetName() {
		case "htpasswd":
			return r.patchHtpasswdSecret(ctx, object)
		case "prometheus-proxy", "grafana-proxy":
			return r.patchProxySecret(ctx, object)
		}
	}
	return nil
}

func (r *controlPlaneInstanceReconciler) processNewObject(ctx context.Context, object *unstructured.Unstructured) error {
	return nil
}

func (r *controlPlaneInstanceReconciler) processDeletedObject(ctx context.Context, object *unstructured.Unstructured) error {
	return nil
}

func (r *controlPlaneInstanceReconciler) patchKialiConfig(ctx context.Context, object *unstructured.Unstructured) error {
	log := common.LogFromContext(ctx)
	log.Info("patching kiali CR", object.GetKind(), object.GetName())

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
		log.Info("attempting to auto-detect jaeger for kiali")
		jaegerRoute := &unstructured.Unstructured{}
		jaegerRoute.SetAPIVersion("route.openshift.io/v1")
		jaegerRoute.SetKind("Route")
		// jaeger is the name of the Jaeger CR
		err = r.Client.Get(ctx, client.ObjectKey{Name: "jaeger", Namespace: object.GetNamespace()}, jaegerRoute)
		if err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "error retrieving jaeger route - will disable it in Kiali")
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
		log.Info("attempting to auto-detect grafana for kiali")
		grafanaRoute := &unstructured.Unstructured{}
		grafanaRoute.SetAPIVersion("route.openshift.io/v1")
		grafanaRoute.SetKind("Route")
		err = r.Client.Get(ctx, client.ObjectKey{Name: "grafana", Namespace: object.GetNamespace()}, grafanaRoute)
		if err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "error retrieving grafana route - will disable it in Kiali")
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

	log.Info("new kiali jaeger settings", jaegerURL, jaegerEnabled)
	log.Info("new kiali grafana setting", grafanaURL, grafanaEnabled)

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

	rawPassword, err := r.getRawHtPasswd(ctx, object)
	if err != nil {
		return err
	}
	err = unstructured.SetNestedField(object.UnstructuredContent(), rawPassword, "spec", "external_services", "grafana", "auth", "password")
	if err != nil {
		return fmt.Errorf("could not set grafana password in kiali CR: %s", err)
	}
	err = unstructured.SetNestedField(object.UnstructuredContent(), rawPassword, "spec", "external_services", "prometheus", "auth", "password")
	if err != nil {
		return fmt.Errorf("could not set prometheus password in kiali CR: %s", err)
	}
	err = unstructured.SetNestedField(object.UnstructuredContent(), rawPassword, "spec", "external_services", "tracing", "auth", "password")
	if err != nil {
		return fmt.Errorf("could not set tracing password in kiali CR: %s", err)
	}

	return nil
}

func (r *controlPlaneInstanceReconciler) waitForWebhookCABundleInitialization(ctx context.Context, object *unstructured.Unstructured) error {
	log := common.LogFromContext(ctx)
	name := object.GetName()
	kind := object.GetKind()
	log.Info("waiting for webhook CABundle initialization", kind, name)
	err := wait.ExponentialBackoff(wait.Backoff{Duration: 6 * time.Second, Steps: 10, Factor: 1.1}, func() (bool, error) {
		err := r.Client.Get(ctx, client.ObjectKey{Name: name}, object)
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
			log.Error(nil, "attempting to wait on unknown webhook", kind, name)
			return true, nil
		}
		log.Error(err, "error waiting for webhook CABundle to become initialized", object.GetKind(), name)
		return false, err
	})
	if err != nil {
		log.Error(nil, "webhook CABundle failed to become initialized in a timely manner", kind, name)
	}
	return nil
}
