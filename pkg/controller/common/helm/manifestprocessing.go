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
	"k8s.io/kubernetes/pkg/kubectl"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

type ManifestProcessor struct {
	common.ControllerResources
	PatchFactory     *PatchFactory
	preprocessObject func(ctx context.Context, obj *unstructured.Unstructured) error
	processNewObject func(ctx context.Context, obj *unstructured.Unstructured) error

	appInstance, appVersion, owner string
}

func NewManifestProcessor(controllerResources common.ControllerResources, patchFactory *PatchFactory, appInstance, appVersion, owner string, preprocessObjectFunc, postProcessObjectFunc func(ctx context.Context, obj *unstructured.Unstructured) error) *ManifestProcessor {
	return &ManifestProcessor{
		ControllerResources: controllerResources,
		PatchFactory:        patchFactory,
		preprocessObject:    preprocessObjectFunc,
		processNewObject:    postProcessObjectFunc,
		appInstance:         appInstance,
		appVersion:          appVersion,
		owner:               owner,
	}
}

func (p *ManifestProcessor) ProcessManifests(ctx context.Context, manifests []manifest.Manifest, component string) error {
	log := common.LogFromContext(ctx)

	allErrors := []error{}
	for _, man := range manifests {
		childCtx := common.NewContextWithLog(ctx, log.WithValues("manifest", man.Name))
		errs := p.ProcessManifest(childCtx, man, component)
		allErrors = append(allErrors, errs...)
	}
	return utilerrors.NewAggregate(allErrors)
}

func (p *ManifestProcessor) ProcessManifest(ctx context.Context, man manifest.Manifest, component string) []error {
	log := common.LogFromContext(ctx)
	if !strings.HasSuffix(man.Name, ".yaml") {
		log.V(2).Info("Skipping rendering of manifest")
		return nil
	}
	log.V(2).Info("Processing resources from manifest")

	allErrors := []error{}
	// split the manifest into individual objects
	objects := releaseutil.SplitManifests(man.Content)
	for _, raw := range objects {
		rawJSON, err := yaml.YAMLToJSON([]byte(raw))
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to convert raw data to JSON: %s", raw))
			allErrors = append(allErrors, err)
			continue
		}
		obj := &unstructured.Unstructured{}
		_, _, err = unstructured.UnstructuredJSONScheme.Decode(rawJSON, nil, obj)
		if err != nil {
			log.Error(err, "unable to decode object into Unstructured")
			allErrors = append(allErrors, err)
			continue
		}

		childCtx := common.NewContextWithLog(ctx, log.WithValues("Resource", v1.NewResourceKey(obj, obj)))
		err = p.processObject(childCtx, obj, component)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}
	return allErrors
}

func (p *ManifestProcessor) processObject(ctx context.Context, obj *unstructured.Unstructured, component string) error {
	log := common.LogFromContext(ctx)

	if obj.GetKind() == "List" {
		allErrors := []error{}
		list, err := obj.ToList()
		if err != nil {
			log.Error(err, "error converting List object")
			return err
		}
		for _, item := range list.Items {
			childCtx := common.NewContextWithLog(ctx, log.WithValues("Resource", v1.NewResourceKey(obj, obj)))
			err = p.processObject(childCtx, &item, component)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
		return utilerrors.NewAggregate(allErrors)
	}

	p.addMetadata(obj, component)

	log.V(2).Info("beginning reconciliation of resource")

	err := p.preprocessObject(ctx, obj)
	if err != nil {
		log.Error(err, "error preprocessing object")
		return err
	}

	err = kubectl.CreateApplyAnnotation(obj, unstructured.UnstructuredJSONScheme)
	if err != nil {
		log.Error(err, "error adding apply annotation to object")
	}

	receiver := v1.NewResourceKey(obj, obj).ToUnstructured()
	objectKey, err := client.ObjectKeyFromObject(receiver)
	if err != nil {
		log.Error(err, "client.ObjectKeyFromObject() failed for resource")
		// This can only happen if reciever isn't an unstructured.Unstructured
		// i.e. this should never happen
		return err
	}

	var patch Patch

	err = p.Client.Get(ctx, objectKey, receiver)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("creating resource")
			err = p.Client.Create(ctx, obj)
			if err == nil {
				// special handling
				if err := p.processNewObject(ctx, obj); err != nil {
					// just log for now
					log.Error(err, "error during postprocessing of new resource")
				}
			} else {
				log.Error(err, "error during creation of new resource")
			}
		}
	} else if patch, err = p.PatchFactory.CreatePatch(receiver, obj); err == nil && patch != nil {
		log.Info("updating existing resource")
		_, err = patch.Apply(ctx)
		if errors.IsInvalid(err) {
			// patch was invalid, try delete/create
			log.Info("patch failed.  attempting to delete and recreate the resource")
			if deleteErr := p.Client.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationBackground)); deleteErr == nil {
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
		}
	}
	log.V(2).Info("resource reconciliation complete")
	if err != nil {
		log.Error(err, "error occurred reconciling resource")
	}
	return err
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
