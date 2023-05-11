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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "maistra.io/istio-operator/api/v1"
	"maistra.io/istio-operator/pkg/common"
	"maistra.io/istio-operator/pkg/helm"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// IstioHelmInstallReconciler reconciles a IstioHelmInstall object
type IstioHelmInstallReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=maistra.io,resources=istiohelminstalls,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=maistra.io,resources=istiohelminstalls/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=maistra.io,resources=istiohelminstalls/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources="*",verbs="*"
//+kubebuilder:rbac:groups="networking.k8s.io",resources="networkpolicies",verbs="*"
//+kubebuilder:rbac:groups="policy",resources="poddisruptionbudgets",verbs="*"
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs="*"
//+kubebuilder:rbac:groups="apps",resources=deployments;daemonsets,verbs="*"
//+kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs="*"
//+kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs="*"
//+kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs="*"

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *IstioHelmInstallReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("reconciler")
	var ihi v1.IstioHelmInstall
	if err := r.Client.Get(ctx, req.NamespacedName, &ihi); err != nil {
		if errors.IsNotFound(err) {
			logger.V(2).Info("IstioHelmInstall not found. Skipping reconciliation")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get IstioHelmInstall from cluster")
	}

	if ihi.DeletionTimestamp != nil {
		_, err := helm.UninstallChart(req.Namespace, req.Name+"-base")
		if err != nil {
			return ctrl.Result{}, err
		}

		_, err = helm.UninstallChart(req.Namespace, req.Name+"-istiod")
		if err != nil {
			return ctrl.Result{}, err
		}
		_, err = helm.UninstallChart(req.Namespace, req.Name+"-cni")
		if err != nil {
			return ctrl.Result{}, err
		}
		err = common.RemoveFinalizer(ctx, &ihi, r.Client)
		if err != nil {
			logger.Info("failed to remove finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	if !common.HasFinalizer(&ihi) {
		err := common.AddFinalizer(ctx, &ihi, r.Client)
		if err != nil {
			logger.Info("failed to add finalizer")
			return ctrl.Result{}, err
		}
	}
	values := ihi.Spec.GetValues()
	_, found, err := unstructured.NestedString(values, "global", "istioNamespace")
	if !found || err != nil {
		_ = unstructured.SetNestedField(values, ihi.Namespace, "global", "istioNamespace")
	}
	_ = unstructured.SetNestedField(values, true, "istio_cni", "enabled")
	_ = unstructured.SetNestedField(values, common.Config.Images3_0.CNI, "cni", "image")
	_ = unstructured.SetNestedField(values, common.Config.Images3_0.Istiod, "pilot", "image")
	_ = unstructured.SetNestedField(values, common.Config.Images3_0.Proxy, "global", "proxy", "image")
	_ = unstructured.SetNestedField(values, common.Config.Images3_0.Proxy, "global", "proxy_init", "image")

	logger.Info("Installing components", "values", values)

	_, err = helm.UpgradeOrInstallChart("istio-cni", ihi.Spec.Version, common.GetOperatorNamespace(), req.Name+"-cni", values)
	if err != nil {
		return ctrl.Result{}, err
	}

	_, err = helm.UpgradeOrInstallChart("base", ihi.Spec.Version, req.Namespace, req.Name+"-base", values)
	if err != nil {
		return ctrl.Result{}, err
	}

	_, err = helm.UpgradeOrInstallChart("istio-control/istio-discovery", ihi.Spec.Version, req.Namespace, req.Name+"-istiod", values)
	if err != nil {
		return ctrl.Result{}, err
	}

	_, err = helm.UpgradeOrInstallChart("gateways/istio-ingress", ihi.Spec.Version, req.Namespace, req.Name+"-ingress", values)
	if err != nil {
		return ctrl.Result{}, err
	}

	_, err = helm.UpgradeOrInstallChart("gateways/istio-egress", ihi.Spec.Version, req.Namespace, req.Name+"-egress", values)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IstioHelmInstallReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.IstioHelmInstall{}).
		Complete(r)
}
