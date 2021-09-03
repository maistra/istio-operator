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
		err = p.processObject(childCtx, obj, component)
		if err != nil {
			allErrors = append(allErrors, err)
		}
	}
	return allErrors
}

func (p *ManifestProcessor) processObject(ctx context.Context, obj *unstructured.Unstructured, component string) error {
	log := common.LogFromContext(ctx)

	var err error
	obj, err = p.convertToSupportedApiVersion(ctx, obj)
	if err != nil {
		return err
	}

	if obj.GetKind() == "List" {
		allErrors := []error{}
		list, err := obj.ToList()
		if err != nil {
			log.Error(err, "error converting List object")
			return err
		}
		for _, item := range list.Items {
			childCtx := common.NewContextWithLog(ctx, log.WithValues("Resource", status.NewResourceKey(obj, obj)))
			err = p.processObject(childCtx, &item, component)
			if err != nil {
				allErrors = append(allErrors, err)
			}
		}
		return utilerrors.NewAggregate(allErrors)
	}

	p.addMetadata(obj, component)

	log.V(2).Info("beginning reconciliation of resource")

	err = p.preprocessObject(ctx, obj)
	if err != nil {
		log.Error(err, "error preprocessing object")
		return err
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
			log.Info(fmt.Sprintf("patch failed: %v.  attempting to delete and recreate the resource", err))
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

// if the given object's apiVersion is not supported by the cluster, the object is converted to one that is
// (e.g. admissionregistration.k8s.io/v1beta1 -> admissionregistration.k8s.io/v1)
func (p *ManifestProcessor) convertToSupportedApiVersion(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if (obj.GetKind() == "MutatingWebhookConfiguration" || obj.GetKind() == "ValidatingWebhookConfiguration") &&
		obj.GetAPIVersion() == "admissionregistration.k8s.io/v1beta1" {
		return convertWebhookConfigurationFromV1beta1ToV1(obj)
	}
	return obj, nil
}

// converts MutatingWebhookConfiguration or ValidationWebhookConfiguration from apiVersion admissionregistration.k8s.io/v1beta1
// to admissionregistration.k8s.io/v1 by doing the following:
// - changes the apiVersion to v1
// - sets field webhooks[*].admissionReviewVersions to ["v1beta1"]
func convertWebhookConfigurationFromV1beta1ToV1(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	obj = obj.DeepCopy()
	err := unstructured.SetNestedField(obj.UnstructuredContent(), "admissionregistration.k8s.io/v1", "apiVersion")
	if err != nil {
		return nil, err
	}
	webhooks, found, err := unstructured.NestedSlice(obj.UnstructuredContent(), "webhooks")
	if err != nil {
		return nil, err
	}
	if found {
		for i, webhook := range webhooks {
			typedWebhook, _ := webhook.(map[string]interface{})
			err = unstructured.SetNestedStringSlice(typedWebhook, []string{"v1beta1"}, "admissionReviewVersions")
			if err != nil {
				return nil, err
			}
			webhooks[i] = webhook
		}
		err = unstructured.SetNestedField(obj.UnstructuredContent(), webhooks, "webhooks")
		if err != nil {
			return nil, err
		}
	}
	return obj, nil
}
