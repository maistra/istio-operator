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

	"k8s.io/apiserver/pkg/authentication/serviceaccount"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var launcherProjectName = "devex"

// XXX: should call this from a hook, e.g. preprocessNewComponent()
func (r *controlPlaneReconciler) createLauncherProject() error {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: launcherProjectName}}
	err := r.client.Get(context.TODO(), client.ObjectKey{Name: launcherProjectName}, namespace)
	if err == nil {
		// project exists
		return nil
	} else if !errors.IsNotFound(err) {
		return err
	}
	r.log.Info("creating launcher project")
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
	return r.client.Create(context.TODO(), projectRequest)
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
		return r.client.Delete(context.TODO(), project)
	}
	return nil
}

func (r *controlPlaneReconciler) patchObject(object *unstructured.Unstructured) error {
	gk := object.GroupVersionKind().GroupKind()
	switch gk.String() {
	case "ConfigMap":
		if object.GetName() == "kiali" {
			return r.patchKialiConfig(object)
		}
	}
	return nil
}

func (r *controlPlaneReconciler) processNewObject(object *unstructured.Unstructured) error {
	gk := object.GroupVersionKind().GroupKind()
	switch gk.String() {
	case "ServiceAccount":
		return r.processNewServiceAccount(object)
	}
	return nil
}

func (r *controlPlaneReconciler) processDeletedObject(object *unstructured.Unstructured) error {
	gk := object.GroupVersionKind().GroupKind()
	switch gk.String() {
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
	configYaml, found, err := unstructured.NestedString(object.UnstructuredContent(), "data", "config.yaml")
	if err != nil {
		// This shouldn't occur if it's really a ConfigMap, but...
		r.log.Error(err, "could not parse kiali ConfigMap")
		return err
	} else if !found {
		return nil
	}

	// get jaeger route host
	jaegerRoute := &unstructured.Unstructured{}
	jaegerRoute.SetAPIVersion("route.openshift.io/v1")
	jaegerRoute.SetKind("Route")
	err = r.client.Get(context.TODO(), client.ObjectKey{Name: "jaeger-query", Namespace: object.GetNamespace()}, jaegerRoute)
	if err != nil && !errors.IsNotFound(err) {
		r.log.Error(err, "error retrieving jaeger route")
		return fmt.Errorf("could not retrieve jaeger route: %s", err)
	}

	// get grafana route host
	grafanaRoute := &unstructured.Unstructured{}
	grafanaRoute.SetAPIVersion("route.openshift.io/v1")
	grafanaRoute.SetKind("Route")
	err = r.client.Get(context.TODO(), client.ObjectKey{Name: "grafana", Namespace: object.GetNamespace()}, grafanaRoute)
	if err != nil && !errors.IsNotFound(err) {
		r.log.Error(err, "error retrieving grafana route")
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
		return r.addUserToSCC("anyuid", serviceaccount.MakeUsername(object.GetNamespace(), object.GetName()))
	case "jaeger":
		return r.addUserToSCC("privileged", serviceaccount.MakeUsername(object.GetNamespace(), object.GetName()))
	}
	return nil
}

func (r *controlPlaneReconciler) addUserToSCC(sccName, user string) error {
	scc := &unstructured.Unstructured{}
	scc.SetAPIVersion("v1")
	scc.SetKind("SecurityContextConstraints")
	err := r.client.Get(context.TODO(), client.ObjectKey{Name: sccName}, scc)

	if err == nil {
		users, exists, _ := unstructured.NestedStringSlice(scc.UnstructuredContent(), "users")
		if !exists {
			users = []string{}
		}
		if indexOf(users, user) < 0 {
			r.log.Info("Adding ServiceAccount to SecurityContextConstraints", "ServiceAccount", user, "SecurityContextConstraints", sccName)
			users = append(users, user)
			unstructured.SetNestedStringSlice(scc.UnstructuredContent(), users, "users")
			err = r.client.Update(context.TODO(), scc)
		}
	}
	return err
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
		return r.removeUserFromSCC("anyuid", serviceaccount.MakeUsername(object.GetNamespace(), object.GetName()))
	case "jaeger":
		return r.removeUserFromSCC("privileged", serviceaccount.MakeUsername(object.GetNamespace(), object.GetName()))
	}
	return nil
}

func (r *controlPlaneReconciler) removeUserFromSCC(sccName, user string) error {
	scc := &unstructured.Unstructured{}
	scc.SetAPIVersion("v1")
	scc.SetKind("SecurityContextConstraints")
	err := r.client.Get(context.TODO(), client.ObjectKey{Name: sccName}, scc)

	if err == nil {
		users, exists, _ := unstructured.NestedStringSlice(scc.UnstructuredContent(), "users")
		if !exists {
			return nil
		}
		if index := indexOf(users, user); index >= 0 {
			r.log.Info("Removing ServiceAccount from SecurityContextConstraints", "ServiceAccount", user, "SecurityContextConstraints", sccName)
			users = append(users[:index], users[index+1:]...)
			unstructured.SetNestedStringSlice(scc.UnstructuredContent(), users, "users")
			err = r.client.Update(context.TODO(), scc)
		}
	}
	return err
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
	r.log.Info("waiting for deployment to become ready", object.GetKind(), name)
	for i := 0; i < 10; i++ {
		err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: object.GetNamespace(), Name: name}, object)
		if err != nil {
			if errors.IsNotFound(err) {
				// ???
				r.log.Error(nil, "attempting to wait on unknown deployment", object.GetKind(), name)
				return nil
			}
			// skip it
			r.log.Error(err, "unexpected error occurred waiting for deployment to become ready", object.GetKind(), name)
			return nil
		}
		if val, _, _ := unstructured.NestedInt64(object.UnstructuredContent(), "status", "readyReplicas"); val > 0 {
			return nil
		}
		time.Sleep(6 * time.Second)
	}
	r.log.Error(nil, "deployment failed to become ready after 600s", object.GetKind(), name)
	return nil
}

func (r *controlPlaneReconciler) waitForWebhookCABundleInitialization(object *unstructured.Unstructured) error {
	name := object.GetName()
	kind := object.GetKind()
	r.log.Info("waiting for webhook CABundle initialization", kind, name)
outer:
	for i := 0; i < 10; i++ {
		r.client.Get(context.TODO(), client.ObjectKey{Name: name}, object)
		webhooks, found, _ := unstructured.NestedSlice(object.UnstructuredContent(), "webhooks")
		if !found || len(webhooks) == 0 {
			return nil
		}
		for _, webhook := range webhooks {
			typedWebhook, _ := webhook.(map[string]interface{})
			if caBundle, found, _ := unstructured.NestedString(typedWebhook, "clientConfig", "caBundle"); !found || len(caBundle) == 0 {
				time.Sleep(6 * time.Second)
				continue outer
			}
		}
		return nil
	}
	r.log.Error(nil, "webhook CABundle failed to become initialized after 600s", kind, name)
	return nil
}
