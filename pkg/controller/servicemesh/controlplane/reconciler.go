package controlplane

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	ownerRefs      []metav1.OwnerReference
	meshGeneration string
	renderings     map[string][]manifest.Manifest
	lastComponent  string
}

// these components have to be installed in the specified order
var orderedCharts = []string{
	"istio", // core istio resources
	"istio/charts/istio_cni",
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

func (r *ControlPlaneReconciler) Reconcile() (reconcile.Result, error) {
	var err error
	var ready bool

	if r.renderings == nil {
		// Render the templates
		err = r.renderCharts()
		if err != nil {
			// we can't progress here
			err = errors.Wrap(err, "unexpected error rendering helm charts")
			updateReconcileStatus(&r.Instance.Status.StatusType, err)
			r.PostStatus()
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
			updateLabels := false
			if namespace.Labels == nil {
				namespace.Labels = map[string]string{}
			}
			// make sure injection is disabled for the control plane
			if label, ok := namespace.Labels["maistra.io/ignore-namespace"]; !ok || label != "ignore" {
				r.Log.Info("Adding maistra.io/ignore-namespace=ignore label to Request.Namespace")
				namespace.Labels["maistra.io/ignore-namespace"] = "ignore"
				updateLabels = true
			}
			// make sure the member-of label is specified, so networking works correctly
			if label, ok := namespace.Labels[common.MemberOfKey]; !ok || label != namespace.GetName() {
				r.Log.Info(fmt.Sprintf("Adding %s label to Request.Namespace", common.MemberOfKey))
				namespace.Labels[common.MemberOfKey] = namespace.GetName()
				updateLabels = true
			}
			if updateLabels {
				err = r.Client.Update(context.TODO(), namespace)
			}
		}
		if err != nil {
			// bail if there was an error updating the namespace
			err = errors.Wrap(err, "unexpected error updating labels on mesh namespace")
			r.renderings = nil
			return reconcile.Result{}, err
		}

		// initialize new Status
		r.Status.StatusType = r.Instance.Status.StatusType
		for chartName := range r.renderings {
			componentName := componentFromChartName(chartName)
			componentStatus := r.Instance.Status.FindComponentByName(componentName)
			if componentStatus == nil {
				componentStatus = v1.NewComponentStatus()
				componentStatus.Resource = componentName
			}
			componentStatus.SetCondition(v1.Condition{
				Type:   v1.ConditionTypeReconciled,
				Status: v1.ConditionStatusFalse,
			})
			r.Status.ComponentStatus = append(r.Status.ComponentStatus, componentStatus)
		}

		// initialize common data
		owner := metav1.NewControllerRef(r.Instance, v1.SchemeGroupVersion.WithKind("ServiceMeshControlPlane"))
		r.ownerRefs = []metav1.OwnerReference{*owner}
		r.meshGeneration = strconv.FormatInt(r.Instance.GetGeneration(), 10)
	} else if notReadyMap, readinessErr := r.calculateNotReadyState(); readinessErr == nil {
		// if we've already begun reconciling, make sure we weren't waiting for
		// the last component to become ready
		if notReady, ok := notReadyMap[r.lastComponent]; ok && notReady {
			// last component has not become ready yet
			r.Log.Info(fmt.Sprintf("Paused until %s becomes ready", r.lastComponent))
			return reconcile.Result{}, nil
		}
	} else {
		// error calculating readiness
		readinessErr = errors.Wrapf(readinessErr, "unexpected error checking readiness of component %s", r.lastComponent)
		r.Log.Error(readinessErr, "")
		return reconcile.Result{}, readinessErr
	}

	// Update reconcile status
	reconcileCondition := r.Instance.Status.GetCondition(v1.ConditionTypeReconciled)
	reconcileCondition.Message = "Progressing"
	r.Instance.Status.SetCondition(reconcileCondition)
	r.PostStatus()

	// make sure status gets updated on exit
	var reconciliationMessage string
	defer func() {
		r.postReconciliationStatus(reconciliationMessage, err)
	}()

	// create components
	for _, chartName := range orderedCharts {
		if ready, err = r.processComponentManifests(chartName); !ready {
			componentName := componentFromChartName(chartName)
			reconciliationMessage = fmt.Sprintf("Paused until %s becomes ready", componentName)
			r.Log.Info(reconciliationMessage)
			err = errors.Wrapf(err, "unexpected error processing component %s", componentName)
			return reconcile.Result{}, err
		}
	}

	// any other istio components
	for key := range r.renderings {
		if !strings.HasPrefix(key, "istio/") {
			continue
		}
		if ready, err = r.processComponentManifests(key); !ready {
			componentName := componentFromChartName(key)
			reconciliationMessage = fmt.Sprintf("Paused until %s becomes ready", componentName)
			r.Log.Info(reconciliationMessage)
			err = errors.Wrapf(err, "unexpected error processing component %s", componentName)
			return reconcile.Result{}, err
		}
	}

	// install 3scale and any other components
	for key := range r.renderings {
		if ready, err = r.processComponentManifests(key); !ready {
			componentName := componentFromChartName(key)
			reconciliationMessage = fmt.Sprintf("Paused until %s becomes ready", componentName)
			r.Log.Info(reconciliationMessage)
			err = errors.Wrapf(err, "unexpected error processing component %s", componentName)
			return reconcile.Result{}, err
		}
	}

	reconciliationMessage = "Pruning old resources"
	// delete unseen components
	err = r.prune(r.Instance.GetGeneration())
	if err != nil {
		err = errors.Wrap(err, "unexpected error pruning old resources")
		return reconcile.Result{}, err
	}

	if r.Status.ObservedGeneration == 0 {
		reconciliationMessage = fmt.Sprintf("Successfully installed generation %d", r.Instance.GetGeneration())
		r.Manager.GetRecorder(controllerName).Event(r.Instance, "Normal", "ServiceMeshInstalled", reconciliationMessage)
	} else {
		reconciliationMessage = fmt.Sprintf("Successfully updated from generation %d to generation %d", r.Instance.Status.ObservedGeneration, r.Instance.GetGeneration())
		r.Manager.GetRecorder(controllerName).Event(r.Instance, "Normal", "ServiceMeshUpdated", reconciliationMessage)
	}
	r.Status.ObservedGeneration = r.Instance.GetGeneration()
	updateReconcileStatus(&r.Status.StatusType, nil)

	r.Log.Info("Completed ServiceMeshControlPlane reconcilation")
	// requeue to ensure readiness is updated
	return reconcile.Result{Requeue: true}, nil
}

func (r *ControlPlaneReconciler) renderCharts() error {
	r.Log.Info("rendering helm charts")
	allErrors := []error{}
	var err error
	var threeScaleRenderings map[string][]manifest.Manifest

	if globalValues, ok := r.Instance.Spec.Istio["global"].(map[string]interface{}); ok {
		globalValues["operatorNamespace"] = r.OperatorNamespace
	} else {
		return fmt.Errorf("Could not set operatorNamespace value, as .Values.global is not a map[string]interface{}: %v", err)
	}

	var CNIValues map[string]interface{}
	var ok bool
	if CNIValues, ok = r.Instance.Spec.Istio["istio_cni"].(map[string]interface{}); !ok {
		CNIValues = make(map[string]interface{})
		r.Instance.Spec.Istio["istio_cni"] = CNIValues
	}
	CNIValues["enabled"] = common.IsCNIEnabled

	r.Log.V(2).Info("rendering Istio charts")
	istioRenderings, _, err := common.RenderHelmChart(path.Join(common.ChartPath, "istio"), r.Instance.GetNamespace(), r.Instance.Spec.Istio)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	if isEnabled(r.Instance.Spec.ThreeScale) {
		r.Log.V(2).Info("rendering 3scale charts")
		threeScaleRenderings, _, err = common.RenderHelmChart(path.Join(common.ChartPath, "maistra-threescale"), r.Instance.GetNamespace(), r.Instance.Spec.ThreeScale)
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

func (r *ControlPlaneReconciler) PostStatus() error {
	return r.Client.Status().Update(context.TODO(), r.Instance)
}

func (r *ControlPlaneReconciler) postReconciliationStatus(reconciliationMessage string, processingErr error) {
	reconciledCondition := r.Status.GetCondition(v1.ConditionTypeReconciled)
	if processingErr == nil {
		reconciledCondition.Message = reconciliationMessage
	} else {
		reconciledCondition.Message = fmt.Sprintf("reconciliation error: %s", processingErr)
	}
	r.Status.SetCondition(reconciledCondition)
	instance := &v1.ServiceMeshControlPlane{}
	if updateErr := r.Client.Get(context.TODO(), client.ObjectKey{Name: r.Instance.Name, Namespace: r.Instance.Namespace}, instance); updateErr == nil {
		instance.Status = *r.Status
		r.Instance = instance
		if updateErr = r.PostStatus(); updateErr != nil && !(apierrors.IsGone(updateErr) || apierrors.IsNotFound(updateErr)) {
			r.Log.Error(updateErr, "error updating ServiceMeshControlPlane status")
		}
	} else if !(apierrors.IsGone(updateErr) || apierrors.IsNotFound(updateErr)) {
		r.Log.Error(updateErr, "error updating ServiceMeshControlPlane status")
	}
}

func componentFromChartName(chartName string) string {
	_, componentName := path.Split(chartName)
	return componentName
}

func isEnabled(spec v1.HelmValuesType) bool {
	if enabledVal, ok := spec["enabled"]; ok {
		if enabled, ok := enabledVal.(bool); ok {
			return enabled
		}
	}
	return false
}
