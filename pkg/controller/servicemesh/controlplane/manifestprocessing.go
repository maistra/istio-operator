package controlplane

import (
	"context"
	"strings"

	"github.com/ghodss/yaml"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/releaseutil"

	"k8s.io/kubernetes/pkg/kubectl"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ControlPlaneReconciler) processComponentManifests(chartName string) (ready bool, err error) {
	componentName := componentFromChartName(chartName)
	origLogger := r.Log
	r.Log = r.Log.WithValues("Component", componentName)
	defer func() { r.Log = origLogger }()

	renderings, hasRenderings := r.renderings[chartName]
	if !hasRenderings {
		r.Log.V(5).Info("no renderings for component")
		ready = true
		return
	}

	r.Log.Info("reconciling component resources")
	status := r.Status.FindComponentByName(componentName)
	defer func() {
		updateReconcileStatus(&status.StatusType, err)
		r.Log.Info("component reconciliation complete")
	}()
	if err = r.processManifests(renderings, status); err != nil {
		return
	}
	if err = r.processNewComponent(componentName, status); err != nil {
		r.Log.Error(err, "unexpected error occurred during postprocessing of component")
		return
	}

	// if we get here, the component has been successfully installed
	delete(r.renderings, chartName)

	// for reentry into the reconcile loop, if not ready
	r.lastComponent = componentName
	if notReadyMap, readyErr := r.calculateNotReadyState(); readyErr == nil {
		ready = !notReadyMap[r.lastComponent]
	} else {
		err = readyErr
	}
	return
}

func (r *ControlPlaneReconciler) processManifests(manifests []manifest.Manifest, status *v1.ComponentStatus) error {
	allErrors := []error{}

	origLogger := r.Log
	defer func() { r.Log = origLogger }()
	for _, manifest := range manifests {
		r.Log = origLogger.WithValues("manifest", manifest.Name)
		if !strings.HasSuffix(manifest.Name, ".yaml") {
			r.Log.V(2).Info("Skipping rendering of manifest")
			continue
		}
		r.Log.V(2).Info("Processing resources from manifest")
		// split the manifest into individual objects
		objects := releaseutil.SplitManifests(manifest.Content)
		for _, raw := range objects {
			rawJSON, err := yaml.YAMLToJSON([]byte(raw))
			if err != nil {
				r.Log.Error(err, "unable to convert raw data to JSON")
				allErrors = append(allErrors, err)
				continue
			}
			obj := &unstructured.Unstructured{}
			_, _, err = unstructured.UnstructuredJSONScheme.Decode(rawJSON, nil, obj)
			if err != nil {
				r.Log.Error(err, "unable to decode object into Unstructured")
				allErrors = append(allErrors, err)
				continue
			}
			err = r.processObject(obj, status)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

func (r *ControlPlaneReconciler) processObject(obj *unstructured.Unstructured, status *v1.ComponentStatus) error {
	origLogger := r.Log
	defer func() { r.Log = origLogger }()

	key := v1.NewResourceKey(obj, obj)
	r.Log = origLogger.WithValues("Resource", key)

	if obj.GetKind() == "List" {
		allErrors := []error{}
		list, err := obj.ToList()
		if err != nil {
			r.Log.Error(err, "error converting List object")
			return err
		}
		for _, item := range list.Items {
			err = r.processObject(&item, status)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
		return utilerrors.NewAggregate(allErrors)
	}

	// Add owner ref
	if obj.GetNamespace() == r.Instance.GetNamespace() {
		obj.SetOwnerReferences(r.ownerRefs)
	} else {
		// XXX: can't set owner reference on cross-namespace or cluster resources
	}

	r.addMetadata(obj, status.Resource)

	r.Log.V(2).Info("beginning reconciliation of resource", "ResourceKey", key)

	err := r.preprocessObject(obj)
	if err != nil {
		r.Log.Error(err, "error preprocessing object")
		return err
	}

	err = kubectl.CreateApplyAnnotation(obj, unstructured.UnstructuredJSONScheme)
	if err != nil {
		r.Log.Error(err, "unexpected error adding apply annotation to object")
	}

	receiver := key.ToUnstructured()
	objectKey, err := client.ObjectKeyFromObject(receiver)
	if err != nil {
		r.Log.Error(err, "client.ObjectKeyFromObject() failed for resource")
		// This can only happen if reciever isn't an unstructured.Unstructured
		// i.e. this should never happen
		return err
	}

	var patch common.Patch

	err = r.Client.Get(context.TODO(), objectKey, receiver)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("creating resource")
			err = r.Client.Create(context.TODO(), obj)
			if err == nil {
				// special handling
				if err := r.processNewObject(obj); err != nil {
					// just log for now
					r.Log.Error(err, "unexpected error occurred during postprocessing of new resource")
				}
			} else {
				r.Log.Error(err, "unexpected error occurred during creation of new resource")
			}
		}
	} else if patch, err = r.PatchFactory.CreatePatch(receiver, obj); err == nil && patch != nil {
		r.Log.Info("updating existing resource")
		_, err = patch.Apply()
	}
	r.Log.V(2).Info("resource reconciliation complete")
	if err != nil {
		r.Log.Error(err, "error occurred reconciling resource")
	}
	return err
}

func (r *ControlPlaneReconciler) addMetadata(obj *unstructured.Unstructured, component string) {
	labels := map[string]string{
		// add app labels
		common.KubernetesAppNameKey:      component,
		common.KubernetesAppInstanceKey:  r.Instance.GetNamespace(),
		common.KubernetesAppVersionKey:   r.meshGeneration,
		common.KubernetesAppComponentKey: component,
		common.KubernetesAppPartOfKey:    "istio",
		common.KubernetesAppManagedByKey: "maistra-istio-operator",
		// legacy
		// add owner label
		common.OwnerKey: r.Instance.GetNamespace(),
	}
	common.SetLabels(obj, labels)

	// add generation annotation
	common.SetAnnotation(obj, common.MeshGenerationKey, r.meshGeneration)
}
