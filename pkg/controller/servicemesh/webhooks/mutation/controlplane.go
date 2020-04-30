package mutation

import (
	"context"
	"fmt"
	"net/http"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

type ControlPlaneMutator struct {
	client          client.Client
	decoder         atypes.Decoder
	namespaceFilter webhookcommon.NamespaceFilter
}

func NewControlPlaneMutator(namespaceFilter webhookcommon.NamespaceFilter) *ControlPlaneMutator {
	return &ControlPlaneMutator{
		namespaceFilter: namespaceFilter,
	}
}

var _ admission.Handler = (*ControlPlaneMutator)(nil)
var _ inject.Client = (*ControlPlaneMutator)(nil)
var _ inject.Decoder = (*ControlPlaneMutator)(nil)

func (v *ControlPlaneMutator) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	log := logf.Log.WithName("smcp-mutator").
		WithValues("ServiceMeshControlPlane", webhookcommon.ToNamespacedName(req.AdmissionRequest))
	smcp := &maistrav1.ServiceMeshControlPlane{}

	err := v.decoder.Decode(req, smcp)
	if err != nil {
		log.Error(err, "error decoding admission request")
		return admission.ErrorResponse(http.StatusBadRequest, err)
	} else if smcp.ObjectMeta.DeletionTimestamp != nil {
		log.Info("skipping deleted smcp resource")
		return admission.ValidationResponse(true, "")
	}

	// do we care about this object?
	if !v.namespaceFilter.Watching(smcp.Namespace) {
		log.Info(fmt.Sprintf("operator is not watching namespace '%s'", smcp.Namespace))
		return admission.ValidationResponse(true, "")
	}

	newSmcp := smcp.DeepCopy()
	smcpMutated := false

	// on create we set the version to the current default version
	// on update we leave the version intact to preserve the v1.0 version
	// implied by the missing version field, which we added in version v1.1
	if smcp.Spec.Version == "" {
		switch req.AdmissionRequest.Operation {
		case admissionv1beta1.Create:
			log.Info("Setting .spec.version to default value", "version", versions.DefaultVersion.String())
			newSmcp.Spec.Version = versions.DefaultVersion.String()
			smcpMutated = true
		case admissionv1beta1.Update:
			log.Info("Setting .spec.version to default value", "version", smcp.Spec.Version)
			newSmcp.Spec.Version = smcp.Spec.Version
			smcpMutated = true
		}
	}

	if smcp.Spec.Template == "" {
		log.Info("Setting .spec.template to default value", "template", maistrav1.DefaultTemplate)
		newSmcp.Spec.Template = maistrav1.DefaultTemplate
		smcpMutated = true
	}

	if smcpMutated {
		return admission.PatchResponse(smcp, newSmcp)
	}
	return admission.ValidationResponse(true, "")
}

// InjectClient injects the client.
func (v *ControlPlaneMutator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *ControlPlaneMutator) InjectDecoder(d atypes.Decoder) error {
	v.decoder = d
	return nil
}
