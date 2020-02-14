package mutation

import (
	"context"
	"fmt"
	"net/http"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

type MemberRollMutator struct {
	client          client.Client
	decoder         atypes.Decoder
	namespaceFilter webhookcommon.NamespaceFilter
}

func NewMemberRollMutator(namespaceFilter webhookcommon.NamespaceFilter) *MemberRollMutator {
	return &MemberRollMutator{
		namespaceFilter: namespaceFilter,
	}
}

var _ admission.Handler = (*MemberRollMutator)(nil)
var _ inject.Client = (*MemberRollMutator)(nil)
var _ inject.Decoder = (*MemberRollMutator)(nil)

func (v *MemberRollMutator) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	log := logf.Log.WithName("smmr-mutator").
		WithValues("ServiceMeshMemberRoll", webhookcommon.ToNamespacedName(req.AdmissionRequest))

	roll := &maistrav1.ServiceMeshMemberRoll{}
	err := v.decoder.Decode(req, roll)
	if err != nil {
		log.Error(err, "error decoding admission request")
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}

	// do we care about this object?
	if !v.namespaceFilter.Watching(roll.Namespace) {
		log.Info(fmt.Sprintf("operator is not watching namespace '%s'", roll.Namespace))
		return admission.ValidationResponse(true, "")
	} else if roll.ObjectMeta.DeletionTimestamp != nil {
		log.Info("skipping deleted smmr resource")
		return admission.ValidationResponse(true, "")
	}

	rollMutated := false

	// remove control plane namespace from members list
	filteredMembers := []string{}
	for _, ns := range roll.Spec.Members {
		if ns == roll.Namespace {
			log.Info("Removing control plane namespace from ServiceMeshMemberRoll")
			rollMutated = true
		} else {
			filteredMembers = append(filteredMembers, ns)
		}
	}

	if rollMutated {
		newRoll := roll.DeepCopy()
		newRoll.Spec.Members = filteredMembers
		return admission.PatchResponse(roll, newRoll)
	}
	return admission.ValidationResponse(true, "")
}

// InjectClient injects the client.
func (v *MemberRollMutator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *MemberRollMutator) InjectDecoder(d atypes.Decoder) error {
	v.decoder = d
	return nil
}
