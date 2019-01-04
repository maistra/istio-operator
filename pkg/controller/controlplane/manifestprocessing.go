package controlplane

import (
	"context"
	"reflect"
	"strings"

	"github.com/ghodss/yaml"

	istiov1alpha3 "github.com/maistra/istio-operator/pkg/apis/istio/v1alpha3"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/releaseutil"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *controlPlaneReconciler) processComponentManifests(componentName string) error {
	var err error
	status, hasStatus := r.instance.Status.ComponentStatus[componentName]
	renderings, hasRenderings := r.renderings[componentName]
	origLogger := r.log
	r.log = r.log.WithValues("Component", componentName)
	defer func() { r.log = origLogger }()
	if hasRenderings {
		if !hasStatus {
			status = istiov1alpha3.NewComponentStatus()
			r.instance.Status.ComponentStatus[componentName] = status
		}
		r.log.Info("reconciling component resources")
		status.RemoveCondition(istiov1alpha3.ConditionTypeReconciled)
		err := r.processManifests(renderings, status)
		updateReconcileStatus(&status.StatusType, err)
		status.ObservedGeneration = r.instance.GetGeneration()
		r.processNewComponent(componentName, status)
	} else if hasStatus && status.GetCondition(istiov1alpha3.ConditionTypeInstalled).Status != istiov1alpha3.ConditionStatusFalse {
		// delete resources
		r.log.Info("deleting component resources")
		err := r.processManifests([]manifest.Manifest{}, status)
		updateDeleteStatus(&status.StatusType, err)
		status.ObservedGeneration = r.instance.GetGeneration()
		r.processDeletedComponent(componentName, status)
	} else {
		r.log.Info("no renderings for component")
	}
	r.log.Info("component reconciliation complete")
	return err
}

func (r *controlPlaneReconciler) processManifests(manifests []manifest.Manifest,
	componentStatus *istiov1alpha3.ComponentStatus) error {

	allErrors := []error{}
	resourcesProcessed := map[istiov1alpha3.ResourceKey]struct{}{}

	origLogger := r.log
	defer func() { r.log = origLogger }()
	for _, manifest := range manifests {
		r.log = origLogger.WithValues("manifest", manifest.Name)
		if !strings.HasSuffix(manifest.Name, ".yaml") {
			r.log.V(2).Info("Skipping rendering of manifest")
			continue
		}
		r.log.V(2).Info("Processing resources from manifest")
		// split the manifest into individual objects
		objects := releaseutil.SplitManifests(manifest.Content)
		for _, raw := range objects {
			rawJSON, err := yaml.YAMLToJSON([]byte(raw))
			if err != nil {
				r.log.Error(err, "unable to convert raw data to JSON")
				allErrors = append(allErrors, err)
				continue
			}
			obj := &unstructured.Unstructured{}
			_, _, err = unstructured.UnstructuredJSONScheme.Decode(rawJSON, nil, obj)
			if err != nil {
				r.log.Error(err, "unable to decode object into Unstructured")
				allErrors = append(allErrors, err)
				continue
			}
			err = r.processObject(obj, resourcesProcessed, componentStatus)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
	}

	// handle deletions
	// XXX: should these be processed in reverse order of creation?
	for key, status := range componentStatus.ResourceStatus {
		r.log = origLogger.WithValues("Resource", key)
		if _, ok := resourcesProcessed[key]; !ok {
			if condition := status.GetCondition(istiov1alpha3.ConditionTypeInstalled); condition.Status != istiov1alpha3.ConditionStatusFalse {
				r.log.Info("deleting resource")
				unstructured := key.ToUnstructured()
				err := r.client.Delete(context.TODO(), unstructured, client.PropagationPolicy(metav1.DeletePropagationForeground))
				updateDeleteStatus(status, err)
				if err == nil || errors.IsNotFound(err) {
					status.ObservedGeneration = 0
					// special handling
					r.processDeletedObject(unstructured)
				} else {
					r.log.Error(err, "error deleting resource")
					allErrors = append(allErrors, err)
				}
			}
		}
	}
	return utilerrors.NewAggregate(allErrors)
}

func (r *controlPlaneReconciler) processObject(obj *unstructured.Unstructured, resourcesProcessed map[istiov1alpha3.ResourceKey]struct{},
	componentStatus *istiov1alpha3.ComponentStatus) error {
	origLogger := r.log
	defer func() { r.log = origLogger }()

	key := istiov1alpha3.NewResourceKey(obj, obj)
	r.log = origLogger.WithValues("Resource", key)

	if obj.GetKind() == "List" {
		allErrors := []error{}
		list, err := obj.ToList()
		if err != nil {
			r.log.Error(err, "error converting List object")
			return err
		}
		for _, item := range list.Items {
			err = r.processObject(&item, resourcesProcessed, componentStatus)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
		return utilerrors.NewAggregate(allErrors)
	}

	// Add owner ref
	obj.SetOwnerReferences(r.ownerRefs)

	r.log.V(2).Info("beginning reconciliation of resource", "ResourceKey", key)

	resourcesProcessed[key] = seen
	status, ok := componentStatus.ResourceStatus[key]
	if !ok {
		newStatus := istiov1alpha3.NewStatus()
		status = &newStatus
		componentStatus.ResourceStatus[key] = status
	}

	err := r.patchObject(obj)
	if err != nil {
		r.log.Error(err, "error patching object")
		updateReconcileStatus(status, err)
		return err
	}

	receiver := key.ToUnstructured()
	objectKey, err := client.ObjectKeyFromObject(receiver)
	if err != nil {
		r.log.Error(err, "client.ObjectKeyFromObject() failed for resource")
		// This can only happen if reciever isn't an unstructured.Unstructured
		// i.e. this should never happen
		updateReconcileStatus(status, err)
		return err
	}
	err = r.client.Get(context.TODO(), objectKey, receiver)
	if err != nil {
		if errors.IsNotFound(err) {
			r.log.Info("creating resource")
			err = r.client.Create(context.TODO(), obj)
			if err == nil {
				status.ObservedGeneration = 1
				// special handling
				r.processNewObject(obj)
			}
		}
	} else if receiver.GetGeneration() > 0 && receiver.GetGeneration() == status.ObservedGeneration {
		// nothing to do
		r.log.V(2).Info("resource generation matches status")
	} else if shouldUpdate(obj.UnstructuredContent(), receiver.UnstructuredContent()) {
		r.log.Info("updating existing resource")
		status.RemoveCondition(istiov1alpha3.ConditionTypeReconciled)
		//r.log.Info("updates not supported at this time")
		// XXX: k8s barfs on some updates: metadata.resourceVersion: Invalid value: 0x0: must be specified for an update
		obj.SetResourceVersion(receiver.GetResourceVersion())
		err = r.client.Update(context.TODO(), obj)
		if err == nil {
			status.ObservedGeneration = obj.GetGeneration()
		}
	}
	r.log.V(2).Info("resource reconciliation complete")
	updateReconcileStatus(status, err)
	if err != nil {
		r.log.Error(err, "error occurred reconciling resource")
	}
	return err
}

// shouldUpdate checks to see if the spec fields are the same for both objects.
// if the objects don't have a spec field, it checks all other fields, skipping
// known fields that shouldn't impact updates: kind, apiVersion, metadata, and status.
func shouldUpdate(o1, o2 map[string]interface{}) bool {
	if spec1, ok1 := o1["spec"]; ok1 {
		// we assume these are the same type of object
		return reflect.DeepEqual(spec1, o2["spec"])
	}
	for key, value := range o1 {
		if key == "status" || key == "kind" || key == "apiVersion" || key == "metadata" {
			continue
		}
		if !reflect.DeepEqual(value, o2[key]) {
			return true
		}
	}
	return false
}
