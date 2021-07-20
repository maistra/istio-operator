package helm

import (
	"context"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/releaseutil"
	kubectl "k8s.io/kubectl/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/apis/maistra/status"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

type ManifestProcessor struct {
	common.ControllerResources
	PatchFactory             *PatchFactory
	preprocessObject         func(ctx context.Context, obj *unstructured.Unstructured) error
	processNewObject         func(ctx context.Context, obj *unstructured.Unstructured) error
	preprocessObjectForPatch func(ctx context.Context, oldObj, newObj *unstructured.Unstructured) (*unstructured.Unstructured, error)

	appInstance, appVersion, owner string
}

func NewManifestProcessor(controllerResources common.ControllerResources, patchFactory *PatchFactory, appInstance, appVersion, owner string, preprocessObjectFunc, postProcessObjectFunc func(ctx context.Context, obj *unstructured.Unstructured) error, preprocessObjectForPatchFunc func(ctx context.Context, oldObj, newObj *unstructured.Unstructured) (*unstructured.Unstructured, error)) *ManifestProcessor {
	return &ManifestProcessor{
		ControllerResources:      controllerResources,
		PatchFactory:             patchFactory,
		preprocessObject:         preprocessObjectFunc,
		processNewObject:         postProcessObjectFunc,
		preprocessObjectForPatch: preprocessObjectForPatchFunc,
		appInstance:              appInstance,
		appVersion:               appVersion,
		owner:                    owner,
	}
}

func (p *ManifestProcessor) ProcessManifests(ctx context.Context, manifests []manifest.Manifest, component string) (madeChanges bool, err error) {
	log := common.LogFromContext(ctx)

	allErrors := []error{}
	for _, man := range manifests {
		childCtx := common.NewContextWithLog(ctx, log.WithValues("manifest", man.Name))
		changes, errs := p.ProcessManifest(childCtx, man, component)
		madeChanges = madeChanges || changes
		allErrors = append(allErrors, errs...)
	}
	return madeChanges, utilerrors.NewAggregate(allErrors)
}

func (p *ManifestProcessor) ProcessManifest(ctx context.Context, man manifest.Manifest, component string) (madeChanges bool, allErrors []error) {
	log := common.LogFromContext(ctx)
	if !strings.HasSuffix(man.Name, ".yaml") {
		log.V(2).Info("Skipping rendering of manifest")
		return false, nil
	}
	log.V(2).Info("Processing resources from manifest")

	// split the manifest into individual objects
	objects := releaseutil.SplitManifests(man.Content)
	for _, raw := range objects {
		if raw == "---" {
			continue
		}
		rawJSON, err := yaml.YAMLToJSON([]byte(raw))
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to convert raw data to JSON: %s", raw))
			allErrors = append(allErrors, err)
			continue
		}
		if len(rawJSON) == 0 || string(rawJSON) == "{}" || string(rawJSON) == "null" {
			// MAISTRA-2227 ignore empty objects.  this could happen if charts have in empty blocks, e.g. superfluous "---"
			continue
		}
		obj := &unstructured.Unstructured{}
		_, _, err = unstructured.UnstructuredJSONScheme.Decode(rawJSON, nil, obj)
		if err != nil {
			log.Error(err, "unable to decode object into Unstructured")
			log.V(2).Info(fmt.Sprintf("raw bytes:\n%s\n", raw))
			allErrors = append(allErrors, err)
			continue
		}

		childCtx := common.NewContextWithLog(ctx, log.WithValues("Resource", status.NewResourceKey(obj, obj)))
		changes, err := p.processObject(childCtx, obj, component)
		madeChanges = madeChanges || changes
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}
	return madeChanges, allErrors
}

func (p *ManifestProcessor) processObject(ctx context.Context, obj *unstructured.Unstructured, component string) (madeChanges bool, err error) {
	log := common.LogFromContext(ctx)

	if obj.GetKind() == "List" {
		allErrors := []error{}
		list, err := obj.ToList()
		if err != nil {
			log.Error(err, "error converting List object")
			return false, err
		}
		for _, item := range list.Items {
			childCtx := common.NewContextWithLog(ctx, log.WithValues("Resource", status.NewResourceKey(obj, obj)))
			changes, err := p.processObject(childCtx, &item, component)
			madeChanges = madeChanges || changes
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
		return madeChanges, utilerrors.NewAggregate(allErrors)
	}

	p.addMetadata(obj, component)

	log.V(2).Info("beginning reconciliation of resource")

	err = p.preprocessObject(ctx, obj)
	if err != nil {
		log.Error(err, "error preprocessing object")
		return false, err
	}

	err = kubectl.CreateApplyAnnotation(obj, unstructured.UnstructuredJSONScheme)
	if err != nil {
		log.Error(err, "error adding apply annotation to object")
	}

	receiver := status.NewResourceKey(obj, obj).ToUnstructured()
	objectKey, err := client.ObjectKeyFromObject(receiver)
	if err != nil {
		log.Error(err, "client.ObjectKeyFromObject() failed for resource")
		// This can only happen if reciever isn't an unstructured.Unstructured
		// i.e. this should never happen
		return madeChanges, err
	}

	var patch Patch

	err = p.Client.Get(ctx, objectKey, receiver)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("creating resource")
			err = p.Client.Create(ctx, obj)
			if err == nil {
				madeChanges = true
				// special handling
				if err := p.processNewObject(ctx, obj); err != nil {
					// just log for now
					log.Error(err, "error during postprocessing of new resource")
				}
			} else {
				log.Error(err, "error during creation of new resource")
			}
		}
	} else {
		preprocessedObj, err := p.preprocessObjectForPatch(ctx, receiver, obj)
		if err != nil {
			return madeChanges, err
		}
		if patch, err = p.PatchFactory.CreatePatch(receiver, preprocessedObj); err == nil && patch != nil {
			log.Info("updating existing resource")
			_, err = patch.Apply(ctx)
			if errors.IsInvalid(err) {
				// patch was invalid, try delete/create
				log.Info(fmt.Sprintf("patch failed: %v.  attempting to delete and recreate the resource", err))
				if deleteErr := p.Client.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationBackground)); deleteErr == nil {
					madeChanges = true
					// we need to remove the resource version, which was updated by the patching process
					obj.SetResourceVersion("")
					if createErr := p.Client.Create(ctx, obj); createErr == nil {
						log.Info("successfully recreated resource after patch failure")
						err = nil
					} else {
						log.Error(createErr, "error trying to recreate resource after patch failure")
					}
				} else {
					log.Error(deleteErr, "error deleting resource for recreation")
				}
			} else {
				madeChanges = true
			}
		}
	}
	log.V(2).Info("resource reconciliation complete")
	if err != nil {
		log.Error(err, "error occurred reconciling resource")
	}
	return madeChanges, err
}

func (p *ManifestProcessor) addMetadata(obj *unstructured.Unstructured, component string) {
	labels := map[string]string{
		// add app labels
		common.KubernetesAppNameKey:      component,
		common.KubernetesAppInstanceKey:  p.appInstance,
		common.KubernetesAppVersionKey:   p.appVersion,
		common.KubernetesAppComponentKey: component,
		common.KubernetesAppPartOfKey:    "istio",
		common.KubernetesAppManagedByKey: "maistra-istio-operator",
		// legacy
		// add owner label
		common.OwnerKey: p.owner,
	}
	common.SetLabels(obj, labels)
}
