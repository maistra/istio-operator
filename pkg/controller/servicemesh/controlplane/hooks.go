package controlplane

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

func (r *controlPlaneInstanceReconciler) processNewComponent(name string, status *status.ComponentStatus) error {
	return nil
}

func (r *controlPlaneInstanceReconciler) processDeletedComponent(name string, status *status.ComponentStatus) error {
	// nop
	return nil
}

func (r *controlPlaneInstanceReconciler) preprocessObject(ctx context.Context, object *unstructured.Unstructured) (bool, error) {
	// Add owner ref
	if object.GetNamespace() == r.Instance.GetNamespace() {
		object.SetOwnerReferences(r.ownerRefs)
	} else {
		// XXX: can't set owner reference on cross-namespace or cluster resources
	}

	// add generation annotation
	common.SetAnnotation(object, common.MeshGenerationKey, r.meshGeneration)

	// pull chart version
	if r.chartVersion == "" {
		r.chartVersion, _ = common.GetLabel(object, "maistra-version")
	}

	switch object.GetKind() {
	case "Kiali":
		return true, r.patchKialiConfig(ctx, object)
	case "ConfigMap":
		if object.GetName() == "istio-grafana" {
			return true, r.patchGrafanaConfig(ctx, object)
		}
	case "Secret":
		switch object.GetName() {
		case "htpasswd":
			return true, r.patchHtpasswdSecret(ctx, object)
		case "prometheus-proxy", "grafana-proxy":
			return true, r.patchProxySecret(ctx, object)
		}
	case "NetworkPolicy":
		mustContinue := true
		if r.Instance.Spec.Security != nil && r.Instance.Spec.Security.ManageNetworkPolicy != nil {
			mustContinue = *r.Instance.Spec.Security.ManageNetworkPolicy
		}
		return mustContinue, nil
	}

	return true, nil
}

func (r *controlPlaneInstanceReconciler) preprocessObjectForPatch(ctx context.Context, oldObj, newObj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if newObj.GetKind() == "Kiali" {
		accessibleNamespaces, found, err := unstructured.NestedStringSlice(oldObj.UnstructuredContent(), "spec", "deployment", "accessible_namespaces")
		if err != nil {
			return nil, err
		}
		if found {
			newObj = newObj.DeepCopy() // we make a copy in case the patch fails and a full CREATE is then performed; the accessible_namespaces field must be present when creating the object
			err = unstructured.SetNestedStringSlice(newObj.UnstructuredContent(), accessibleNamespaces, "spec", "deployment", "accessible_namespaces")
			if err != nil {
				return nil, err
			}
		}
	}
	return newObj, nil
}

func (r *controlPlaneInstanceReconciler) processNewObject(ctx context.Context, object *unstructured.Unstructured) error {
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
		jaegerURL = r.jaegerURL(ctx, log)
		if jaegerURL == "" {
			jaegerEnabled = false // there is no host on this route - disable it in kiali
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
		grafanaURL, err = r.grafanaURL(ctx, log)
		if err != nil {
			grafanaEnabled = false
		} else if grafanaURL == "" {
			grafanaEnabled = false // there is no host on this route - disable it in kiali
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

	rawPassword, err := r.getRawHtPasswd(ctx)
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
