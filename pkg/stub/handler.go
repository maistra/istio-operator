package stub

import (
	"context"

	"github.com/maistra/istio-operator/pkg/apis/istio/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"bytes"
	"reflect"
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
	defaultIstioVersion = "0.1.0"
	defaultDeploymentType = "origin"

	newline = "\n"

	istioInstallerCRName = "istio-installation"

	istioInstalledState = "Istio Installer Job Created"
)

func NewHandler(openShiftRelease string, masterPublicURL string) sdk.Handler {
	return &Handler{
		openShiftRelease: openShiftRelease,
		masterPublicURL: masterPublicURL,
	}
}

type Handler struct {
	name string
	// It is likely possible to determine these at runtime, we should investigate
	openShiftRelease string
	masterPublicURL string
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.Installation:
		if o.Name == istioInstallerCRName {
			if event.Deleted {
				logrus.Infof("Removing the Istio installation")
				installItems := h.newInstallerJobItems(o)
				h.deleteItems(installItems)
				items := h.newRemovalJobItems(o)
				if h.removalJobExists() {
					if err := h.updateItems(items); err != nil {
						logrus.Errorf("Failed to update the istio removal job: %v", err)
						return err
					}
				} else {
					if err := h.createItems(items); err != nil {
						logrus.Errorf("Failed to create the istio removal job: %v", err)
						return err
					}
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
				items := h.newInstallerJobItems(o)
				if h.installerJobExists() {
					if err := h.updateItems(items); err != nil {
						logrus.Errorf("Failed to update the istio installer job: %v", err)
						return err
					}
				} else {
					if err := h.createItems(items); err != nil {
						logrus.Errorf("Failed to create the istio installer job: %v", err)
						return err
					}
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

func (h *Handler) deleteItems(items []sdk.Object) {
	lastItem := len(items)-1
	for i := range items {
		sdk.Delete(items[lastItem-i])
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

func (h *Handler) updateItems(items []sdk.Object) error {
	for _, item := range items {
		if err := sdk.Update(item); err != nil {
			h.deleteItems(items)
			return err
		}
	}
	return nil
}

func (h *Handler) getIstioImagePrefix(cr *v1alpha1.Installation) string {
	if cr.Spec != nil && cr.Spec.Istio != nil && cr.Spec.Istio.Prefix != nil {
		return *cr.Spec.Istio.Prefix
	} else {
		return defaultIstioPrefix
	}
}

func (h *Handler) getIstioImageVersion(cr *v1alpha1.Installation) string {
	if cr.Spec != nil && cr.Spec.Istio != nil && cr.Spec.Istio.Version != nil {
		return *cr.Spec.Istio.Version
	} else {
		return defaultIstioVersion
	}
}

func (h *Handler) getDeploymentType(cr *v1alpha1.Installation) string {
	if cr.Spec != nil && cr.Spec.DeploymentType != nil {
		return *cr.Spec.DeploymentType
	} else {
		return defaultDeploymentType
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