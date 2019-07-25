package validation

import (
	"context"
	"fmt"
	"net/http"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

	authorizationv1 "k8s.io/api/authorization/v1"
	authenticationv1 "k8s.io/api/authentication/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

const memberRollName = "default"

type memberRollValidator struct {
	client  client.Client
	decoder atypes.Decoder
}

var _ admission.Handler = (*memberRollValidator)(nil)
var _ inject.Client = (*memberRollValidator)(nil)
var _ inject.Decoder = (*memberRollValidator)(nil)

func (v *memberRollValidator) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	logger := log.WithValues("Request.Namespace", req.AdmissionRequest.Namespace, "Request.Name", req.AdmissionRequest.Name).WithName("smmr-validator")
	smmr := &maistrav1.ServiceMeshMemberRoll{}

	err := v.decoder.Decode(req, smmr)
	if err != nil {
		logger.Error(err, "error decoding admission request")
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}

	// do we care about this object?
	if !watchNamespace.watching(smmr.Namespace) {
		logger.Info(fmt.Sprintf("operator is not watching namespace '%s'", smmr.Namespace))
		return admission.ValidationResponse(true, "")
	} else if smmr.ObjectMeta.DeletionTimestamp != nil {
		logger.Info("skipping deleted smmr resource")
		return admission.ValidationResponse(true, "")
	}

	// is this mesh configured for multitenancy?
	smcpList := &maistrav1.ServiceMeshControlPlaneList{}
	err = v.client.List(ctx, client.InNamespace(smmr.Namespace), smcpList)
	if err != nil {
		logger.Error(err, "error listing smcp resources")
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}
	if len(smcpList.Items) == 0 {
		return admission.ErrorResponse(http.StatusBadRequest, fmt.Errorf("no service mesh is configured in namespace '%s'", smmr.Namespace))
	}

	// verify name == default
	if memberRollName != smmr.Name {
		return admission.ErrorResponse(http.StatusBadRequest, fmt.Errorf("ServiceMeshMemberRoll must be named '%s'", memberRollName))
	}

	smmrList := &maistrav1.ServiceMeshMemberRollList{}
	err = v.client.List(ctx, nil, smmrList)
	if err != nil {
		logger.Error(err, "error listing smmr resources")
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}

	// verify no duplicate members across all smmr resources
	namespacesAlreadyConfigured := map[string]struct{}{}
	for _, othermr := range smmrList.Items {
		if othermr.Name == smmr.Name && othermr.Namespace == smmr.Namespace {
			continue
		}
		for _, member := range othermr.Spec.Members {
			namespacesAlreadyConfigured[member] = struct{}{}
		}
	}
	sar := &authorizationv1.SubjectAccessReview{}
	sar.Spec.User = req.AdmissionRequest.UserInfo.Username
	sar.Spec.UID = req.AdmissionRequest.UserInfo.UID
	sar.Spec.Extra = convertUserInfoExtra(req.AdmissionRequest.UserInfo.Extra)
	sar.Spec.Groups = make([]string, len(req.AdmissionRequest.UserInfo.Groups))
	copy(sar.Spec.Groups, req.AdmissionRequest.UserInfo.Groups)
	sar.Spec.ResourceAttributes = &authorizationv1.ResourceAttributes{
		Verb:     "update",
		Group:    "",
		Resource: "pods",
	}
	for _, member := range smmr.Spec.Members {
		if _, ok := namespacesAlreadyConfigured[member]; ok {
			return admission.ErrorResponse(http.StatusBadRequest, fmt.Errorf("one or more members are already defined in another ServiceMeshMemberRoll"))
		} else if smmr.Namespace == member {
			return admission.ErrorResponse(http.StatusBadRequest, fmt.Errorf("mesh project/namespace cannot be listed as a member"))
		}
		// verify user can access all smmr member namespaces
		sar.Spec.ResourceAttributes.Namespace = member
		sar.Status = authorizationv1.SubjectAccessReviewStatus{}
		err = v.client.Create(ctx, sar)
		if err != nil {
			logger.Error(err, "error processing SubjectAccessReview")
			return admission.ErrorResponse(http.StatusInternalServerError, err)
		}
		if !sar.Status.Allowed || sar.Status.Denied {
			return admission.ErrorResponse(http.StatusForbidden, fmt.Errorf("user '%s' does not have permission to access project/namespace '%s'", req.AdmissionRequest.UserInfo.Username, member))
		}
	}

	return admission.ValidationResponse(true, "")
}

func convertUserInfoExtra(extra map[string]authenticationv1.ExtraValue) map[string]authorizationv1.ExtraValue {
	converted := map[string]authorizationv1.ExtraValue{}
	for key, value := range extra {
		converted[key] = authorizationv1.ExtraValue(value)
	}
	return converted
}

// InjectClient injects the client.
func (v *memberRollValidator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *memberRollValidator) InjectDecoder(d atypes.Decoder) error {
	v.decoder = d
	return nil
}
