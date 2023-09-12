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
	"reflect"
	"regexp"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	maistrav1 "maistra.io/istio-operator/api/v1alpha1"
	"maistra.io/istio-operator/pkg/helm"
	"maistra.io/istio-operator/pkg/istio"
	"maistra.io/istio-operator/pkg/kube"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
)

// IstioReconciler reconciles a Istio object
type IstioReconciler struct {
	ResourceDirectory string
	RestClientGetter  genericclioptions.RESTClientGetter
	client.Client
	Scheme *runtime.Scheme
}

func NewIstioReconciler(client client.Client, scheme *runtime.Scheme, restConfig *rest.Config, resourceDir string) *IstioReconciler {
	return &IstioReconciler{
		ResourceDirectory: resourceDir,
		RestClientGetter:  helm.NewRESTClientGetter(restConfig),
		Client:            client,
		Scheme:            scheme,
	}
}

// charts to deploy in the operator namespace (and their suffixes)
var systemCharts = map[string]string{
	"istio-cni": "-cni",
}

// charts to deploy in the ihi namespace (and their suffixes)
var userCharts = map[string]string{
	"base":                          "-base",
	"istio-control/istio-discovery": "-istiod",
}

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
	var ihi maistrav1.Istio
	if err := r.Client.Get(ctx, req.NamespacedName, &ihi); err != nil {
		if errors.IsNotFound(err) {
			logger.V(2).Info("Istio not found. Skipping reconciliation")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get Istio from cluster")
	}

	if ihi.DeletionTimestamp != nil {
		err := helm.UninstallCharts(r.RestClientGetter, systemCharts, ihi.Name, kube.GetOperatorNamespace())
		if err != nil {
			return ctrl.Result{}, err
		}

		err = helm.UninstallCharts(r.RestClientGetter, userCharts, ihi.Name, ihi.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = kube.RemoveFinalizer(ctx, &ihi, r.Client)
		if err != nil {
			logger.Info("failed to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	if ihi.Spec.Version == "" {
		return ctrl.Result{}, fmt.Errorf("no spec.version set")
	}
	if !kube.HasFinalizer(&ihi) {
		err := kube.AddFinalizer(ctx, &ihi, r.Client)
		if err != nil {
			logger.Info("failed to add finalizer")
			return ctrl.Result{}, err
		}
	}
	s := istio.Maistra30Strategy{}
	err := s.ApplyDefaults(&ihi)
	if err != nil {
		logger.Error(err, "failed to apply default values. requeuing request")
		return ctrl.Result{Requeue: true}, nil
	}
	values := ihi.Spec.GetValues()

	logger.Info("Installing components", "values", values)
	defer func() {
		logger.Info("Reconciliation complete. Writing status")
		appliedValues, err := json.Marshal(values)
		if err != nil {
			logger.Error(err, "failed to marshal status")
			return
		}
		err = r.Client.Status().Patch(ctx, &ihi, kube.NewStatusPatch(maistrav1.IstioStatus{AppliedValues: appliedValues}))
		if err != nil {
			logger.Error(err, "failed to patch status")
		}
	}()

	ownerReference := metav1.OwnerReference{
		APIVersion:         maistrav1.GroupVersion.String(),
		Kind:               maistrav1.IstioKind,
		Name:               ihi.Name,
		UID:                ihi.UID,
		Controller:         pointer.Bool(true),
		BlockOwnerDeletion: pointer.Bool(true),
	}

	err = helm.UpgradeOrInstallCharts(ctx, r.RestClientGetter, systemCharts, values,
		ihi.Spec.Version, ihi.Name, kube.GetOperatorNamespace(), ownerReference, ihi.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = helm.UpgradeOrInstallCharts(ctx, r.RestClientGetter, userCharts, values,
		ihi.Spec.Version, ihi.Name, ihi.Namespace, ownerReference, ihi.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IstioReconciler) SetupWithManager(mgr ctrl.Manager) error {
	clusterScopedResourceHandler := handler.EnqueueRequestsFromMapFunc(mapOwnerAnnotationsToReconcileRequest)

	return ctrl.NewControllerManagedBy(mgr).
		For(&maistrav1.Istio{}).

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
		Watches(sourceForKind(&rbacv1.ClusterRole{}), clusterScopedResourceHandler).
		Watches(sourceForKind(&rbacv1.ClusterRoleBinding{}), clusterScopedResourceHandler).
		Watches(sourceForKind(&admissionv1.MutatingWebhookConfiguration{}), clusterScopedResourceHandler).
		Watches(sourceForKind(&admissionv1.ValidatingWebhookConfiguration{}),
			clusterScopedResourceHandler,
			builder.WithPredicates(validatingWebhookConfigPredicate{})).

		// +lint-watches:ignore: CustomResourceDefinition (prevents `make lint-watches` from bugging us about CRDs)
		Complete(r)
}

func mapOwnerAnnotationsToReconcileRequest(obj client.Object) []reconcile.Request {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	namespacedName, kind, apiGroup := helm.GetOwnerFromAnnotations(annotations)
	if namespacedName != nil && kind == maistrav1.IstioKind && apiGroup == maistrav1.GroupVersion.Group {
		return []reconcile.Request{{NamespacedName: *namespacedName}}
	}
	return nil
}

func sourceForKind(obj client.Object) source.Source {
	return &source.Kind{
		Type: obj,
	}
}

type validatingWebhookConfigPredicate struct {
	predicate.Funcs
}

func (v validatingWebhookConfigPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	if matched, _ := regexp.MatchString("istiod-.*-validator", e.ObjectNew.GetName()); matched {
		// Istiod updates the caBundle and failurePolicy fields in istiod-<ns>-validator webhook config.
		// We must ignore changes to these fields to prevent an endless update loop.
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
