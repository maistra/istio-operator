package controlplane

import (
	"context"
	"path"
	"strconv"
	"strings"

	"github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/helm/pkg/manifest"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ControlPlaneReconciler struct {
	*ReconcileControlPlane
	Instance       *v1.ServiceMeshControlPlane
	Status         *v1.ControlPlaneStatus
	NewOwnerRef    func(*v1.ServiceMeshControlPlane) *metav1.OwnerReference
	UpdateStatus   func() error
	ownerRefs      []metav1.OwnerReference
	meshGeneration string
	renderings     map[string][]manifest.Manifest
}

var seen = struct{}{}

func (r *ControlPlaneReconciler) Reconcile() (reconcile.Result, error) {
	allErrors := []error{}
	var err error

	// prepare to write a new reconciliation status
	r.Instance.Status.RemoveCondition(v1.ConditionTypeReconciled)
	// ensure ComponentStatus is ready
	if r.Instance.Status.ComponentStatus == nil {
		r.Instance.Status.ComponentStatus = []*v1.ComponentStatus{}
	}

	// Render the templates
	err = r.renderCharts()
	if err != nil {
		// we can't progress here
		updateReconcileStatus(&r.Instance.Status.StatusType, err)
		r.Client.Status().Update(context.TODO(), r.Instance)
		return reconcile.Result{}, err
	}

	// install istio

	// set the auto-injection flag
	// update injection label on namespace
	// XXX: this should probably only be done when installing a control plane
	// e.g. spec.pilot.enabled || spec.mixer.enabled || spec.galley.enabled || spec.sidecarInjectorWebhook.enabled || ....
	// which is all we're supporting atm.  if the scope expands to allow
	// installing custom gateways, etc., we should revisit this.
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: r.Instance.Namespace}}
	err = r.Client.Get(context.TODO(), client.ObjectKey{Name: r.Instance.Namespace}, namespace)
	if err == nil {
		if namespace.Labels == nil {
			namespace.Labels = map[string]string{}
		}
		if label, ok := namespace.Labels["maistra.io/ignore-namespace"]; !ok || label != "ignore" {
			r.Log.Info("Adding maistra.io/ignore-namespace=ignore label to Request.Namespace")
			namespace.Labels["maistra.io/ignore-namespace"] = "ignore"
			err = r.Client.Update(context.TODO(), namespace)
		}
	} else {
		allErrors = append(allErrors, err)
	}

	// initialize common data
	owner := r.NewOwnerRef(r.Instance)
	r.ownerRefs = []metav1.OwnerReference{*owner}
	r.meshGeneration = strconv.FormatInt(r.Instance.GetGeneration(), 10)

	// create components
	componentsProcessed := map[string]struct{}{}

	// these components have to be installed in the specified order
	orderedComponents := []string{
		"istio", // core istio resources
		"istio/charts/security",
		"istio/charts/prometheus",
		"istio/charts/tracing",
		"istio/charts/galley",
		"istio/charts/mixer",
		"istio/charts/pilot",
		"istio/charts/gateways",
		"istio/charts/sidecarInjectorWebhook",
		"istio/charts/grafana",
		"istio/charts/kiali",
	}
	for _, componentName := range orderedComponents {
		componentsProcessed[componentName] = seen
		err = r.processComponentManifests(componentName)
		if err != nil {
			allErrors = append(allErrors, err)
		}
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
	err = r.prune(r.Instance.GetGeneration())
	if err != nil {
		allErrors = append(allErrors, err)
	}

	r.Status.ObservedGeneration = r.Instance.GetGeneration()
	err = utilerrors.NewAggregate(allErrors)
	updateReconcileStatus(&r.Status.StatusType, err)

	r.Instance.Status = *r.Status
	updateErr := r.UpdateStatus()
	if updateErr != nil {
		r.Log.Error(err, "error updating ServiceMeshControlPlane status")
		if err == nil {
			// XXX: is this the right thing to do?
			return reconcile.Result{}, updateErr
		}
	}

	r.Log.Info("reconciliation complete")

	return reconcile.Result{}, err
}

func (r *ControlPlaneReconciler) renderCharts() error {
	allErrors := []error{}
	var err error
	var threeScaleRenderings map[string][]manifest.Manifest

	r.Log.V(2).Info("rendering Istio charts")
	istioRenderings, _, err := RenderHelmChart(path.Join(ChartPath, "istio"), r.Instance.GetNamespace(), r.Instance.Spec.Istio)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	if isEnabled(r.Instance.Spec.ThreeScale) {
		r.Log.V(2).Info("rendering 3scale charts")
		threeScaleRenderings, _, err = RenderHelmChart(path.Join(ChartPath, "maistra-threescale"), r.Instance.GetNamespace(), r.Instance.Spec.ThreeScale)
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

func isEnabled(spec v1.HelmValuesType) bool {
	if enabledVal, ok := spec["enabled"]; ok {
		if enabled, ok := enabledVal.(bool); ok {
			return enabled
		}
	}
	return false
}
