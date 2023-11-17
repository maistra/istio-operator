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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"reflect"
	"regexp"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/helm"
	"maistra.io/istio-operator/pkg/kube"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"istio.io/istio/pkg/ptr"
	"istio.io/istio/pkg/util/sets"
)

// IstioReconciler reconciles a Istio object
type IstioReconciler struct {
	ResourceDirectory string
	DefaultProfiles   []string
	RestClientGetter  genericclioptions.RESTClientGetter
	client.Client
	Scheme *runtime.Scheme
}

func NewIstioReconciler(client client.Client, scheme *runtime.Scheme, restConfig *rest.Config, resourceDir string, defaultProfiles []string) *IstioReconciler {
	return &IstioReconciler{
		ResourceDirectory: resourceDir,
		DefaultProfiles:   defaultProfiles,
		RestClientGetter:  helm.NewRESTClientGetter(restConfig),
		Client:            client,
		Scheme:            scheme,
	}
}

// charts to deploy in the istio namespace
var userCharts = []string{"base", "istiod"}

// +kubebuilder:rbac:groups=operator.istio.io,resources=istios,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.istio.io,resources=istios/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.istio.io,resources=istios/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources="*",verbs="*"
// +kubebuilder:rbac:groups="networking.k8s.io",resources="networkpolicies",verbs="*"
// +kubebuilder:rbac:groups="policy",resources="poddisruptionbudgets",verbs="*"
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs="*"
// +kubebuilder:rbac:groups="apps",resources=deployments;daemonsets,verbs="*"
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs="*"
// +kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs="*"
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups="k8s.cni.cncf.io",resources=network-attachment-definitions,verbs="*"
// +kubebuilder:rbac:groups="security.openshift.io",resources=securitycontextconstraints,resourceNames=privileged,verbs=use
// +kubebuilder:rbac:groups="networking.istio.io",resources=envoyfilters,verbs="*"

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
		if err := r.uninstallHelmCharts(&istio); err != nil {
			return ctrl.Result{}, err
		}

		if err := kube.RemoveFinalizer(ctx, &istio, r.Client); err != nil {
			logger.Info("failed to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if istio.Spec.Version == "" {
		return ctrl.Result{}, fmt.Errorf("no spec.version set")
	}

	if !kube.HasFinalizer(&istio) {
		err := kube.AddFinalizer(ctx, &istio, r.Client)
		if err != nil {
			logger.Info("failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	values, err := getAggregatedValues(istio, r.DefaultProfiles, r.ResourceDirectory)
	if err != nil {
		err = r.updateStatus(ctx, logger, &istio, istio.Spec.GetValues(), err)
		return ctrl.Result{}, err
	}

	logger.Info("Installing components", "values", values)
	err = r.installHelmCharts(ctx, &istio, values)

	logger.Info("Reconciliation done. Updating status.")
	err = r.updateStatus(ctx, logger, &istio, values, err)

	return ctrl.Result{}, err
}

func getAggregatedValues(istio v1alpha1.Istio, defaultProfiles []string, resourceDir string) (helm.HelmValues, error) {
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
	if err := values.Set("global.istioNamespace", istio.Namespace); err != nil {
		return nil, err
	}
	return values, nil
}

func (r *IstioReconciler) installHelmCharts(ctx context.Context, istio *v1alpha1.Istio, values helm.HelmValues) error {
	ownerReference := metav1.OwnerReference{
		APIVersion:         v1alpha1.GroupVersion.String(),
		Kind:               v1alpha1.IstioKind,
		Name:               istio.Name,
		UID:                istio.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	if cniEnabled, err := isCNIEnabled(values); err != nil {
		return err
	} else if cniEnabled {
		if err := helm.UpgradeOrInstallCharts(ctx, r.RestClientGetter, []string{"cni"}, values,
			istio.Spec.Version, istio.Name, istio.Namespace, ownerReference); err != nil {
			return err
		}
	}

	if err := helm.UpgradeOrInstallCharts(ctx, r.RestClientGetter, userCharts, values,
		istio.Spec.Version, istio.Name, istio.Namespace, ownerReference); err != nil {
		return err
	}
	return nil
}

func (r *IstioReconciler) uninstallHelmCharts(istio *v1alpha1.Istio) error {
	if err := helm.UninstallCharts(r.RestClientGetter, []string{"cni"}, istio.Name, istio.Namespace); err != nil {
		return err
	}

	if err := helm.UninstallCharts(r.RestClientGetter, userCharts, istio.Name, istio.Namespace); err != nil {
		return err
	}
	return nil
}

func isCNIEnabled(values helm.HelmValues) (bool, error) {
	enabled, _, err := values.GetBool("istio_cni.enabled")
	return enabled, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *IstioReconciler) SetupWithManager(mgr ctrl.Manager) error {
	clusterScopedResourceHandler := handler.EnqueueRequestsFromMapFunc(mapOwnerAnnotationsToReconcileRequest)

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Istio{}).

		// namespaced resources
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.Endpoints{}).
		Owns(&corev1.ResourceQuota{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&networkingv1alpha3.EnvoyFilter{}).

		// TODO: only register NetAttachDef if the CRD is installed (may also need to watch for CRD creation)
		// Owns(&multusv1.NetworkAttachmentDefinition{}).

		// cluster-scoped resources
		Watches(&rbacv1.ClusterRole{}, clusterScopedResourceHandler).
		Watches(&rbacv1.ClusterRoleBinding{}, clusterScopedResourceHandler).
		Watches(&admissionv1.MutatingWebhookConfiguration{}, clusterScopedResourceHandler).
		Watches(&admissionv1.ValidatingWebhookConfiguration{},
			clusterScopedResourceHandler,
			builder.WithPredicates(validatingWebhookConfigPredicate{})).

		// +lint-watches:ignore: CustomResourceDefinition (prevents `make lint-watches` from bugging us about CRDs)
		Complete(r)
}

func (r *IstioReconciler) updateStatus(ctx context.Context, log logr.Logger, istio *v1alpha1.Istio, values helm.HelmValues, err error) error {
	reconciledCondition := determineReconciledCondition(err)
	readyCondition, err := r.determineReadyCondition(ctx, istio)
	if err != nil {
		return err
	}

	status := istio.Status.DeepCopy()
	status.ObservedGeneration = istio.Generation
	status.SetCondition(reconciledCondition)
	status.SetCondition(readyCondition)
	status.State = deriveState(reconciledCondition, readyCondition)

	appliedValues, err2 := json.Marshal(values)
	if err2 != nil {
		log.Error(err2, "failed to marshal status")
		if err == nil {
			return err2
		}
		return err
	}
	status.AppliedValues = appliedValues

	statusErr := r.Client.Status().Patch(ctx, istio, kube.NewStatusPatch(*status))
	if statusErr != nil {
		log.Error(statusErr, "failed to patch status")

		// ensure that we retry the reconcile by returning the status error
		// (but without overriding the original error)
		if err == nil {
			return statusErr
		}
	}
	return err
}

func deriveState(reconciledCondition, readyCondition v1alpha1.IstioCondition) v1alpha1.IstioConditionReason {
	if reconciledCondition.Status == metav1.ConditionFalse {
		return reconciledCondition.Reason
	} else if readyCondition.Status == metav1.ConditionFalse {
		return readyCondition.Reason
	}

	return v1alpha1.ConditionReasonHealthy
}

func determineReconciledCondition(err error) v1alpha1.IstioCondition {
	if err == nil {
		return v1alpha1.IstioCondition{
			Type:   v1alpha1.ConditionTypeReconciled,
			Status: metav1.ConditionTrue,
		}
	}

	return v1alpha1.IstioCondition{
		Type:    v1alpha1.ConditionTypeReconciled,
		Status:  metav1.ConditionFalse,
		Reason:  v1alpha1.ConditionReasonReconcileError,
		Message: fmt.Sprintf("error reconciling resource: %v", err),
	}
}

func (r *IstioReconciler) determineReadyCondition(ctx context.Context, istio *v1alpha1.Istio) (v1alpha1.IstioCondition, error) {
	notReady := func(reason v1alpha1.IstioConditionReason, message string) v1alpha1.IstioCondition {
		return v1alpha1.IstioCondition{
			Type:    v1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		}
	}

	istiod := appsv1.Deployment{}
	if err := r.Client.Get(ctx, istiodDeploymentKey(istio), &istiod); err != nil {
		if errors.IsNotFound(err) {
			return notReady(v1alpha1.ConditionReasonIstiodNotReady, "istiod Deployment not found"), nil
		}
		return notReady(v1alpha1.ConditionReasonReconcileError, fmt.Sprintf("failed to get readiness: %v", err)), nil
	}

	if istiod.Status.Replicas == 0 {
		return notReady(v1alpha1.ConditionReasonIstiodNotReady, "istiod Deployment is scaled to zero replicas"), nil
	} else if istiod.Status.ReadyReplicas < istiod.Status.Replicas {
		return notReady(v1alpha1.ConditionReasonIstiodNotReady, "not all istiod pods are ready"), nil
	}

	if cniEnabled, err := isCNIEnabled(istio.Spec.GetValues()); err != nil {
		return v1alpha1.IstioCondition{}, err
	} else if cniEnabled {
		cni := appsv1.DaemonSet{}
		if err := r.Client.Get(ctx, cniDaemonSetKey(istio), &cni); err != nil {
			if errors.IsNotFound(err) {
				return notReady(v1alpha1.ConditionReasonCNINotReady, "istio-cni-node DaemonSet not found"), nil
			}
			return notReady(v1alpha1.ConditionReasonReconcileError, fmt.Sprintf("failed to get readiness: %v", err)), nil
		}

		if cni.Status.CurrentNumberScheduled == 0 {
			return notReady(v1alpha1.ConditionReasonCNINotReady, "no istio-cni-node pods are currently scheduled"), nil
		} else if cni.Status.NumberReady < cni.Status.CurrentNumberScheduled {
			return notReady(v1alpha1.ConditionReasonCNINotReady, "not all istio-cni-node pods are ready"), nil
		}
	}

	return v1alpha1.IstioCondition{
		Type:   v1alpha1.ConditionTypeReady,
		Status: metav1.ConditionTrue,
	}, nil
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

func cniDaemonSetKey(istio *v1alpha1.Istio) client.ObjectKey {
	return client.ObjectKey{
		Namespace: istio.Namespace,
		Name:      "istio-cni-node",
	}
}

func istiodDeploymentKey(istio *v1alpha1.Istio) client.ObjectKey {
	return client.ObjectKey{
		Namespace: istio.Namespace,
		Name:      "istiod",
	}
}

func mapOwnerAnnotationsToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	namespacedName, kind, apiGroup := helm.GetOwnerFromAnnotations(annotations)
	if namespacedName != nil && kind == v1alpha1.IstioKind && apiGroup == v1alpha1.GroupVersion.Group {
		return []reconcile.Request{{NamespacedName: *namespacedName}}
	}
	return nil
}

type validatingWebhookConfigPredicate struct {
	predicate.Funcs
}

func (v validatingWebhookConfigPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	if matched, _ := regexp.MatchString("istiod-.*-validator|istio-validator.*", e.ObjectNew.GetName()); matched {
		// Istiod updates the caBundle and failurePolicy fields in istiod-<ns>-validator and istio-validator[-<rev>]-<ns>
		// webhook configs. We must ignore changes to these fields to prevent an endless update loop.
		clearIgnoredFields(e.ObjectOld)
		clearIgnoredFields(e.ObjectNew)
		return !reflect.DeepEqual(e.ObjectNew, e.ObjectOld)
	}
	return true
}

func clearIgnoredFields(obj client.Object) {
	obj.SetResourceVersion("")
	obj.SetGeneration(0)
	obj.SetManagedFields(nil)
	if webhookConfig, ok := obj.(*admissionv1.ValidatingWebhookConfiguration); ok {
		for i := 0; i < len(webhookConfig.Webhooks); i++ {
			webhookConfig.Webhooks[i].FailurePolicy = nil
		}
	}
}
