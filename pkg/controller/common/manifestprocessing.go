package common

import (
	"context"
	"strings"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/releaseutil"

	"k8s.io/kubernetes/pkg/kubectl"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ManifestProcessor struct {
	ResourceManager
	preprocessObject func(obj *unstructured.Unstructured) error
	processNewObject func(obj *unstructured.Unstructured) error

	appInstance, appVersion, owner string
}

func NewManifestProcessor(resourceManager ResourceManager, appInstance, appVersion, owner string, preprocessObjectFunc, postProcessObjectFunc func(obj *unstructured.Unstructured) error) *ManifestProcessor {
	return &ManifestProcessor{
		ResourceManager:  resourceManager,
		preprocessObject: preprocessObjectFunc,
		processNewObject: postProcessObjectFunc,
		appInstance:      appInstance,
		appVersion:       appVersion,
		owner:            owner,
	}
}

func (p *ManifestProcessor) ProcessManifests(manifests []manifest.Manifest, component string) error {
	allErrors := []error{}

	origLogger := p.Log
	defer func() { p.Log = origLogger }()
	for _, man := range manifests {
		p.Log = origLogger.WithValues("manifest", man.Name)
		if !strings.HasSuffix(man.Name, ".yaml") {
			p.Log.V(2).Info("Skipping rendering of manifest")
			continue
		}
		p.Log.V(2).Info("Processing resources from manifest")
		// split the manifest into individual objects
		objects := releaseutil.SplitManifests(man.Content)
		for _, raw := range objects {
			rawJSON, err := yaml.YAMLToJSON([]byte(raw))
			if err != nil {
				p.Log.Error(err, "unable to convert raw data to JSON")
				allErrors = append(allErrors, err)
				continue
			}
			obj := &unstructured.Unstructured{}
			_, _, err = unstructured.UnstructuredJSONScheme.Decode(rawJSON, nil, obj)
			if err != nil {
				p.Log.Error(err, "unable to decode object into Unstructured")
				allErrors = append(allErrors, err)
				continue
			}
			err = p.processObject(obj, component)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
	}

	return utilerrors.NewAggregate(allErrors)
}

func (p *ManifestProcessor) processObject(obj *unstructured.Unstructured, component string) error {
	origLogger := p.Log
	defer func() { p.Log = origLogger }()

	key := v1.NewResourceKey(obj, obj)
	p.Log = origLogger.WithValues("Resource", key)

	if obj.GetKind() == "List" {
		allErrors := []error{}
		list, err := obj.ToList()
		if err != nil {
			p.Log.Error(err, "error converting List object")
			return err
		}
		for _, item := range list.Items {
			err = p.processObject(&item, component)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
		return utilerrors.NewAggregate(allErrors)
	}

	p.addMetadata(obj, component)

	p.Log.V(2).Info("beginning reconciliation of resource", "ResourceKey", key)

	err := p.preprocessObject(obj)
	if err != nil {
		p.Log.Error(err, "error preprocessing object")
		return err
	}

	err = kubectl.CreateApplyAnnotation(obj, unstructured.UnstructuredJSONScheme)
	if err != nil {
		p.Log.Error(err, "error adding apply annotation to object")
	}

	receiver := key.ToUnstructured()
	objectKey, err := client.ObjectKeyFromObject(receiver)
	if err != nil {
		p.Log.Error(err, "client.ObjectKeyFromObject() failed for resource")
		// This can only happen if reciever isn't an unstructured.Unstructured
		// i.e. this should never happen
		return err
	}

	var patch Patch

	err = p.Client.Get(context.TODO(), objectKey, receiver)
	if err != nil {
		if errors.IsNotFound(err) {
			p.Log.Info("creating resource")
			err = p.Client.Create(context.TODO(), obj)
			if err == nil {
				// special handling
				if err := p.processNewObject(obj); err != nil {
					// just log for now
					p.Log.Error(err, "error during postprocessing of new resource")
				}
			} else {
				p.Log.Error(err, "error during creation of new resource")
			}
		}
	} else if patch, err = p.PatchFactory.CreatePatch(receiver, obj); err == nil && patch != nil {
		p.Log.Info("updating existing resource")
		_, err = patch.Apply()
		if errors.IsInvalid(err) {
			// patch was invalid, try delete/create
			p.Log.Info("patch failed.  attempting to delete and recreate the resource")
			if deleteErr := p.Client.Delete(context.TODO(), obj, client.PropagationPolicy(metav1.DeletePropagationBackground)); deleteErr == nil {
				// we need to remove the resource version, which was updated by the patching process
				obj.SetResourceVersion("")
				if createErr := p.Client.Create(context.TODO(), obj); createErr == nil {
					p.Log.Info("successfully recreated resource after patch failure")
					err = nil
				} else {
					p.Log.Error(createErr, "error trying to recreate resource after patch failure")
				}
			} else {
				p.Log.Error(deleteErr, "error deleting resource for recreation")
			}
		}
	}
	p.Log.V(2).Info("resource reconciliation complete")
	if err != nil {
		p.Log.Error(err, "error occurred reconciling resource")
	}
	return err
}

func (p *ManifestProcessor) addMetadata(obj *unstructured.Unstructured, component string) {
	labels := map[string]string{
		// add app labels
		KubernetesAppNameKey:      component,
		KubernetesAppInstanceKey:  p.appInstance,
		KubernetesAppVersionKey:   p.appVersion,
		KubernetesAppComponentKey: component,
		KubernetesAppPartOfKey:    "istio",
		KubernetesAppManagedByKey: "maistra-istio-operator",
		// legacy
		// add owner label
		OwnerKey: p.owner,
	}
	SetLabels(obj, labels)
}
