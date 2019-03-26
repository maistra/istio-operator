package controlplane

import (
	"context"
	"path"
	"strings"

	istiov1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/v1alpha3"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/helm/pkg/manifest"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type controlPlaneReconciler struct {
	*ReconcileControlPlane
	instance   *istiov1alpha3.ControlPlane
	status     *istiov1alpha3.ControlPlaneStatus
	ownerRefs  []metav1.OwnerReference
	renderings map[string][]manifest.Manifest
}

var seen = struct{}{}

func (r *controlPlaneReconciler) Reconcile() (reconcile.Result, error) {
	allErrors := []error{}
	var err error

	// prepare to write a new reconciliation status
	r.instance.Status.RemoveCondition(istiov1alpha3.ConditionTypeReconciled)
	// ensure ComponentStatus is ready
	if r.instance.Status.ComponentStatus == nil {
		r.instance.Status.ComponentStatus = []*istiov1alpha3.ComponentStatus{}
	}

	// Render the templates
	err = r.renderCharts()
	if err != nil {
		// we can't progress here
		updateReconcileStatus(&r.instance.Status.StatusType, err)
		r.Client.Status().Update(context.TODO(), r.instance)
		return reconcile.Result{}, err
	}

	// install istio

	// set the auto-injection flag
	// update injection label on namespace
	// XXX: this should probably only be done when installing a control plane
	// e.g. spec.pilot.enabled || spec.mixer.enabled || spec.galley.enabled || spec.sidecarInjectorWebhook.enabled || ....
	// which is all we're supporting atm.  if the scope expands to allow
	// installing custom gateways, etc., we should revisit this.
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: r.instance.Namespace}}
	err = r.Client.Get(context.TODO(), client.ObjectKey{Name: r.instance.Namespace}, namespace)
	if err == nil {
		if namespace.Labels == nil {
			namespace.Labels = map[string]string{}
		}
		if label, ok := namespace.Labels["istio.openshift.com/ignore-namespace"]; !ok || label != "ignore" {
			r.Log.Info("Adding istio.openshift.com/ignore-namespace=ignore label to Request.Namespace")
			namespace.Labels["istio.openshift.com/ignore-namespace"] = "ignore"
			err = r.Client.Update(context.TODO(), namespace)
		}
	} else {
		allErrors = append(allErrors, err)
	}

	// create components
	owner := metav1.NewControllerRef(r.instance, istiov1alpha3.SchemeGroupVersion.WithKind("ControlPlane"))
	r.ownerRefs = []metav1.OwnerReference{*owner}

	componentsProcessed := map[string]struct{}{}

	// create core istio resources
	componentsProcessed["istio"] = seen
	err = r.processComponentManifests("istio")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// create security
	componentsProcessed["istio/charts/security"] = seen
	err = r.processComponentManifests("istio/charts/security")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// create galley
	componentsProcessed["istio/charts/galley"] = seen
	err = r.processComponentManifests("istio/charts/galley")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// prometheus
	componentsProcessed["istio/charts/prometheus"] = seen
	err = r.processComponentManifests("istio/charts/prometheus")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// create mixer
	componentsProcessed["istio/charts/mixer"] = seen
	err = r.processComponentManifests("istio/charts/mixer")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// create pilot
	componentsProcessed["istio/charts/pilot"] = seen
	err = r.processComponentManifests("istio/charts/pilot")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// gateways
	componentsProcessed["istio/charts/gateways"] = seen
	err = r.processComponentManifests("istio/charts/gateways")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// sidecar injector
	componentsProcessed["istio/charts/sidecarInjectorWebhook"] = seen
	err = r.processComponentManifests("istio/charts/sidecarInjectorWebhook")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// install grafana
	componentsProcessed["istio/charts/grafana"] = seen
	err = r.processComponentManifests("istio/charts/grafana")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// install jaeger
	componentsProcessed["istio/charts/tracing"] = seen
	err = r.processComponentManifests("istio/charts/tracing")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// install kiali
	componentsProcessed["istio/charts/kiali"] = seen
	err = r.processComponentManifests("istio/charts/kiali")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// other components
	for key := range r.renderings {
		if !strings.HasPrefix(key, "istio/") {
			continue
		}
		if _, ok := componentsProcessed[key]; ok {
			// already processed this component
			continue
		}
		componentsProcessed[key] = seen
		err = r.processComponentManifests(key)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	// install 3scale
	componentsProcessed["maistra-threescale"] = seen
	err = r.processComponentManifests("maistra-threescale")
	if err != nil {
		allErrors = append(allErrors, err)
	}

	// delete unseen components
	for index := len(r.instance.Status.ComponentStatus) -1; index >= 0; index-- {
		status := r.instance.Status.ComponentStatus[index]
		if _, ok := componentsProcessed[status.Resource]; ok {
			continue
		}
		componentsProcessed[status.Resource] = seen
		err = r.processComponentManifests(status.Resource)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}

	r.status.ObservedGeneration = r.instance.GetGeneration()
	err = utilerrors.NewAggregate(allErrors)
	updateReconcileStatus(&r.status.StatusType, err)

	r.instance.Status = *r.status
	updateErr := r.Client.Status().Update(context.TODO(), r.instance)
	if updateErr != nil {
		r.Log.Error(err, "error updating ControlPlane status")
		if err == nil {
			// XXX: is this the right thing to do?
			return reconcile.Result{}, updateErr
		}
	}

	r.Log.Info("reconciliation complete")

	return reconcile.Result{}, err
}

func (r *controlPlaneReconciler) renderCharts() error {
	allErrors := []error{}
	var err error
	var threeScaleRenderings map[string][]manifest.Manifest

	r.Log.V(2).Info("rendering Istio charts")
	istioRenderings, _, err := RenderHelmChart(path.Join(ChartPath, "istio"), r.instance.GetNamespace(), r.instance.Spec.Istio)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	if isEnabled(r.instance.Spec.ThreeScale) {
		r.Log.V(2).Info("rendering 3scale charts")
		threeScaleRenderings, _, err = RenderHelmChart(path.Join(ChartPath, "maistra-threescale"), r.instance.GetNamespace(), r.instance.Spec.ThreeScale)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	} else {
		threeScaleRenderings = map[string][]manifest.Manifest{}
	}

	if len(allErrors) > 0 {
		return utilerrors.NewAggregate(allErrors)
	}

	// merge the rendernings
	r.renderings = map[string][]manifest.Manifest{}
	for key, value := range istioRenderings {
		r.renderings[key] = value
	}
	for key, value := range threeScaleRenderings {
		r.renderings[key] = value
	}
	return nil
}

func isEnabled(spec istiov1alpha3.HelmValuesType) bool {
	if enabledVal, ok := spec["enabled"]; ok {
		if enabled, ok := enabledVal.(bool); ok {
			return enabled
		}
	}
	return false
}
