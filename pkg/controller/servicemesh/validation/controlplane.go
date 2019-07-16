package validation

import (
	"context"
	"fmt"
	"net/http"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

type controlPlaneValidator struct {
	client  client.Client
	decoder atypes.Decoder
}

var _ admission.Handler = (*controlPlaneValidator)(nil)
var _ inject.Client = (*controlPlaneValidator)(nil)
var _ inject.Decoder = (*controlPlaneValidator)(nil)

func (v *controlPlaneValidator) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	logger := log.WithValues("Request.Namespace", req.AdmissionRequest.Namespace, "Request.Name", req.AdmissionRequest.Name).WithName("smcp-validator")
	smcp := &maistrav1.ServiceMeshControlPlane{}

	err := v.decoder.Decode(req, smcp)
	if err != nil {
		logger.Error(err, "error decoding admission request")
		return admission.ErrorResponse(http.StatusBadRequest, err)
	} else if smcp.ObjectMeta.DeletionTimestamp != nil {
		logger.Info("skipping deleted smcp resource")
		return admission.ValidationResponse(true, "")
	}

	// do we care about this object?
	if !watchNamespace.watching(smcp.Namespace) {
		logger.Info(fmt.Sprintf("operator is not watching namespace '%s'", smcp.Namespace))
		return admission.ValidationResponse(true, "")
	}

	smcpList := &maistrav1.ServiceMeshControlPlaneList{}
	err = v.client.List(ctx, nil, smcpList)
	if err != nil {
		logger.Error(err, "error listing smcp resources")
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}

	namespace := smcp.Namespace
	for _, othercp := range smcpList.Items {
		if othercp.Name == smcp.Name && othercp.Namespace == smcp.Namespace {
			continue
		}
		if othercp.Namespace == namespace {
			// verify single instance per namespace
			return admission.ErrorResponse(http.StatusBadRequest, fmt.Errorf("only one service mesh may be installed per project/namespace"))
		}
	}

	return admission.ValidationResponse(true, "")
}

// InjectClient injects the client.
func (v *controlPlaneValidator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *controlPlaneValidator) InjectDecoder(d atypes.Decoder) error {
	v.decoder = d
	return nil
}
