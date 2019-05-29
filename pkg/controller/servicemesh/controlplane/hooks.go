package controlplane

import (
	"context"
	"fmt"
	"regexp"
	"time"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/apiserver/pkg/authentication/serviceaccount"

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

func (r *ControlPlaneReconciler) processNewObject(object *unstructured.Unstructured) error {
	switch object.GetKind() {
	case "ServiceAccount":
		return r.processNewServiceAccount(object)
	}
	return nil
}

func (r *ControlPlaneReconciler) processDeletedObject(object *unstructured.Unstructured) error {
	switch object.GetKind() {
	case "ServiceAccount":
		return r.processDeletedServiceAccount(object)
	case "Route":
		if object.GetName() == "kiali" {
			return r.processDeletedKialiRoute(object)
		}
	}
	return nil
}

var (
	grafanaRegexp = regexp.MustCompile("(grafana:\\s*url:).*?\n")
	jaegerRegexp  = regexp.MustCompile("(jaeger:\\s*url:).*?\n")
)

func (r *ControlPlaneReconciler) patchKialiConfig(object *unstructured.Unstructured) error {
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

func (r *ControlPlaneReconciler) processDeletedKialiRoute(route *unstructured.Unstructured) error {
	oauthClient := &unstructured.Unstructured{}
	oauthClient.SetAPIVersion("oauth.openshift.io/v1")
	oauthClient.SetKind("OAuthClient")
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: "kiali"}, oauthClient)
	if err != nil {
		r.Log.Error(err, "error retrieving kiali OAuthClient")
		return err
	}
	err = r.patchKialiOAuthClient(oauthClient)
	if err != nil {
		r.Log.Error(err, "error removing deleted Route from OAuthClient", "Route", route.GetName())
		return err
	}
	redirectURIs, exists, err := unstructured.NestedStringSlice(oauthClient.UnstructuredContent(), "redirectURIs")
	if err != nil {
		r.Log.Error(err, "error retrieving redirectURIs from kiali OAuthClient")
		return err
	}
	if !exists || len(redirectURIs) == 0 {
		return r.Client.Delete(context.TODO(), oauthClient)
	}
	return r.Client.Update(context.TODO(), oauthClient)
}

func (r *ControlPlaneReconciler) patchKialiOAuthClient(object *unstructured.Unstructured) error {
	r.Log.Info("patching kiali OAuthClient", object.GetKind(), object.GetName())

	// get kiali route host
	kialiRouteList := &unstructured.UnstructuredList{}
	kialiRouteList.SetAPIVersion("route.openshift.io/v1")
	kialiRouteList.SetKind("Route")
	listOptions := client.MatchingField("metadata.name", "kiali")
	labelSelector, err := labels.NewRequirement(common.OwnerKey, selection.Exists, []string{})
	if err != nil {
		r.Log.Error(err, "error creating label selector for kiali routes associated with service meshes")
		return fmt.Errorf("error creating label selectors for kiali routes: %s", err)
	}
	listOptions.LabelSelector = labels.NewSelector().Add(*labelSelector)
	err = r.Client.List(context.TODO(), listOptions, kialiRouteList)
	if err != nil && !errors.IsNotFound(err) {
		r.Log.Error(err, "error retrieving kiali route")
		return fmt.Errorf("could not retrieve kiali route: %s", err)
	}

	redirectURIs := make([]string, 0, len(kialiRouteList.Items))
	for _, kialiRoute := range kialiRouteList.Items {
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
		redirectURIs = append(redirectURIs, kialiURL)
	}

	// delete the owner key so this doesn't get deleted during a normal prune
	common.DeleteLabel(object, common.OwnerKey)
	common.DeleteAnnotation(object, common.MeshGenerationKey)

	// set the redirect URIs
	return unstructured.SetNestedStringSlice(object.UnstructuredContent(), redirectURIs, "redirectURIs")
}

// add-scc-to-user anyuid to service accounts: citadel, egressgateway, galley, ingressgateway, mixer, pilot, sidecar-injector
// plus: grafana, prometheus

// add-scc-to-user privileged service accounts: jaeger
func (r *ControlPlaneReconciler) processNewServiceAccount(object *unstructured.Unstructured) error {
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

func (r *ControlPlaneReconciler) processDeletedServiceAccount(object *unstructured.Unstructured) error {
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
