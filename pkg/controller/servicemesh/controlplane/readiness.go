package controlplane

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (r *controlPlaneInstanceReconciler) UpdateReadiness() error {
	update, err := r.updateReadinessStatus()
	if update && !r.skipStatusUpdate() {
		statusErr := r.PostStatus()
		if statusErr != nil {
			// original error is more important than the status update error
			if err == nil {
				// if there's no original error, we can return the status update error
				return statusErr
			}
			// otherwise, we must log the status update error and return the original error
			r.Log.Error(statusErr, "Error updating status")
		}
	}
	return err
}

func (r *controlPlaneInstanceReconciler) updateReadinessStatus() (bool, error) {
	r.Log.Info("Updating ServiceMeshControlPlane readiness state")
	notReadyState, err := r.calculateNotReadyState()
	if err != nil {
		condition := v1.Condition{
			Type:    v1.ConditionTypeReady,
			Status:  v1.ConditionStatusUnknown,
			Reason:  v1.ConditionReasonProbeError,
			Message: fmt.Sprintf("Error collecting ready state: %s", err),
		}
		r.Status.SetCondition(condition)
		r.EventRecorder.Event(r.Instance, corev1.EventTypeWarning, eventReasonNotReady, condition.Message)
		return true, err
	}
	unreadyComponents := make([]string, 0, len(notReadyState))
	for component, notReady := range notReadyState {
		if notReady {
			r.Log.Info(fmt.Sprintf("%s resources are not fully available", component))
			unreadyComponents = append(unreadyComponents, component)
		}
	}
	readyCondition := r.Status.GetCondition(v1.ConditionTypeReady)
	updateStatus := false
	if len(unreadyComponents) > 0 {
		if readyCondition.Status != v1.ConditionStatusFalse {
			condition := v1.Condition{
				Type:    v1.ConditionTypeReady,
				Status:  v1.ConditionStatusFalse,
				Reason:  v1.ConditionReasonComponentsNotReady,
				Message: "Some components are not fully available",
			}
			r.Status.SetCondition(condition)
			r.EventRecorder.Event(r.Instance, corev1.EventTypeWarning, eventReasonNotReady, fmt.Sprintf("The following components are not fully available: %s", unreadyComponents))
			updateStatus = true
		}
	} else {
		if readyCondition.Status != v1.ConditionStatusTrue {
			condition := v1.Condition{
				Type:    v1.ConditionTypeReady,
				Status:  v1.ConditionStatusTrue,
				Reason:  v1.ConditionReasonComponentsReady,
				Message: "All component deployments are Available",
			}
			r.Status.SetCondition(condition)
			r.EventRecorder.Event(r.Instance, corev1.EventTypeNormal, eventReasonReady, condition.Message)
			updateStatus = true
		}
	}

	return updateStatus, nil
}

func (r *controlPlaneInstanceReconciler) calculateNotReadyState() (map[string]bool, error) {
	var cniNotReady bool
	notReadyState := map[string]bool{}
	err := r.calculateNotReadyStateForType(appsv1.SchemeGroupVersion.WithKind("Deployment"), notReadyState, r.deploymentReady)
	if err != nil {
		return notReadyState, err
	}
	err = r.calculateNotReadyStateForType(appsv1.SchemeGroupVersion.WithKind("StatefulSet"), notReadyState, r.statefulSetReady)
	if err != nil {
		return notReadyState, err
	}
	err = r.calculateNotReadyStateForType(appsv1.SchemeGroupVersion.WithKind("DaemonSet"), notReadyState, r.daemonSetReady)
	if err != nil {
		return notReadyState, err
	}
	cniNotReady, err = r.calculateNotReadyStateForCNI()
	notReadyState["cni"] = cniNotReady
	return notReadyState, err
}

func (r *controlPlaneInstanceReconciler) calculateNotReadyStateForCNI() (bool, error) {
	if !common.IsCNIEnabled {
		return false, nil
	}
	labelSelector := map[string]string{"istio": "cni"}
	daemonSets := &unstructured.UnstructuredList{}
	daemonSets.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
	operatorNamespace := common.GetOperatorNamespace()
	if err := r.Client.List(context.TODO(), client.MatchingLabels(labelSelector).InNamespace(operatorNamespace), daemonSets); err != nil {
		return true, err
	}
	for _, ds := range daemonSets.Items {
		if !r.daemonSetReady(&ds) {
			return true, nil
		}
	}
	return false, nil
}

func (r *controlPlaneInstanceReconciler) calculateNotReadyStateForType(gvk schema.GroupVersionKind, notReadyState map[string]bool, isReady func(*unstructured.Unstructured) bool) error {
	resources, err := common.FetchOwnedResources(r.Client, gvk, r.Instance.GetNamespace(), r.Instance.GetNamespace())
	if err != nil {
		return err
	}
	for _, resource := range resources.Items {
		if component, ok := common.GetLabel(&resource, common.KubernetesAppComponentKey); ok {
			notReadyState[component] = notReadyState[component] || !isReady(&resource)
		} else {
			// how do we have an owned resource with no component label?
			r.Log.Error(nil, "skipping resource for readiness check: resource has no component label", gvk.Kind, resource.GetName())
		}
	}
	return nil
}

func (r *controlPlaneInstanceReconciler) deploymentReady(deployment *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(deployment.UnstructuredContent(), "status", "conditions")
	if err != nil {
		r.Log.Error(err, "error reading Deployment.Status", "Deployment", deployment.GetName())
		return false
	}
	if !found {
		return false
	}

	for _, condition := range conditions {
		if conditionMap, ok := condition.(map[string]interface{}); ok {
			conditionType, _, _ := unstructured.NestedString(conditionMap, "type")
			if conditionType == "Available" {
				conditionStatus, _, _ := unstructured.NestedString(conditionMap, "status")
				return conditionStatus == "True"
			}
		} else {
			r.Log.Error(nil, "cannot convert Deployment condition")
		}
	}

	return false
}

func (r *controlPlaneInstanceReconciler) statefulSetReady(statefulSet *unstructured.Unstructured) bool {
	replicas, found, err := unstructured.NestedInt64(statefulSet.UnstructuredContent(), "status", "replicas")
	if err != nil {
		r.Log.Error(err, "error reading StatefulSet.Status", "StatefulSet", statefulSet.GetName())
		return false
	}
	if !found {
		return false
	}

	readyReplicas, found, err := unstructured.NestedInt64(statefulSet.UnstructuredContent(), "status", "readyReplicas")
	if err != nil {
		r.Log.Error(err, "error reading StatefulSet.Status", "StatefulSet", statefulSet.GetName())
		return false
	}
	if !found {
		return false
	}

	return readyReplicas >= replicas
}

func (r *controlPlaneInstanceReconciler) daemonSetReady(daemonSet *unstructured.Unstructured) bool {
	unavailable, found, err := unstructured.NestedInt64(daemonSet.UnstructuredContent(), "status", "numberUnavailable")
	if err != nil {
		r.Log.Error(err, "error reading DaemonSet.Status", "DaemonSet", daemonSet.GetName())
		return false
	}

	return !found || unavailable == 0
}
