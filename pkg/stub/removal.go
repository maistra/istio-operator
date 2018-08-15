package stub

import (
	"github.com/maistra/istio-operator/pkg/apis/istio/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"bytes"
	"k8s.io/api/batch/v1"
)

const (
	removalConfigMapName = "removal.istio.inventory"
	removalJobName = "openshift-ansible-istio-removal-job"
)

func (h *Handler) getRemovalJob(cr *v1alpha1.Installation) *v1.Job {
	return h.getJob(removalJobName, namespace)
}

func (h *Handler) newRemovalJobItems(cr *v1alpha1.Installation) []sdk.Object {
	return h.newJobItems(cr, removalJobName, removalConfigMapName, namespace, h.getRemovalInventory(cr))
}

func (h *Handler) getRemovalInventory(cr *v1alpha1.Installation) string {
	var b bytes.Buffer

	b.WriteString(`
[OSEv3:children]
masters

[OSEv3:vars]
openshift_istio_install=false`)
	b.WriteString(newline)
	addStringValue(&b, "openshift_release=", h.getOpenShiftRelease())

	b.WriteString(`
[masters]
127.0.0.1 ansible_connection=local
`)
	return b.String()
}