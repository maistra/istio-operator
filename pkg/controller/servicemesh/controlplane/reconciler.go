package controlplane

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/bootstrap"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/hacks"

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

const (
	smcpDefaultTemplate = "default"

	// Event reasons
	eventReasonInstalling              = "Installing"
	eventReasonPausingInstall          = "PausingInstall"
	eventReasonPausingUpdate           = "PausingUpdate"
	eventReasonInstalled               = "Installed"
	eventReasonUpdating                = "Updating"
	eventReasonUpdated                 = "Updated"
	eventReasonDeleting                = "Deleting"
	eventReasonDeleted                 = "Deleted"
	eventReasonPruning                 = "Pruning"
	eventReasonFailedRemovingFinalizer = "FailedRemovingFinalizer"
	eventReasonFailedDeletingResources = "FailedDeletingResources"
	eventReasonNotReady                = "NotReady"
	eventReasonReady                   = "Ready"
)

func (r *ControlPlaneReconciler) Reconcile() (result reconcile.Result, err error) {
	if r.Status.GetCondition(v1.ConditionTypeReconciled).Status != v1.ConditionStatusFalse {
		r.initializeReconcileStatus()
		err := r.PostStatus()
		return reconcile.Result{}, err // ensure that the new reconcile status is posted immediately. Reconciliation will resume when the status update comes back into the operator
	}

	var ready bool
	// make sure status gets updated on exit
	reconciledCondition := r.Status.GetCondition(v1.ConditionTypeReconciled)
	reconciliationMessage := reconciledCondition.Message
	reconciliationReason := reconciledCondition.Reason
	reconciliationComplete := false
	defer func() {
		// this ensures we're updating status (if necessary) and recording events on exit
		if statusErr := r.postReconciliationStatus(reconciliationReason, reconciliationMessage, err); statusErr != nil {
			if err == nil {
				err = statusErr
			} else {
				r.Log.Error(statusErr, "Error posting reconciliation status")
			}
		}
		if reconciliationComplete {
			hacks.ReduceLikelihoodOfRepeatedReconciliation()
		}
	}()

	if r.renderings == nil {
		// error handling
		defer func() {
			if err != nil {
				r.renderings = nil
				r.lastComponent = ""
				updateReconcileStatus(&r.Status.StatusType, err)
			}
		}()

		// Render the templates
		err = r.renderCharts()
		if err != nil {
			// we can't progress here
			reconciliationReason = v1.ConditionReasonReconcileError
			reconciliationMessage = "Error rendering helm charts"
			err = errors.Wrap(err, reconciliationMessage)
			return
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
			reconciliationReason = v1.ConditionReasonReconcileError
			reconciliationMessage = "Error updating labels on mesh namespace"
			err = errors.Wrap(err, reconciliationMessage)
			return
		}

		// initialize new Status
		componentStatuses := make([]*v1.ComponentStatus, 0, len(r.Status.ComponentStatus))
		for chartName := range r.renderings {
			componentName := componentFromChartName(chartName)
			componentStatus := r.Status.FindComponentByName(componentName)
			if componentStatus == nil {
				componentStatus = v1.NewComponentStatus()
				componentStatus.Resource = componentName
			}
			componentStatus.SetCondition(v1.Condition{
				Type:   v1.ConditionTypeReconciled,
				Status: v1.ConditionStatusFalse,
			})
			componentStatuses = append(componentStatuses, componentStatus)
		}
		r.Status.ComponentStatus = componentStatuses

		// initialize common data
		owner := metav1.NewControllerRef(r.Instance, v1.SchemeGroupVersion.WithKind("ServiceMeshControlPlane"))
		r.ownerRefs = []metav1.OwnerReference{*owner}
		r.meshGeneration = common.ReconciledVersion(r.Instance.GetGeneration())

		// Ensure CRDs are installed
		if err = bootstrap.InstallCRDs(r.Manager); err != nil {
			reconciliationReason = v1.ConditionReasonReconcileError
			reconciliationMessage = "Failed to install/update Istio CRDs"
			r.Log.Error(err, reconciliationMessage)
			return
		}

		// Ensure Istio CNI is installed
		if common.IsCNIEnabled {
			r.lastComponent = "cni"
			if err = bootstrap.InstallCNI(r.Manager); err != nil {
				reconciliationReason = v1.ConditionReasonReconcileError
				reconciliationMessage = "Failed to install/update Istio CNI"
				r.Log.Error(err, reconciliationMessage)
				return
			} else if notReady, _ := r.calculateNotReadyStateForCNI(); notReady {
				reconciliationReason = v1.ConditionReasonPausingInstall
				reconciliationMessage = fmt.Sprintf("Paused until %s becomes ready", "cni")
				return
			}
		}
	} else if r.lastComponent != "" {
		if notReadyMap, readinessErr := r.calculateNotReadyState(); readinessErr == nil {
			// if we've already begun reconciling, make sure we weren't waiting for
			// the last component to become ready
			if notReady, ok := notReadyMap[r.lastComponent]; ok && notReady {
				// last component has not become ready yet
				r.Log.Info(fmt.Sprintf("Paused until %s becomes ready", r.lastComponent))
				return
			}
		} else {
			// error calculating readiness
			reconciliationReason = v1.ConditionReasonProbeError
			reconciliationMessage = fmt.Sprintf("Error checking readiness of component %s", r.lastComponent)
			err = errors.Wrap(readinessErr, reconciliationMessage)
			r.Log.Error(err, reconciliationMessage)
			return
		}
	}

	// create components
	for _, chartName := range orderedCharts {
		if ready, err = r.processComponentManifests(chartName); !ready {
			reconciliationReason, reconciliationMessage, err = r.pauseReconciliation(chartName, err)
			return
		}
	}

	// any other istio components
	for key := range r.renderings {
		if !strings.HasPrefix(key, "istio/") {
			continue
		}
		if ready, err = r.processComponentManifests(key); !ready {
			reconciliationReason, reconciliationMessage, err = r.pauseReconciliation(key, err)
			return
		}
	}

	// install 3scale and any other components
	for key := range r.renderings {
		if ready, err = r.processComponentManifests(key); !ready {
			reconciliationReason, reconciliationMessage, err = r.pauseReconciliation(key, err)
			return
		}
	}

	// we still need to prune if this is the first generation, e.g. if the operator was updated during the install,
	// it's possible that some resources in the original version may not be present in the new version.
	// delete unseen components
	reconciliationMessage = "Pruning obsolete resources"
	r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeNormal, eventReasonPruning, reconciliationMessage)
	r.Log.Info(reconciliationMessage)
	err = r.prune(r.meshGeneration)
	if err != nil {
		reconciliationReason = v1.ConditionReasonReconcileError
		reconciliationMessage = "Error pruning obsolete resources"
		err = errors.Wrap(err, reconciliationMessage)
		return
	}

	if r.isUpdating() {
		reconciliationReason = v1.ConditionReasonUpdateSuccessful
		reconciliationMessage = fmt.Sprintf("Successfully updated from version %s to version %s", r.Status.GetReconciledVersion(), r.meshGeneration)
		r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeNormal, eventReasonUpdated, reconciliationMessage)
	} else {
		reconciliationReason = v1.ConditionReasonInstallSuccessful
		reconciliationMessage = fmt.Sprintf("Successfully installed version %s", r.meshGeneration)
		r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeNormal, eventReasonInstalled, reconciliationMessage)
	}
	r.Status.ObservedGeneration = r.Instance.GetGeneration()
	r.Status.ReconciledVersion = r.meshGeneration
	updateReconcileStatus(&r.Status.StatusType, nil)

	_, err = r.updateReadinessStatus() // this only updates the local object instance; it doesn't post the status update; postReconciliationStatus (called using defer) actually does that

	reconciliationComplete = true
	r.Log.Info("Completed ServiceMeshControlPlane reconcilation")
	return
}

func (r *ControlPlaneReconciler) pauseReconciliation(chartName string, err error) (v1.ConditionReason, string, error) {
	var eventReason string
	var conditionReason v1.ConditionReason
	var reconciliationMessage string
	if r.isUpdating() {
		eventReason = eventReasonPausingUpdate
		conditionReason = v1.ConditionReasonPausingUpdate
	} else {
		eventReason = eventReasonPausingInstall
		conditionReason = v1.ConditionReasonPausingInstall
	}
	componentName := componentFromChartName(chartName)
	if err == nil {
		reconciliationMessage = fmt.Sprintf("Paused until %s becomes ready", componentName)
		r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeNormal, eventReason, reconciliationMessage)
		r.Log.Info(reconciliationMessage)
	} else {
		conditionReason = v1.ConditionReasonReconcileError
		reconciliationMessage = fmt.Sprintf("Error processing component %s", componentName)
		r.Log.Error(err, reconciliationMessage)
	}
	return conditionReason, reconciliationMessage, errors.Wrapf(err, reconciliationMessage)
}

func (r *ControlPlaneReconciler) isUpdating() bool {
	return r.Instance.Status.ObservedGeneration != 0
}

// mergeValues merges a map containing input values on top of a map containing
// base values, giving preference to the base values for conflicts
func mergeValues(base map[string]interface{}, input map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = make(map[string]interface{}, 1)
	}

	for key, value := range input {
		//if the key doesn't already exist, add it
		if _, exists := base[key]; !exists {
			base[key] = value
			continue
		}

		// at this point, key exists in both input and base.
		// If both are maps, recurse.
		// If only input is a map, ignore it. We don't want to overrwrite base.
		// If both are values, again, ignore it since we don't want to overrwrite base.
		if baseKeyAsMap, baseOK := base[key].(map[string]interface{}); baseOK {
			if inputAsMap, inputOK := value.(map[string]interface{}); inputOK {
				base[key] = mergeValues(baseKeyAsMap, inputAsMap)
			}
		}
	}
	return base
}

func (r *ControlPlaneReconciler) getSMCPTemplate(name string) (v1.ControlPlaneSpec, error) {
	if strings.Contains(name, "/") {
		return v1.ControlPlaneSpec{}, fmt.Errorf("template name contains invalid character '/'")
	}

	templateContent, err := ioutil.ReadFile(path.Join(common.GetTemplatesDir(), name))
	if err != nil {
		//if we can't read from the user template path, try from the default path
		//we use two paths because Kubernetes will not auto-update volume mounted
		//configmaps mounted in directories with pre-existing content
		defaultTemplateContent, defaultErr := ioutil.ReadFile(path.Join(common.GetDefaultTemplatesDir(), name))
		if defaultErr != nil {
			return v1.ControlPlaneSpec{}, fmt.Errorf("template cannot be loaded from user or default directory. Error from user: %s. Error from default: %s", err, defaultErr)
		}
		templateContent = defaultTemplateContent
	}

	var template v1.ServiceMeshControlPlane
	if err = yaml.Unmarshal(templateContent, &template); err != nil {
		return v1.ControlPlaneSpec{}, fmt.Errorf("failed to parse template %s contents: %s", name, err)
	}
	return template.Spec, nil
}

//renderSMCPTemplates traverses and processes all of the references templates
func (r *ControlPlaneReconciler) recursivelyApplyTemplates(smcp v1.ControlPlaneSpec, visited map[string]struct{}) (v1.ControlPlaneSpec, error) {
	if smcp.Template == "" {
		return smcp, nil
	}
	r.Log.Info(fmt.Sprintf("processing smcp template %s", smcp.Template))

	if _, ok := visited[smcp.Template]; ok {
		return smcp, fmt.Errorf("SMCP templates form cyclic dependency. Cannot proceed")
	}

	template, err := r.getSMCPTemplate(smcp.Template)
	if err != nil {
		return smcp, err
	}

	template, err = r.recursivelyApplyTemplates(template, visited)
	if err != nil {
		r.Log.Info(fmt.Sprintf("error rendering SMCP templates: %s\n", err))
		return smcp, err
	}

	visited[smcp.Template] = struct{}{}

	smcp.Istio = mergeValues(smcp.Istio, template.Istio)
	smcp.ThreeScale = mergeValues(smcp.ThreeScale, template.ThreeScale)
	return smcp, nil
}

func (r *ControlPlaneReconciler) applyTemplates(smcpSpec v1.ControlPlaneSpec) (v1.ControlPlaneSpec, error) {
	r.Log.Info("updating servicemeshcontrolplane with templates")
	if smcpSpec.Template == "" {
		smcpSpec.Template = smcpDefaultTemplate
		r.Log.Info("No template provided. Using default")
	}

	spec, err := r.recursivelyApplyTemplates(smcpSpec, make(map[string]struct{}, 0))
	r.Log.Info(fmt.Sprintf("finished updating ServiceMeshControlPlane: %+v", spec))

	return spec, err
}

func (r *ControlPlaneReconciler) validateSMCPSpec(spec v1.ControlPlaneSpec) error {
	if spec.Istio == nil {
		return fmt.Errorf("ServiceMeshControlPlane missing Istio section")
	}

	if _, ok := spec.Istio["global"].(map[string]interface{}); !ok {
		return fmt.Errorf("ServiceMeshControlPlane missing global section")
	}
	return nil
}

func (r *ControlPlaneReconciler) renderCharts() error {
	//Generate the spec
	r.Status.LastAppliedConfiguration = r.Instance.Spec

	spec, err := r.applyTemplates(r.Status.LastAppliedConfiguration)
	if err != nil {
		r.Log.Error(err, "warning: failed to apply ServiceMeshControlPlane templates")

		return err
	}
	r.Status.LastAppliedConfiguration = spec

	if err := r.validateSMCPSpec(r.Status.LastAppliedConfiguration); err != nil {
		return err
	}

	if globalValues, ok := r.Status.LastAppliedConfiguration.Istio["global"].(map[string]interface{}); ok {
		globalValues["operatorNamespace"] = r.OperatorNamespace
	}

	var CNIValues map[string]interface{}
	var ok bool
	if CNIValues, ok = r.Status.LastAppliedConfiguration.Istio["istio_cni"].(map[string]interface{}); !ok {
		CNIValues = make(map[string]interface{})
		r.Status.LastAppliedConfiguration.Istio["istio_cni"] = CNIValues
	}
	CNIValues["enabled"] = common.IsCNIEnabled

	//Render the charts
	allErrors := []error{}
	var threeScaleRenderings map[string][]manifest.Manifest
	r.Log.Info("rendering helm charts")
	r.Log.V(2).Info("rendering Istio charts")
	istioRenderings, _, err := common.RenderHelmChart(path.Join(common.GetHelmDir(), "istio"), r.Instance.GetNamespace(), r.Status.LastAppliedConfiguration.Istio)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	if isEnabled(r.Instance.Spec.ThreeScale) {
		r.Log.V(2).Info("rendering 3scale charts")
		threeScaleRenderings, _, err = common.RenderHelmChart(path.Join(common.GetHelmDir(), "maistra-threescale"), r.Instance.GetNamespace(), r.Status.LastAppliedConfiguration.ThreeScale)
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
	instance := &v1.ServiceMeshControlPlane{}
	r.Log.Info("Posting status update", "conditions", r.Status.Conditions)
	if err := r.Client.Get(context.TODO(), client.ObjectKey{Name: r.Instance.Name, Namespace: r.Instance.Namespace}, instance); err == nil {
		instance.Status = *r.Status.DeepCopy()
		if err = r.Client.Status().Update(context.TODO(), instance); err != nil && !(apierrors.IsGone(err) || apierrors.IsNotFound(err)) {
			return errors.Wrap(err, "error updating ServiceMeshControlPlane status")
		}
	} else if !(apierrors.IsGone(err) || apierrors.IsNotFound(err)) {
		return errors.Wrap(err, "error getting ServiceMeshControlPlane prior to updating status")
	}

	return nil
}

func (r *ControlPlaneReconciler) postReconciliationStatus(reconciliationReason v1.ConditionReason, reconciliationMessage string, processingErr error) error {
	var reason string
	if r.isUpdating() {
		reason = eventReasonUpdating
	} else {
		reason = eventReasonInstalling
	}
	reconciledCondition := r.Status.GetCondition(v1.ConditionTypeReconciled)
	reconciledCondition.Reason = reconciliationReason
	if processingErr == nil {
		reconciledCondition.Message = reconciliationMessage
	} else {
		// grab the cause, as it's likely the error includes the reconciliation message
		reconciledCondition.Message = fmt.Sprintf("%s: error: %s", reconciliationMessage, errors.Cause(processingErr))
		r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeWarning, reason, reconciledCondition.Message)
	}
	r.Status.SetCondition(reconciledCondition)

	// we should only post status updates if condition status has changed
	if r.skipStatusUpdate() {
		return nil
	}

	return r.PostStatus()
}

func (r *ControlPlaneReconciler) skipStatusUpdate() bool {
	for _, conditionType := range []v1.ConditionType{v1.ConditionTypeInstalled, v1.ConditionTypeReconciled, v1.ConditionTypeReady} {
		if r.Status.GetCondition(conditionType).Status != r.Instance.Status.GetCondition(conditionType).Status {
			return false
		}
	}
	return true
}

func (r *ControlPlaneReconciler) initializeReconcileStatus() {
	var readyMessage string
	var eventReason string
	var conditionReason v1.ConditionReason
	if r.isUpdating() {
		readyMessage = fmt.Sprintf("Updating mesh from generation %s to generation %s", r.Status.GetReconciledVersion(), common.ReconciledVersion(r.Instance.GetGeneration()))
		eventReason = eventReasonUpdating
		conditionReason = v1.ConditionReasonSpecUpdated
	} else {
		readyMessage = fmt.Sprintf("Installing mesh generation %s", common.ReconciledVersion(r.Instance.GetGeneration()))
		eventReason = eventReasonInstalling
		conditionReason = v1.ConditionReasonResourceCreated

		r.Status.SetCondition(v1.Condition{
			Type:    v1.ConditionTypeInstalled,
			Status:  v1.ConditionStatusFalse,
			Reason:  conditionReason,
			Message: readyMessage,
		})
	}
	r.Manager.GetRecorder(controllerName).Event(r.Instance, corev1.EventTypeNormal, eventReason, readyMessage)
	r.Status.SetCondition(v1.Condition{
		Type:    v1.ConditionTypeReconciled,
		Status:  v1.ConditionStatusFalse,
		Reason:  conditionReason,
		Message: readyMessage,
	})
	r.Status.SetCondition(v1.Condition{
		Type:    v1.ConditionTypeReady,
		Status:  v1.ConditionStatusFalse,
		Reason:  conditionReason,
		Message: readyMessage,
	})
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
