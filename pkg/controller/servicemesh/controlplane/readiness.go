package controlplane

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const statusAnnotationReadyComponentCount = "readyComponentCount"

func (r *controlPlaneInstanceReconciler) UpdateReadiness(ctx context.Context) error {
	log := common.LogFromContext(ctx)
	update, err := r.updateReadinessStatus(ctx)
	if update && !r.skipStatusUpdate() {
		statusErr := r.PostStatus(ctx)
		if statusErr != nil {
			// original error is more important than the status update error
			if err == nil {
				// if there's no original error, we can return the status update error
				return statusErr
			}
			// otherwise, we must log the status update error and return the original error
			log.Error(statusErr, "Error updating status")
		}
	}
	return err
}

func (r *controlPlaneInstanceReconciler) updateReadinessStatus(ctx context.Context) (bool, error) {
	log := common.LogFromContext(ctx)
	log.Info("Updating ServiceMeshControlPlane readiness state")
	readinessMap, err := r.calculateComponentReadiness(ctx)
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

	readyComponents := sets.NewString()
	unreadyComponents := sets.NewString()
	for component, ready := range readinessMap {
		if ready {
			readyComponents.Insert(component)
		} else {
			log.Info(fmt.Sprintf("%s resources are not fully available", component))
			unreadyComponents.Insert(component)
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

	if r.Status.Annotations == nil {
		r.Status.Annotations = map[string]string{}
	}
	r.Status.Annotations[statusAnnotationReadyComponentCount] = fmt.Sprintf("%d/%d", len(readyComponents), len(readinessMap))

	return updateStatus, nil
}

func (r *controlPlaneInstanceReconciler) calculateComponentReadiness(ctx context.Context) (map[string]bool, error) {
	readinessMap := map[string]bool{}
	typesToCheck := map[schema.GroupVersionKind]func(context.Context, *unstructured.Unstructured) bool{
		appsv1.SchemeGroupVersion.WithKind("Deployment"):  r.deploymentReady,
		appsv1.SchemeGroupVersion.WithKind("StatefulSet"): r.statefulSetReady,
		appsv1.SchemeGroupVersion.WithKind("DaemonSet"):   r.daemonSetReady,
	}
	for gvk, isReadyFunc := range typesToCheck {
		err := r.calculateReadinessForType(ctx, gvk, readinessMap, isReadyFunc)
		if err != nil {
			return readinessMap, err
		}
	}

	cniReady, err := r.isCNIReady(ctx)
	readinessMap["cni"] = cniReady
	return readinessMap, err
}

func (r *controlPlaneInstanceReconciler) isCNIReady(ctx context.Context) (bool, error) {
	if !r.cniConfig.Enabled {
		return true, nil
	}
	labelSelector := map[string]string{"istio": "cni"}
	daemonSets := &unstructured.UnstructuredList{}
	daemonSets.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
	operatorNamespace := common.GetOperatorNamespace()
	if err := r.Client.List(ctx, client.MatchingLabels(labelSelector).InNamespace(operatorNamespace), daemonSets); err != nil {
		return false, err
	}
	for _, ds := range daemonSets.Items {
		if !r.daemonSetReady(ctx, &ds) {
			return false, nil
		}
	}
	return true, nil
}

func (r *controlPlaneInstanceReconciler) calculateReadinessForType(ctx context.Context, gvk schema.GroupVersionKind, readinessMap map[string]bool, isReady func(context.Context, *unstructured.Unstructured) bool) error {
	log := common.LogFromContext(ctx)
	resources, err := common.FetchOwnedResources(ctx, r.Client, gvk, r.Instance.GetNamespace(), r.Instance.GetNamespace())
	if err != nil {
		return err
	}
	for _, resource := range resources.Items {
		if component, ok := common.GetLabel(&resource, common.KubernetesAppComponentKey); ok {
			componentReady := isReady(ctx, &resource)
			if ready, exists := readinessMap[component]; exists {
				readinessMap[component] = ready && componentReady
			} else {
				readinessMap[component] = componentReady
			}
		} else {
			// how do we have an owned resource with no component label?
			log.Error(nil, "skipping resource for readiness check: resource has no component label", gvk.Kind, resource.GetName())
		}
	}
	return nil
}

func (r *controlPlaneInstanceReconciler) deploymentReady(ctx context.Context, deployment *unstructured.Unstructured) bool {
	log := common.LogFromContext(ctx)
	conditions, found, err := unstructured.NestedSlice(deployment.UnstructuredContent(), "status", "conditions")
	if err != nil {
		log.Error(err, "error reading Deployment.Status", "Deployment", deployment.GetName())
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
			log.Error(nil, "cannot convert Deployment condition")
		}
	}

	return false
}

func (r *controlPlaneInstanceReconciler) statefulSetReady(ctx context.Context, statefulSet *unstructured.Unstructured) bool {
	log := common.LogFromContext(ctx)
	replicas, found, err := unstructured.NestedInt64(statefulSet.UnstructuredContent(), "status", "replicas")
	if err != nil {
		log.Error(err, "error reading StatefulSet.Status", "StatefulSet", statefulSet.GetName())
		return false
	}
	if !found {
		return false
	}

	readyReplicas, found, err := unstructured.NestedInt64(statefulSet.UnstructuredContent(), "status", "readyReplicas")
	if err != nil {
		log.Error(err, "error reading StatefulSet.Status", "StatefulSet", statefulSet.GetName())
		return false
	}
	if !found {
		return false
	}

	return readyReplicas >= replicas
}

func (r *controlPlaneInstanceReconciler) daemonSetReady(ctx context.Context, daemonSet *unstructured.Unstructured) bool {
	log := common.LogFromContext(ctx)
	unavailable, found, err := unstructured.NestedInt64(daemonSet.UnstructuredContent(), "status", "numberUnavailable")
	if err != nil {
		log.Error(err, "error reading DaemonSet.Status", "DaemonSet", daemonSet.GetName())
		return false
	}

	return !found || unavailable == 0
}
