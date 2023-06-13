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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	v1 "maistra.io/istio-operator/api/v1"
	"maistra.io/istio-operator/pkg/helm"
	"maistra.io/istio-operator/pkg/istio"
	"maistra.io/istio-operator/pkg/kube"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// IstioHelmInstallReconciler reconciles a IstioHelmInstall object
type IstioHelmInstallReconciler struct {
	ResourceDirectory string
	Config            *rest.Config
	client.Client
	Scheme *runtime.Scheme
}

// charts to deploy (with their suffixes)
var charts = map[string]string{
	"istio-cni":                     "-cni",
	"base":                          "-base",
	"istio-control/istio-discovery": "-istiod",
	"gateways/istio-ingress":        "-ingress",
	"gateways/istio-egress":         "-egress",
}

func namespaceForChart(chartName string, ihi v1.IstioHelmInstall) string {
	if chartName == "istio-cni" {
		return kube.GetOperatorNamespace()
	}
	return ihi.Namespace
}

// +kubebuilder:rbac:groups=maistra.io,resources=istiohelminstalls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=maistra.io,resources=istiohelminstalls/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=maistra.io,resources=istiohelminstalls/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources="*",verbs="*"
// +kubebuilder:rbac:groups="networking.k8s.io",resources="networkpolicies",verbs="*"
// +kubebuilder:rbac:groups="policy",resources="poddisruptionbudgets",verbs="*"
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs="*"
// +kubebuilder:rbac:groups="apps",resources=deployments;daemonsets,verbs="*"
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs="*"
// +kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs="*"
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs="*"
// +kubebuilder:rbac:groups="k8s.cni.cncf.io",resources=network-attachment-definitions,verbs="*"

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
		for chartName, suffix := range charts {
			_, err := helm.UninstallChart(r.Config, namespaceForChart(chartName, ihi), ihi.Name+suffix)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		err := kube.RemoveFinalizer(ctx, &ihi, r.Client)
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
		err = r.Client.Status().Patch(ctx, &ihi, kube.NewStatusPatch(v1.IstioHelmInstallStatus{AppliedValues: appliedValues}))
		if err != nil {
			logger.Error(err, "failed to patch status")
		}
	}()

	for chartName, suffix := range charts {
		_, err = helm.UpgradeOrInstallChart(ctx, r.Config, chartName, ihi.Spec.Version, namespaceForChart(chartName, ihi), req.Name+suffix, values)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IstioHelmInstallReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Config = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.IstioHelmInstall{}).
		Complete(r)
}
