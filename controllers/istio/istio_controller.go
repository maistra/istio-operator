/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package istio

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/helm"
	"maistra.io/istio-operator/pkg/kube"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"istio.io/istio/pkg/ptr"
	"istio.io/istio/pkg/util/sets"
)

// IstioReconciler reconciles an Istio object
type IstioReconciler struct {
	ResourceDirectory string
	DefaultProfiles   []string
	client.Client
	Scheme *runtime.Scheme
}

func NewIstioReconciler(client client.Client, scheme *runtime.Scheme, resourceDir string, defaultProfiles []string) *IstioReconciler {
	return &IstioReconciler{
		ResourceDirectory: resourceDir,
		DefaultProfiles:   defaultProfiles,
		Client:            client,
		Scheme:            scheme,
	}
}

// +kubebuilder:rbac:groups=operator.istio.io,resources=istios,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.istio.io,resources=istios/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.istio.io,resources=istios/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *IstioReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("reconciler")

	var istio v1alpha1.Istio
	if err := r.Client.Get(ctx, req.NamespacedName, &istio); err != nil {
		if errors.IsNotFound(err) {
			logger.V(2).Info("Istio not found. Skipping reconciliation")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get Istio from cluster")
	}

	if istio.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling")
	result, err := r.doReconcile(ctx, istio)

	logger.Info("Reconciliation done. Updating status.")
	err = r.updateStatus(ctx, &istio, err)

	return result, err
}

// doReconcile is the function that actually reconciles the Istio object. Any error reported by this
// function should get reported in the status of the Istio object by the caller.
func (r *IstioReconciler) doReconcile(ctx context.Context, istio v1alpha1.Istio) (result ctrl.Result, err error) {
	if istio.Spec.Version == "" {
		return ctrl.Result{}, fmt.Errorf("no spec.version set")
	}
	if istio.Spec.Namespace == "" {
		return ctrl.Result{}, fmt.Errorf("no spec.namespace set")
	}

	var values helm.HelmValues
	if values, err = computeIstioRevisionValues(istio, r.DefaultProfiles, r.ResourceDirectory); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcileActiveRevision(ctx, &istio, values); err != nil {
		return ctrl.Result{}, err
	}

	return r.pruneInactiveRevisions(ctx, &istio)
}

func (r *IstioReconciler) reconcileActiveRevision(ctx context.Context, istio *v1alpha1.Istio, values helm.HelmValues) error {
	logger := log.FromContext(ctx).WithName("reconciler")

	valuesRawMessage, err := json.Marshal(values)
	if err != nil {
		return err
	}

	rev, err := r.getActiveRevision(ctx, istio)
	if err == nil {
		// update
		rev.Spec.Version = istio.Spec.Version
		rev.Spec.Values = valuesRawMessage
		logger.Info("Updating IstioRevision", "name", istio.Name, "spec", rev.Spec)
		return r.Client.Update(ctx, &rev)
	} else if errors.IsNotFound(err) {
		// create new
		rev = v1alpha1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: getActiveRevisionName(istio),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         v1alpha1.GroupVersion.String(),
						Kind:               v1alpha1.IstioKind,
						Name:               istio.Name,
						UID:                istio.UID,
						Controller:         ptr.Of(true),
						BlockOwnerDeletion: ptr.Of(true),
					},
				},
			},
			Spec: v1alpha1.IstioRevisionSpec{
				Version:   istio.Spec.Version,
				Namespace: istio.Spec.Namespace,
				Values:    valuesRawMessage,
			},
		}
		logger.Info("Creating IstioRevision", "name", istio.Name, "spec", rev.Spec)
		return r.Client.Create(ctx, &rev)
	}
	return err
}

func (r *IstioReconciler) pruneInactiveRevisions(ctx context.Context, istio *v1alpha1.Istio) (ctrl.Result, error) {
	revisions, err := r.getNonActiveRevisions(ctx, istio)
	if err != nil {
		return ctrl.Result{}, err
	}

	// the following code does two things:
	// - prunes revisions whose grace period has expired
	// - finds the time when the next revision is to be pruned
	var nextPruneTimestamp *time.Time
	for _, rev := range revisions {
		inUseCondition := rev.Status.GetCondition(v1alpha1.IstioRevisionConditionTypeInUse)
		inUse := inUseCondition.Status == metav1.ConditionTrue
		if inUse {
			continue
		}

		pruneTimestamp := inUseCondition.LastTransitionTime.Time.Add(getPruningGracePeriod(istio))
		expired := pruneTimestamp.Before(time.Now())
		if expired {
			err = r.Client.Delete(ctx, &rev)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else if nextPruneTimestamp == nil || nextPruneTimestamp.After(pruneTimestamp) {
			nextPruneTimestamp = &pruneTimestamp
		}
	}
	if nextPruneTimestamp == nil {
		return ctrl.Result{}, nil
	}
	// requeue so that we prune the next revision at the right time (if we didn't, we would prune it when
	// something else triggers another reconciliation)
	return ctrl.Result{RequeueAfter: time.Until(*nextPruneTimestamp)}, nil
}

func getPruningGracePeriod(istio *v1alpha1.Istio) time.Duration {
	strategy := istio.Spec.UpdateStrategy
	period := int64(v1alpha1.DefaultRevisionDeletionGracePeriodSeconds)
	if strategy != nil && strategy.InactiveRevisionDeletionGracePeriodSeconds != nil {
		period = *strategy.InactiveRevisionDeletionGracePeriodSeconds
	}
	if period < v1alpha1.MinRevisionDeletionGracePeriodSeconds {
		period = v1alpha1.MinRevisionDeletionGracePeriodSeconds
	}
	return time.Duration(period) * time.Second
}

func (r *IstioReconciler) getActiveRevision(ctx context.Context, istio *v1alpha1.Istio) (v1alpha1.IstioRevision, error) {
	rev := v1alpha1.IstioRevision{}
	err := r.Client.Get(ctx, getActiveRevisionKey(istio), &rev)
	return rev, err
}

func (r *IstioReconciler) getNonActiveRevisions(ctx context.Context, istio *v1alpha1.Istio) ([]v1alpha1.IstioRevision, error) {
	revList := v1alpha1.IstioRevisionList{}
	if err := r.Client.List(ctx, &revList); err != nil {
		return nil, err
	}

	nonActiveRevisions := []v1alpha1.IstioRevision{}
	for _, rev := range revList.Items {
		if isRevisionOwnedByIstio(rev, istio) && rev.Name != getActiveRevisionName(istio) {
			nonActiveRevisions = append(nonActiveRevisions, rev)
		}
	}
	return nonActiveRevisions, nil
}

func isRevisionOwnedByIstio(rev v1alpha1.IstioRevision, istio *v1alpha1.Istio) bool {
	for _, owner := range rev.OwnerReferences {
		if owner.UID == istio.UID {
			return true
		}
	}
	return false
}

func getActiveRevisionKey(istio *v1alpha1.Istio) types.NamespacedName {
	return types.NamespacedName{
		Name: getActiveRevisionName(istio),
	}
}

func getActiveRevisionName(istio *v1alpha1.Istio) string {
	var strategy v1alpha1.UpdateStrategyType
	if istio.Spec.UpdateStrategy != nil {
		strategy = istio.Spec.UpdateStrategy.Type
	}

	switch strategy {
	default:
		fallthrough
	case v1alpha1.UpdateStrategyTypeInPlace:
		return istio.Name
	case v1alpha1.UpdateStrategyTypeRevisionBased:
		return istio.Name + "-" + strings.ReplaceAll(istio.Spec.Version, ".", "-")
	}
}

func computeIstioRevisionValues(istio v1alpha1.Istio, defaultProfiles []string, resourceDir string) (helm.HelmValues, error) {
	// 1. start with values aggregated from default profiles and the profile in Istio.spec.profile
	values, err := getValuesFromProfiles(getProfilesDir(resourceDir, istio), getProfiles(istio, defaultProfiles))
	if err != nil {
		return nil, err
	}

	// 2. apply values from Istio.spec.values, overwriting values from profiles
	values = mergeOverwrite(values, istio.Spec.GetValues())

	// 3. apply values from Istio.spec.rawValues, overwriting the current values
	values = mergeOverwrite(values, istio.Spec.GetRawValues())

	// 4. override values that are not configurable by the user
	return applyOverrides(&istio, values)
}

func getProfiles(istio v1alpha1.Istio, defaultProfiles []string) []string {
	if istio.Spec.Profile == "" {
		return defaultProfiles
	}
	return append(defaultProfiles, istio.Spec.Profile)
}

func getProfilesDir(resourceDir string, istio v1alpha1.Istio) string {
	return path.Join(resourceDir, istio.Spec.Version, "profiles")
}

func applyOverrides(istio *v1alpha1.Istio, values helm.HelmValues) (helm.HelmValues, error) {
	revisionName := getActiveRevisionName(istio)

	// Set revision name to "" if revision name is "default". This is a temporary fix until we fix the injection
	// mutatingwebhook manifest; the webhook performs injection on namespaces labeled with "istio-injection: enabled"
	// only when revision is "", but not also for "default", which it should, since elsewhere in the same manifest,
	// the "" revision is mapped to "default".
	if revisionName == "default" {
		revisionName = ""
	}
	if err := values.Set("revision", revisionName); err != nil {
		return nil, err
	}

	if err := values.Set("global.istioNamespace", istio.Spec.Namespace); err != nil {
		return nil, err
	}
	return values, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IstioReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Istio{}).
		Owns(&v1alpha1.IstioRevision{}).
		Complete(r)
}

func (r *IstioReconciler) updateStatus(ctx context.Context, istio *v1alpha1.Istio, reconciliationErr error) error {
	status := istio.Status.DeepCopy()
	status.ObservedGeneration = istio.Generation

	if reconciliationErr != nil {
		status.SetCondition(v1alpha1.IstioCondition{
			Type:    v1alpha1.IstioConditionTypeReconciled,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.IstioConditionReasonReconcileError,
			Message: reconciliationErr.Error(),
		})
		status.SetCondition(v1alpha1.IstioCondition{
			Type:    v1alpha1.IstioConditionTypeReady,
			Status:  metav1.ConditionUnknown,
			Reason:  v1alpha1.IstioConditionReasonReconcileError,
			Message: "cannot determine readiness due to reconciliation error",
		})
		status.State = v1alpha1.IstioConditionReasonReconcileError
	} else {
		rev, err := r.getActiveRevision(ctx, istio)
		if errors.IsNotFound(err) {
			revisionNotFound := func(conditionType v1alpha1.IstioConditionType) v1alpha1.IstioCondition {
				return v1alpha1.IstioCondition{
					Type:    conditionType,
					Status:  metav1.ConditionFalse,
					Reason:  v1alpha1.IstioConditionReasonIstioRevisionNotFound,
					Message: "active IstioRevision not found",
				}
			}

			status.SetCondition(revisionNotFound(v1alpha1.IstioConditionTypeReconciled))
			status.SetCondition(revisionNotFound(v1alpha1.IstioConditionTypeReady))
			status.State = v1alpha1.IstioConditionReasonIstioRevisionNotFound
		} else if err == nil {
			status.SetCondition(convertCondition(rev.Status.GetCondition(v1alpha1.IstioRevisionConditionTypeReconciled)))
			status.SetCondition(convertCondition(rev.Status.GetCondition(v1alpha1.IstioRevisionConditionTypeReady)))
			status.State = convertConditionReason(rev.Status.State)
		} else {
			return err
		}
	}

	if reflect.DeepEqual(istio.Status, *status) {
		return nil
	}

	statusErr := r.Client.Status().Patch(ctx, istio, kube.NewStatusPatch(*status))
	if statusErr != nil {
		return statusErr
	}
	return reconciliationErr
}

func convertCondition(condition v1alpha1.IstioRevisionCondition) v1alpha1.IstioCondition {
	return v1alpha1.IstioCondition{
		Type:    convertConditionType(condition),
		Status:  condition.Status,
		Reason:  convertConditionReason(condition.Reason),
		Message: condition.Message,
	}
}

func convertConditionType(condition v1alpha1.IstioRevisionCondition) v1alpha1.IstioConditionType {
	switch condition.Type {
	case v1alpha1.IstioRevisionConditionTypeReconciled:
		return v1alpha1.IstioConditionTypeReconciled
	case v1alpha1.IstioRevisionConditionTypeReady:
		return v1alpha1.IstioConditionTypeReady
	default:
		panic(fmt.Sprintf("can't convert IstioRevisionConditionType: %s", condition.Type))
	}
}

func convertConditionReason(reason v1alpha1.IstioRevisionConditionReason) v1alpha1.IstioConditionReason {
	switch reason {
	case "":
		return ""
	case v1alpha1.IstioRevisionConditionReasonIstiodNotReady:
		return v1alpha1.IstioConditionReasonIstiodNotReady
	case v1alpha1.IstioRevisionConditionReasonCNINotReady:
		return v1alpha1.IstioConditionReasonCNINotReady
	case v1alpha1.IstioRevisionConditionReasonHealthy:
		return v1alpha1.IstioConditionReasonHealthy
	case v1alpha1.IstioRevisionConditionReasonReconcileError:
		return v1alpha1.IstioConditionReasonReconcileError
	default:
		panic(fmt.Sprintf("can't convert IstioRevisionConditionReason: %s", reason))
	}
}

func getValuesFromProfiles(profilesDir string, profiles []string) (helm.HelmValues, error) {
	// start with an empty values map
	values := helm.HelmValues{}

	// apply profiles in order, overwriting values from previous profiles
	alreadyApplied := sets.New[string]()
	for _, profile := range profiles {
		if profile == "" {
			return nil, fmt.Errorf("profile name cannot be empty")
		}
		if alreadyApplied.Contains(profile) {
			continue
		}
		alreadyApplied.Insert(profile)

		file := path.Join(profilesDir, profile+".yaml")
		// prevent path traversal attacks
		if path.Dir(file) != profilesDir {
			return nil, fmt.Errorf("invalid profile name %s", profile)
		}

		profileValues, err := getProfileValues(file)
		if err != nil {
			return nil, err
		}
		values = mergeOverwrite(values, profileValues)
	}

	return values, nil
}

func getProfileValues(file string) (helm.HelmValues, error) {
	fileContents, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file %v: %v", file, err)
	}

	var profile map[string]any
	err = yaml.Unmarshal(fileContents, &profile)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile YAML %s: %v", file, err)
	}

	val, found, err := unstructured.NestedFieldNoCopy(profile, "spec", "values")
	if !found || err != nil {
		return nil, err
	}
	m, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("spec.values is not a map[string]any")
	}
	return m, nil
}

func mergeOverwrite(base map[string]any, overrides map[string]any) map[string]any {
	if base == nil {
		base = make(map[string]any, 1)
	}

	for key, value := range overrides {
		// if the key doesn't already exist, add it
		if _, exists := base[key]; !exists {
			base[key] = value
			continue
		}

		// At this point, key exists in both base and overrides.
		// If both are maps, recurse so that we override only specific values in the map.
		// If only override value is a map, overwrite base value completely.
		// If both are values, overwrite base.
		childOverrides, overrideValueIsMap := value.(map[string]any)
		childBase, baseValueIsMap := base[key].(map[string]any)
		if baseValueIsMap && overrideValueIsMap {
			base[key] = mergeOverwrite(childBase, childOverrides)
		} else {
			base[key] = value
		}
	}
	return base
}
