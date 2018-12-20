package stub

import (
	"context"

	"github.com/maistra/istio-operator/pkg/apis/istio/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"bytes"
	"reflect"
	batchv1	"k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	namespace = "istio-system"
	serviceAccountName = "openshift-ansible"

	inventoryDir = "/tmp/inventory/"
	inventoryFile = inventoryDir + "istio.inventory"

	playbookFile = "playbooks/openshift-istio/config.yml"
	playbookOptions = "-vvv"

	configurationDir = "/etc/origin/master"

	defaultIstioPrefix = "docker.io/maistra/"
	defaultIstioVersion = "0.6.0"
	defaultDeploymentType = "origin"

	newline = "\n"

	istioInstallerCRName = "istio-installation"

	istioInstalledState = "Istio Installer Job Created"
)

func NewHandler(openShiftRelease string, masterPublicURL, istioPrefix, istioVersion, deploymentType string, alwaysPull bool) sdk.Handler {
	return &Handler{
		openShiftRelease: openShiftRelease,
		masterPublicURL: masterPublicURL,
		istioPrefix: istioPrefix,
		istioVersion: istioVersion,
		deploymentType: deploymentType,
		alwaysPull: alwaysPull,
	}
}

type Handler struct {
	name string
	// It is likely possible to determine these at runtime, we should investigate
	openShiftRelease string
	masterPublicURL  string
	istioPrefix      string
	istioVersion     string
	deploymentType   string
	alwaysPull       bool
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.Installation:
		if o.Name == istioInstallerCRName {
			if event.Deleted {
				logrus.Infof("Removing the Istio installation")
				if err := ensureProjectAndServiceAccount() ; err != nil {
					return err
				}
				removalJob := h.getRemovalJob(o)
				h.deleteJob(removalJob)
				installerJob := h.getInstallerJob(o)
				h.deleteJob(installerJob)
				items := h.newRemovalJobItems(o)
				if err := h.createItems(items); err != nil {
					logrus.Errorf("Failed to create the istio removal job: %v", err)
					return err
				}
			} else {
				if o.Status != nil && o.Status.State != nil {
					if *o.Status.State == istioInstalledState {
						if reflect.DeepEqual(o.Spec, o.Status.Spec) {
							logrus.Debugf("Ignoring installed state for %v %v", o.Kind, o.Name)
							return nil
						}
					} else {
						logrus.Infof("Reinstalling istio for %v %v", o.Kind, o.Name)
					}
				} else {
					logrus.Infof("Installing istio for %v %v", o.Kind, o.Name)
				}
				if err := ensureProjectAndServiceAccount() ; err != nil {
					return err
				}
				installerJob := h.getInstallerJob(o)
				h.deleteJob(installerJob)
				removalJob := h.getRemovalJob(o)
				h.deleteJob(removalJob)
				items := h.newInstallerJobItems(o)
				if err := h.createItems(items); err != nil {
					logrus.Errorf("Failed to create the istio installer job: %v", err)
					return err
				}
				state := istioInstalledState
				if o.Status == nil {
					o.Status = &v1alpha1.InstallationStatus {
						State: &state,
						Spec: o.Spec.DeepCopy(),
					}
				} else {
					o.Status.State = &state
					o.Status.Spec = o.Spec.DeepCopy()
				}
				if err := sdk.Update(o); err != nil {
					logrus.Errorf("Failed to update the installation state in the resource: %v", err)
				}
			}
		} else {
			logrus.Infof("Ignoring istio installer CR %v, please redeploy using the %v name", o.Name, istioInstallerCRName)
		}
	}
	return nil
}

func (h *Handler) deleteJob(job *batchv1.Job) {
	err := sdk.Get(job) ; if err == nil {
		uid := job.UID
		var parallelism int32 = 0
		job.Spec.Parallelism = &parallelism
		sdk.Update(job)
		podList := corev1.PodList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
		}

		labelSelector := labels.SelectorFromSet(labels.Set(map[string]string{"controller-uid": string(uid)}))
		listOptions := sdk.WithListOptions(&metav1.ListOptions{
			LabelSelector:        labelSelector.String(),
			IncludeUninitialized: false,
		})

		err := sdk.List(namespace, &podList, listOptions) ; if err == nil {
			for _, pod := range podList.Items {
				sdk.Delete(&pod)
			}
			orphanDependents := false
			sdk.Delete(job, sdk.WithDeleteOptions(&metav1.DeleteOptions{OrphanDependents: &orphanDependents}))
		}
	}
}

func (h *Handler) deleteItem(object sdk.Object) {
	switch item := object.(type) {
	case *batchv1.Job:
		h.deleteJob(item)
	default:
		sdk.Delete(item)
	}
}

func (h *Handler) deleteItems(items []sdk.Object) {
	lastItem := len(items)-1
	for i := range items {
		item:= items[lastItem-i]
		h.deleteItem(item)
	}
}

func (h *Handler) createItems(items []sdk.Object) error {
	for _, item := range items {
		if err := sdk.Create(item); err != nil {
			h.deleteItems(items)
			return err
		}
	}
	return nil
}

func (h *Handler) getIstioImagePrefix(cr *v1alpha1.Installation) string {
	if cr.Spec != nil && cr.Spec.Istio != nil && cr.Spec.Istio.Prefix != nil {
		return *cr.Spec.Istio.Prefix
	} else if h.istioPrefix != "" {
		return h.istioPrefix
	} else {
		return defaultIstioPrefix
	}
}

func (h *Handler) getIstioImageVersion(cr *v1alpha1.Installation) string {
	if cr.Spec != nil && cr.Spec.Istio != nil && cr.Spec.Istio.Version != nil {
		return *cr.Spec.Istio.Version
	} else if h.istioVersion != "" {
		return h.istioVersion
	} else {
		return defaultIstioVersion
	}
}

func (h *Handler) getDeploymentType(cr *v1alpha1.Installation) string {
	if cr.Spec != nil && cr.Spec.DeploymentType != nil {
		return *cr.Spec.DeploymentType
	} else if h.deploymentType != "" {
		return h.deploymentType
	} else {
		return defaultDeploymentType
	}
}

func (h *Handler) getAlwaysPull() corev1.PullPolicy {
	if h.alwaysPull {
		return corev1.PullAlways
	} else {
		return corev1.PullIfNotPresent
	}
}

func (h *Handler) getOpenShiftRelease() string {
	return h.openShiftRelease
}

func (h *Handler) getMasterPublicURL() *string {
	if h.masterPublicURL == "" {
		return nil
	}
	return &h.masterPublicURL
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
