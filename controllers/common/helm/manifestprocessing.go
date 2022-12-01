package helm

import (
	"context"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	errors2 "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/releaseutil"
	kubectl "k8s.io/kubectl/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/apis/maistra/status"
	"github.com/maistra/istio-operator/controllers/common"
)

// List of resource kinds found only in OpenShift, not bare Kubernetes.
// When creation of these kinds of resources fails, the failure is logged,
// but the reconciliation of the SMCP continues.
var openshiftSpecificResourceKinds = []schema.GroupVersionKind{
	{
		Group:   "route.openshift.io",
		Version: "v1",
		Kind:    "Route",
	},
}

type ManifestProcessor struct {
	common.ControllerResources
	PatchFactory             *PatchFactory
	preprocessObject         func(ctx context.Context, obj *unstructured.Unstructured) (bool, error)
	processNewObject         func(ctx context.Context, obj *unstructured.Unstructured) error
	preprocessObjectForPatch func(ctx context.Context, oldObj, newObj *unstructured.Unstructured) (*unstructured.Unstructured, error)

	appInstance, appVersion string
	owner                   types.NamespacedName
}

func NewManifestProcessor(controllerResources common.ControllerResources, patchFactory *PatchFactory,
	appInstance, appVersion string, owner types.NamespacedName,
	preprocessObjectFunc func(ctx context.Context, obj *unstructured.Unstructured) (bool, error),
	postProcessObjectFunc func(ctx context.Context, obj *unstructured.Unstructured) error,
	preprocessObjectForPatchFunc func(ctx context.Context, oldObj, newObj *unstructured.Unstructured) (*unstructured.Unstructured, error),
) *ManifestProcessor {
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
			allErrors = append(allErrors, errors2.Wrap(err, man.Name))
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
			allErrors = append(allErrors, errors2.Wrap(err, man.Name))
			continue
		}

		childCtx := common.NewContextWithLog(ctx, log.WithValues("Resource", status.NewResourceKey(obj, obj)))
		changes, err := p.processObject(childCtx, obj, component)
		madeChanges = madeChanges || changes
		if err != nil {
			allErrors = append(allErrors, errors2.Wrap(err, man.Name))
		}
	}
	return madeChanges, allErrors
}

func (p *ManifestProcessor) processObject(ctx context.Context, obj *unstructured.Unstructured, component string) (madeChanges bool, err error) {
	log := common.LogFromContext(ctx)

	obj, err = p.convertToSupportedAPIVersion(obj)
	if err != nil {
		return false, err
	}

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

	mustContinue, err := p.preprocessObject(ctx, obj)
	if err != nil {
		log.Error(err, "error preprocessing object")
		return false, err
	}
	if !mustContinue {
		log.Info("skipping processing of resource due to spec.security.manageNetworkPolicy = false")
		return false, nil
	}

	err = kubectl.CreateApplyAnnotation(obj, unstructured.UnstructuredJSONScheme)
	if err != nil {
		log.Error(err, "error adding apply annotation to object")
	}

	receiver := status.NewResourceKey(obj, obj).ToUnstructured()
	objectKey := client.ObjectKeyFromObject(receiver)

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
			}
		}
	} else {
		var preprocessedObj *unstructured.Unstructured
		preprocessedObj, err = p.preprocessObjectForPatch(ctx, receiver, obj)
		if err != nil {
			return madeChanges, err
		}
		if patch, err = p.PatchFactory.CreatePatch(receiver, preprocessedObj); err == nil && patch != nil {
			log.Info("updating existing resource")
			_, err = patch.Apply(ctx)
			if errors.IsInvalid(err) || IsRouteNoHostError(err) {
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
	if err == nil {
		log.V(2).Info("resource reconciliation complete")
	} else {
		if meta.IsNoMatchError(err) && isOpenShiftSpecificResource(obj) {
			log.Info("resource kind not supported by cluster; operator likely not running in OpenShift, but in vanilla Kubernetes")
			err = nil
		} else {
			log.Error(err, "error occurred reconciling resource")
		}
	}
	return madeChanges, err
}

func isOpenShiftSpecificResource(obj *unstructured.Unstructured) bool {
	for _, gvk := range openshiftSpecificResourceKinds {
		if gvk == obj.GetObjectKind().GroupVersionKind() {
			return true
		}
	}
	return false
}

func (p *ManifestProcessor) addMetadata(obj *unstructured.Unstructured, component string) {
	labels := map[string]string{
		// add app labels
		common.KubernetesAppNameKey:      component,
		common.KubernetesAppInstanceKey:  p.appInstance,
		common.KubernetesAppVersionKey:   p.appVersion,
		common.KubernetesAppComponentKey: component,
		common.KubernetesAppPartOfKey:    common.KubernetesAppPartOfValue,
		common.KubernetesAppManagedByKey: common.KubernetesAppManagedByValue,
		// legacy
		// add owner label
		common.OwnerKey:     p.owner.Namespace,
		common.OwnerNameKey: p.owner.Name,
	}
	common.SetLabels(obj, labels)
}

// if the given object's apiVersion is not supported by the cluster, the object is converted to one that is
// (e.g. admissionregistration.k8s.io/v1beta1 -> admissionregistration.k8s.io/v1)
func (p *ManifestProcessor) convertToSupportedAPIVersion(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
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
			admissionReviewVersions, exists, err := unstructured.NestedSlice(typedWebhook, "admissionReviewVersions")
			if err != nil {
				return nil, err
			}
			if !exists || len(admissionReviewVersions) == 0 {
				err = unstructured.SetNestedStringSlice(typedWebhook, []string{"v1beta1"}, "admissionReviewVersions")
				if err != nil {
					return nil, err
				}
			}

			sideEffects, exists, err := unstructured.NestedString(typedWebhook, "sideEffects")
			if err != nil {
				return nil, err
			}
			if !exists || sideEffects == "" {
				err = unstructured.SetNestedField(typedWebhook, "None", "sideEffects")
				if err != nil {
					return nil, err
				}
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
