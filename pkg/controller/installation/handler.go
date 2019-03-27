package installation

import (
	"bytes"
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/maistra/istio-operator/pkg/apis/istio/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace          = "istio-system"
	serviceAccountName = "openshift-ansible"

	inventoryDir  = "/tmp/inventory/"
	inventoryFile = inventoryDir + "istio.inventory"

	playbookFile    = "playbooks/openshift-istio/config.yml"
	playbookOptions = "-vvv"

	configurationDir = "/etc/origin/master"

	defaultIstioPrefix    = "docker.io/maistra/"
	defaultIstioVersion   = "0.10.0"
	defaultDeploymentType = "origin"

	newline = "\n"

	istioInstallerCRName = "istio-installation"

	istioInstalledState = "Istio Installer Job Created"
)

var (
	installationHandler *Handler
)

func RegisterHandler(h *Handler) {
	installationHandler = h
}

type Handler struct {
	// It is likely possible to determine these at runtime, we should investigate
	OpenShiftRelease string
	MasterPublicURL  string
	IstioPrefix      string
	IstioVersion     string
	DeploymentType   string
	AlwaysPull       bool
	Enable3Scale     bool
}

func (h *ReconcileInstallation) Handle(instance *v1alpha1.Installation) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", instance.Namespace, "Request.Name", instance.Name)
	reqLogger.Info("Reconciling Installation")

	deleted := instance.GetDeletionTimestamp() != nil
	finalizers := instance.GetFinalizers()
	finalizerIndex := indexOf(finalizers, finalizer)
	if !deleted && finalizerIndex < 0 {
		reqLogger.V(1).Info("Adding finalizer", "finalizer", finalizer)
		finalizers = append(finalizers, finalizer)
		instance.SetFinalizers(finalizers)
		err := h.client.Update(context.TODO(), instance)
		return reconcile.Result{}, err
	}

	if deleted {
		if finalizerIndex < 0 {
			// already deleted ourselves
			return reconcile.Result{}, nil
		}

		reqLogger.Info("Removing the Istio installation")
		if err := h.ensureProjectAndServiceAccount(); err != nil {
			return reconcile.Result{}, err
		}
		removalJob := h.getRemovalJob(instance)
		h.deleteJob(removalJob)
		installerJob := h.getInstallerJob(instance)
		h.deleteJob(installerJob)
		items := h.newRemovalJobItems(instance)
		if err := h.createItems(items); err != nil {
			reqLogger.Error(err, "Failed to create the istio removal job")
			return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
		}
		finalizers = append(finalizers[:finalizerIndex], finalizers[finalizerIndex+1:]...)
		instance.SetFinalizers(finalizers)
		err := h.client.Update(context.TODO(), instance)
		return reconcile.Result{}, err
	}
	if instance.Status != nil && instance.Status.State != nil {
		if *instance.Status.State == istioInstalledState {
			if reflect.DeepEqual(instance.Spec, instance.Status.Spec) {
				reqLogger.V(2).Info("Ignoring installed state for %v %v", instance.Kind, instance.Name)
				return reconcile.Result{}, nil
			}
		} else {
			reqLogger.Info("Reinstalling istio for %v %v", instance.Kind, instance.Name)
		}
	} else {
		reqLogger.Info("Installing istio for %v %v", instance.Kind, instance.Name)
	}

	if err := h.ensureProjectAndServiceAccount(); err != nil {
		return reconcile.Result{}, err
	}

	installerJob := h.getInstallerJob(instance)
	h.deleteJob(installerJob)
	removalJob := h.getRemovalJob(instance)
	h.deleteJob(removalJob)
	items := h.newInstallerJobItems(instance)
	if err := h.createItems(items); err != nil {
		reqLogger.Error(err, "Failed to create the istio installer job")
		// XXX: do we need to do something in the result?
		return reconcile.Result{}, err
	}
	state := istioInstalledState
	if instance.Status == nil {
		instance.Status = &v1alpha1.InstallationStatus{
			State: &state,
			Spec:  instance.Spec.DeepCopy(),
		}
	} else {
		instance.Status.State = &state
		instance.Status.Spec = instance.Spec.DeepCopy()
	}
	if err := h.client.Status().Update(context.TODO(), instance); err != nil {
		reqLogger.Error(err, "Failed to update the installation state in the resource")
		// XXX: do we need to do something in the result?
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (h *ReconcileInstallation) deleteJob(job *batchv1.Job) {
	objectKey, err := client.ObjectKeyFromObject(job)
	if err != nil {
		return
	}
	err = h.client.Get(context.TODO(), objectKey, job)
	if err == nil {
		uid := job.UID
		var parallelism int32 = 0
		job.Spec.Parallelism = &parallelism
		h.client.Update(context.TODO(), job)
		podList := corev1.PodList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
		}

		labelSelector := labels.SelectorFromSet(labels.Set(map[string]string{"controller-uid": string(uid)}))
		listOptions := &client.ListOptions{
			Raw: &metav1.ListOptions{
				LabelSelector:        labelSelector.String(),
				IncludeUninitialized: false,
			},
		}

		err := h.client.List(context.TODO(), listOptions, &podList)
		if err == nil {
			for _, pod := range podList.Items {
				h.client.Delete(context.TODO(), &pod)
			}
			orphanDependents := false
			h.client.Delete(context.TODO(), job, func(opts *client.DeleteOptions) { deleteOrphanedDependents(opts, &orphanDependents) })
		}
	}
}

func deleteOrphanedDependents(opts *client.DeleteOptions, orphanedDependents *bool) {
	raw := opts.Raw
	if raw == nil {
		raw = &metav1.DeleteOptions{}
		opts.Raw = raw
	}
	raw.OrphanDependents = orphanedDependents
}

func (h *ReconcileInstallation) deleteItem(object runtime.Object) {
	switch item := object.(type) {
	case *batchv1.Job:
		h.deleteJob(item)
	default:
		h.client.Delete(context.TODO(), item)
	}
}

func (h *ReconcileInstallation) deleteItems(items []runtime.Object) {
	lastItem := len(items) - 1
	for i := range items {
		item := items[lastItem-i]
		h.deleteItem(item)
	}
}

func (h *ReconcileInstallation) createItems(items []runtime.Object) error {
	for _, item := range items {
		if err := h.client.Create(context.TODO(), item); err != nil {
			h.deleteItems(items)
			return err
		}
	}
	return nil
}

func (h *Handler) getIstioImagePrefix(cr *v1alpha1.Installation) string {
	if cr.Spec != nil && cr.Spec.Istio != nil && cr.Spec.Istio.Prefix != nil {
		return *cr.Spec.Istio.Prefix
	} else if h.IstioPrefix != "" {
		return h.IstioPrefix
	} else {
		return defaultIstioPrefix
	}
}

func (h *Handler) getIstioImageVersion(cr *v1alpha1.Installation) string {
	if cr.Spec != nil && cr.Spec.Istio != nil && cr.Spec.Istio.Version != nil {
		return *cr.Spec.Istio.Version
	} else if h.IstioVersion != "" {
		return h.IstioVersion
	} else {
		return defaultIstioVersion
	}
}

func (h *Handler) getDeploymentType(cr *v1alpha1.Installation) string {
	if cr.Spec != nil && cr.Spec.DeploymentType != nil {
		return *cr.Spec.DeploymentType
	} else if h.DeploymentType != "" {
		return h.DeploymentType
	} else {
		return defaultDeploymentType
	}
}

func (h *Handler) getAlwaysPull() corev1.PullPolicy {
	if h.AlwaysPull {
		return corev1.PullAlways
	} else {
		return corev1.PullIfNotPresent
	}
}

func (h *Handler) getOpenShiftRelease() string {
	return h.OpenShiftRelease
}

func (h *Handler) getMasterPublicURL() *string {
	if h.MasterPublicURL == "" {
		return nil
	}
	return &h.MasterPublicURL
}

func (h *Handler) getThreeScaleEnabled(threeScale *v1alpha1.ThreeScaleSpec) bool {
	if threeScale != nil && threeScale.Enabled != nil {
		return *threeScale.Enabled
	} else {
		return h.Enable3Scale
	}
}

func addStringValue(b *bytes.Buffer, key string, value string) {
	b.WriteString(key)
	b.WriteString(value)
	b.WriteString(newline)
}

func addStringPtrValue(b *bytes.Buffer, key string, value *string) {
	if value != nil {
		addStringValue(b, key, *value)
	}
}

func addBooleanPtrValue(b *bytes.Buffer, key string, value *bool) {
	if value != nil {
		addBooleanValue(b, key, *value)
	}
}

func addBooleanValue(b *bytes.Buffer, key string, value bool) {
	if value {
		addStringValue(b, key, "True")
	} else {
		addStringValue(b, key, "False")
	}
}

func addInt32PtrValue(b *bytes.Buffer, key string, value *int32) {
	if value != nil {
		addInt32Value(b, key, *value)
	}
}

func addInt32Value(b *bytes.Buffer, key string, value int32) {
	addStringValue(b, key, strconv.FormatInt(int64(value), 10))
}

func addIntPtrValue(b *bytes.Buffer, key string, value *int) {
	if value != nil {
		addIntValue(b, key, *value)
	}
}

func addIntValue(b *bytes.Buffer, key string, value int) {
	addStringValue(b, key, strconv.FormatInt(int64(value), 10))
}
