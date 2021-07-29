package controlplane

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	"github.com/maistra/istio-operator/pkg/controller/common"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const statusAnnotationReadyComponentCount = "readyComponentCount"
const statusAnnotationAlwaysReadyComponents = "alwaysReadyComponents"

func (r *controlPlaneInstanceReconciler) UpdateReadiness(ctx context.Context) error {
	log := common.LogFromContext(ctx)
	update, err := r.updateReadinessStatus(ctx)
	if update {
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
	readyComponents, unreadyComponents, err := r.calculateComponentReadiness(ctx)
	if err != nil {
		condition := status.Condition{
			Type:    status.ConditionTypeReady,
			Status:  status.ConditionStatusUnknown,
			Reason:  status.ConditionReasonProbeError,
			Message: fmt.Sprintf("Error collecting ready state: %s", err),
		}
		r.Status.SetCondition(condition)
		r.EventRecorder.Event(r.Instance, corev1.EventTypeWarning, eventReasonNotReady, condition.Message)
		return true, err
	}

	readyCondition := r.Status.GetCondition(status.ConditionTypeReady)
	updateStatus := false
	reconciledCondition := r.Status.GetCondition(status.ConditionTypeReconciled)
	if reconciledCondition.Status != status.ConditionStatusTrue {
		if !readyCondition.Matches(reconciledCondition.Status, reconciledCondition.Reason, reconciledCondition.Message) {
			r.Status.SetCondition(status.Condition{
				Type:    status.ConditionTypeReady,
				Status:  reconciledCondition.Status,
				Reason:  reconciledCondition.Reason,
				Message: reconciledCondition.Message,
			})
			updateStatus = true
		}
	} else {
		if len(unreadyComponents) > 0 {
			message := fmt.Sprintf("The following components are not fully available: %s", unreadyComponents.List())
			if !readyCondition.Matches(status.ConditionStatusFalse, status.ConditionReasonComponentsNotReady, message) {
				r.Status.SetCondition(status.Condition{
					Type:    status.ConditionTypeReady,
					Status:  status.ConditionStatusFalse,
					Reason:  status.ConditionReasonComponentsNotReady,
					Message: message,
				})
				r.EventRecorder.Event(r.Instance, corev1.EventTypeWarning, eventReasonNotReady, message)
				updateStatus = true
			}
		} else {
			message := "All component deployments are Available"
			if !readyCondition.Matches(status.ConditionStatusTrue, status.ConditionReasonComponentsReady, message) {
				r.Status.SetCondition(status.Condition{
					Type:    status.ConditionTypeReady,
					Status:  status.ConditionStatusTrue,
					Reason:  status.ConditionReasonComponentsReady,
					Message: message,
				})
				r.EventRecorder.Event(r.Instance, corev1.EventTypeNormal, eventReasonReady, message)
				updateStatus = true
			}
		}
	}

	readyComponentCount := fmt.Sprintf("%d/%d", len(readyComponents), len(r.Status.ComponentStatus))
	if r.Status.GetAnnotation(statusAnnotationReadyComponentCount) != readyComponentCount {
		r.Status.SetAnnotation(statusAnnotationReadyComponentCount, readyComponentCount)
		updateStatus = true
	}

	allComponents := sets.NewString()
	for _, comp := range r.Status.ComponentStatus {
		allComponents.Insert(comp.Resource)
	}

	readinessMap := maistrav2.ReadinessMap{
		"ready":   readyComponents.List(),
		"unready": unreadyComponents.List(),
		"pending": allComponents.Difference(readyComponents).Difference(unreadyComponents).List(),
	}
	if !reflect.DeepEqual(r.Status.Readiness.Components, readinessMap) {
		r.Status.Readiness.Components = readinessMap
		updateStatus = true
	}
	return updateStatus, nil
}

type isReadyFunc func(runtime.Object) bool

// keep this in sync with kinds in calculateComponentReadiness()
var kindsWithReadiness = sets.NewString("Deployment", "StatefulSet", "DaemonSet")

func (r *controlPlaneInstanceReconciler) hasReadiness(kind string) bool {
	return kindsWithReadiness.Has(kind)
}

func (r *controlPlaneInstanceReconciler) calculateComponentReadiness(ctx context.Context) (readyComponents, unreadyComponents sets.String, err error) {
	readyComponents = sets.NewString()
	unreadyComponents = sets.NewString()

	var readinessMap map[string]bool
	readinessMap, err = r.calculateComponentReadinessMap(ctx)
	if err != nil {
		return
	}
	for component, ready := range readinessMap {
		if ready {
			readyComponents.Insert(component)
		} else {
			unreadyComponents.Insert(component)
		}
	}
	return
}

func (r *controlPlaneInstanceReconciler) calculateComponentReadinessMap(ctx context.Context) (map[string]bool, error) {
	readinessMap := map[string]bool{}
	typesToCheck := []struct {
		list  runtime.Object
		ready isReadyFunc
	}{
		// keep this in sync with kindsWithReadiness
		{
			list: &appsv1.DeploymentList{},
			ready: func(obj runtime.Object) bool {
				deployment := obj.(*appsv1.Deployment)
				if deployment.Status.ReadyReplicas < deployment.Status.Replicas || deployment.Status.ObservedGeneration < deployment.Generation {
					return false
				}
				for _, condition := range deployment.Status.Conditions {
					if condition.Type == appsv1.DeploymentAvailable {
						return condition.Status == corev1.ConditionTrue
					}
				}
				return false
			},
		},
		{
			list: &appsv1.StatefulSetList{},
			ready: func(obj runtime.Object) bool {
				statefulSet := obj.(*appsv1.StatefulSet)
				return statefulSet.Status.ReadyReplicas >= statefulSet.Status.Replicas
			},
		},
		{
			list: &appsv1.DaemonSetList{},
			ready: func(obj runtime.Object) bool {
				daemonSet := obj.(*appsv1.DaemonSet)
				return r.daemonSetReady(daemonSet)
			},
		},
	}
	for _, check := range typesToCheck {
		err := r.calculateReadinessForType(ctx, check.list, check.ready, readinessMap)
		if err != nil {
			return readinessMap, err
		}
	}

	alwaysReadyComponents := r.Status.GetAnnotation(statusAnnotationAlwaysReadyComponents)
	if alwaysReadyComponents != "" {
		for _, c := range strings.Split(alwaysReadyComponents, ",") {
			readinessMap[c] = true
		}
	}
	return readinessMap, nil
}

func (r *controlPlaneInstanceReconciler) isCNIReady(ctx context.Context) (bool, error) {
	if !r.cniConfig.Enabled {
		return true, nil
	}
	labelSelector := map[string]string{"istio": "cni"}
	daemonSets := &appsv1.DaemonSetList{}
	operatorNamespace := common.GetOperatorNamespace()
	if err := r.Client.List(ctx, daemonSets, client.InNamespace(operatorNamespace), client.MatchingLabels(labelSelector)); err != nil {
		return true, err
	}
	for _, ds := range daemonSets.Items {
		if !r.daemonSetReady(&ds) {
			return false, nil
		}
	}
	return true, nil
}

func (r *controlPlaneInstanceReconciler) calculateReadinessForType(ctx context.Context, list runtime.Object, isReady isReadyFunc, readinessMap map[string]bool) error {
	log := common.LogFromContext(ctx)

	meshNamespace := r.Instance.GetNamespace()
	selector := map[string]string{common.OwnerKey: meshNamespace}
	err := r.Client.List(ctx, list, client.InNamespace(meshNamespace), client.MatchingLabels(selector))
	if err != nil {
		return err
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}

	for _, obj := range items {
		metaObject, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		if component, ok := metaObject.GetLabels()[common.KubernetesAppComponentKey]; ok {
			ready, exists := readinessMap[component]
			readinessMap[component] = (ready || !exists) && isReady(obj)
		} else {
			// how do we have an owned resource with no component label?
			log.Error(nil, "skipping resource for readiness check: resource has no component label", obj.GetObjectKind(), metaObject.GetName())
		}
	}
	return nil
}

func (r *controlPlaneInstanceReconciler) daemonSetReady(ds *appsv1.DaemonSet) bool {
	return ds.Status.NumberUnavailable == 0
}
