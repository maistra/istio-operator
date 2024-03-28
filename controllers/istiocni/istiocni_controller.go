// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package istiocni

import (
	"context"
	"errors"
	"fmt"
	"path"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/profiles"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"istio.io/istio/pkg/ptr"
)

const (
	cniReleaseName = "istio-cni"
	finalizer      = constants.FinalizerName
)

// IstioCNIReconciler reconciles an IstioCNI object
type IstioCNIReconciler struct {
	ResourceDirectory string
	DefaultProfiles   []string
	client.Client
	Scheme       *runtime.Scheme
	ChartManager *helm.ChartManager
}

func NewIstioCNIReconciler(
	client client.Client, scheme *runtime.Scheme, resourceDir string, chartManager *helm.ChartManager, defaultProfiles []string,
) *IstioCNIReconciler {
	return &IstioCNIReconciler{
		ResourceDirectory: resourceDir,
		DefaultProfiles:   defaultProfiles,
		Client:            client,
		Scheme:            scheme,
		ChartManager:      chartManager,
	}
}

// +kubebuilder:rbac:groups=operator.istio.io,resources=istiocnis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.istio.io,resources=istiocnis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.istio.io,resources=istiocnis/finalizers,verbs=update
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
func (r *IstioCNIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	var cni v1alpha1.IstioCNI
	if err := r.Client.Get(ctx, req.NamespacedName, &cni); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(2).Info("IstioCNI not found. Skipping reconciliation")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get IstioCNI: %v", err)
	}

	if cni.DeletionTimestamp != nil {
		if err := r.uninstallHelmChart(ctx, &cni); err != nil {
			return ctrl.Result{}, err
		}
		return kube.RemoveFinalizer(ctx, r.Client, &cni, finalizer)
	}

	if !kube.HasFinalizer(&cni, finalizer) {
		return kube.AddFinalizer(ctx, r.Client, &cni, finalizer)
	}

	if err := validateIstioCNI(cni); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Installing components")
	reconcileErr := r.installHelmChart(ctx, &cni)

	log.Info("Reconciliation done. Updating status.")
	statusErr := r.updateStatus(ctx, &cni, reconcileErr)

	return ctrl.Result{}, errors.Join(reconcileErr, statusErr)
}

func validateIstioCNI(cni v1alpha1.IstioCNI) error {
	if cni.Spec.Version == "" {
		return fmt.Errorf("spec.version not set")
	}
	if cni.Spec.Namespace == "" {
		return fmt.Errorf("spec.namespace not set")
	}
	return nil
}

func (r *IstioCNIReconciler) installHelmChart(ctx context.Context, cni *v1alpha1.IstioCNI) error {
	ownerReference := metav1.OwnerReference{
		APIVersion:         v1alpha1.GroupVersion.String(),
		Kind:               v1alpha1.IstioCNIKind,
		Name:               cni.Name,
		UID:                cni.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	// get userValues from Istio.spec.values
	userValues := cni.Spec.Values

	// apply image digests from configuration, if not already set by user
	userValues = applyImageDigests(cni, userValues, config.Config)

	// apply userValues on top of defaultValues from profiles
	mergedHelmValues, err := profiles.Apply(getProfilesDir(r.ResourceDirectory, cni), getProfiles(cni, r.DefaultProfiles), helm.FromValues(userValues))
	if err != nil {
		return err
	}

	_, err = r.ChartManager.UpgradeOrInstallChart(ctx, r.getChartDir(cni), mergedHelmValues, cni.Spec.Namespace, cniReleaseName, ownerReference)
	return err
}

func (r *IstioCNIReconciler) getChartDir(cni *v1alpha1.IstioCNI) string {
	return path.Join(r.ResourceDirectory, cni.Spec.Version, "charts", "cni")
}

func getProfiles(cni *v1alpha1.IstioCNI, defaultProfiles []string) []string {
	if cni.Spec.Profile == "" {
		return defaultProfiles
	}
	return append(defaultProfiles, cni.Spec.Profile)
}

func getProfilesDir(resourceDir string, cni *v1alpha1.IstioCNI) string {
	return path.Join(resourceDir, cni.Spec.Version, "profiles")
}

func applyImageDigests(cni *v1alpha1.IstioCNI, values *v1alpha1.CNIValues, config config.OperatorConfig) *v1alpha1.CNIValues {
	imageDigests, digestsDefined := config.ImageDigests[cni.Spec.Version]
	// if we don't have default image digests defined for this version, it's a no-op
	if !digestsDefined {
		return values
	}

	if values == nil {
		values = &v1alpha1.CNIValues{}
	}

	// set image digest unless any part of the image has been configured by the user
	if values.Cni == nil {
		values.Cni = &v1alpha1.CNIConfig{}
	}
	if values.Cni.Image == "" && values.Cni.Hub == "" && values.Cni.Tag == nil {
		values.Cni.Image = imageDigests.CNIImage
	}
	return values
}

func (r *IstioCNIReconciler) uninstallHelmChart(ctx context.Context, cni *v1alpha1.IstioCNI) error {
	_, err := r.ChartManager.UninstallChart(ctx, cniReleaseName, cni.Spec.Namespace)
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *IstioCNIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// ownedResourceHandler handles resources that are owned by the IstioCNI CR
	ownedResourceHandler := handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1alpha1.IstioCNI{}, handler.OnlyControllerOwner())

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := mgr.GetLogger().WithName("ctrlr").WithName("istiocni")
				if req != nil {
					log = log.WithValues("IstioCNI", req.Name)
				}
				return log
			},
		}).
		For(&v1alpha1.IstioCNI{}).

		// namespaced resources
		Watches(&corev1.ConfigMap{}, ownedResourceHandler).
		Watches(&appsv1.DaemonSet{}, ownedResourceHandler).
		Watches(&corev1.ResourceQuota{}, ownedResourceHandler).
		Watches(&corev1.ServiceAccount{}, ownedResourceHandler).
		Watches(&rbacv1.RoleBinding{}, ownedResourceHandler).

		// TODO: only register NetAttachDef if the CRD is installed (may also need to watch for CRD creation)
		// Owns(&multusv1.NetworkAttachmentDefinition{}).

		// cluster-scoped resources
		Watches(&rbacv1.ClusterRole{}, ownedResourceHandler).
		Watches(&rbacv1.ClusterRoleBinding{}, ownedResourceHandler).
		Complete(r)
}

func (r *IstioCNIReconciler) determineStatus(ctx context.Context, cni *v1alpha1.IstioCNI, reconcileErr error) (*v1alpha1.IstioCNIStatus, error) {
	reconciledCondition := r.determineReconciledCondition(reconcileErr)
	readyCondition, err := r.determineReadyCondition(ctx, cni)
	if err != nil {
		return nil, err
	}

	status := cni.Status.DeepCopy()
	status.ObservedGeneration = cni.Generation
	status.SetCondition(reconciledCondition)
	status.SetCondition(readyCondition)
	status.State = deriveState(reconciledCondition, readyCondition)
	return status, nil
}

func (r *IstioCNIReconciler) updateStatus(ctx context.Context, cni *v1alpha1.IstioCNI, reconcileErr error) error {
	status, err := r.determineStatus(ctx, cni, reconcileErr)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(cni.Status, *status) {
		return nil
	}
	return r.Client.Status().Patch(ctx, cni, kube.NewStatusPatch(*status))
}

func deriveState(reconciledCondition, readyCondition v1alpha1.IstioCNICondition) v1alpha1.IstioCNIConditionReason {
	if reconciledCondition.Status != metav1.ConditionTrue {
		return reconciledCondition.Reason
	} else if readyCondition.Status != metav1.ConditionTrue {
		return readyCondition.Reason
	}
	return v1alpha1.IstioCNIReasonHealthy
}

func (r *IstioCNIReconciler) determineReconciledCondition(err error) v1alpha1.IstioCNICondition {
	c := v1alpha1.IstioCNICondition{Type: v1alpha1.IstioCNIConditionReconciled}

	if err == nil {
		c.Status = metav1.ConditionTrue
	} else {
		c.Status = metav1.ConditionFalse
		c.Reason = v1alpha1.IstioCNIReasonReconcileError
		c.Message = fmt.Sprintf("error reconciling resource: %v", err)
	}
	return c
}

func (r *IstioCNIReconciler) determineReadyCondition(ctx context.Context, cni *v1alpha1.IstioCNI) (v1alpha1.IstioCNICondition, error) {
	c := v1alpha1.IstioCNICondition{
		Type:   v1alpha1.IstioCNIConditionReady,
		Status: metav1.ConditionFalse,
	}

	ds := appsv1.DaemonSet{}
	if err := r.Client.Get(ctx, r.cniDaemonSetKey(cni), &ds); err == nil {
		if ds.Status.CurrentNumberScheduled == 0 {
			c.Reason = v1alpha1.IstioCNIDaemonSetNotReady
			c.Message = "no istio-cni-node pods are currently scheduled"
		} else if ds.Status.NumberReady < ds.Status.CurrentNumberScheduled {
			c.Reason = v1alpha1.IstioCNIDaemonSetNotReady
			c.Message = "not all istio-cni-node pods are ready"
		} else {
			c.Status = metav1.ConditionTrue
		}
	} else if apierrors.IsNotFound(err) {
		c.Reason = v1alpha1.IstioCNIDaemonSetNotReady
		c.Message = "istio-cni-node DaemonSet not found"
	} else {
		c.Status = metav1.ConditionUnknown
		c.Reason = v1alpha1.IstioCNIReasonReadinessCheckFailed
		c.Message = fmt.Sprintf("failed to get readiness: %v", err)
		return c, err
	}
	return c, nil
}

func (r *IstioCNIReconciler) cniDaemonSetKey(cni *v1alpha1.IstioCNI) client.ObjectKey {
	return client.ObjectKey{
		Namespace: cni.Spec.Namespace,
		Name:      "istio-cni-node",
	}
}
