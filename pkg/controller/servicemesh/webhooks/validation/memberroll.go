package validation

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

type MemberRollValidator struct {
	client          client.Client
	decoder         atypes.Decoder
	namespaceFilter webhookcommon.NamespaceFilter
}

func NewMemberRollValidator(namespaceFilter webhookcommon.NamespaceFilter) *MemberRollValidator {
	return &MemberRollValidator{
		namespaceFilter: namespaceFilter,
	}
}

var _ admission.Handler = (*MemberRollValidator)(nil)
var _ inject.Client = (*MemberRollValidator)(nil)
var _ inject.Decoder = (*MemberRollValidator)(nil)

func (v *MemberRollValidator) Handle(ctx context.Context, req atypes.Request) atypes.Response {
	logger := logf.Log.WithName("smmr-validator").
		WithValues("ServiceMeshMemberRoll", webhookcommon.ToNamespacedName(req.AdmissionRequest))
	smmr := &maistrav1.ServiceMeshMemberRoll{}

	err := v.decoder.Decode(req, smmr)
	if err != nil {
		logger.Error(err, "error decoding admission request")
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}

	// do we care about this object?
	if !v.namespaceFilter.Watching(smmr.Namespace) {
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
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("no service mesh is configured in namespace '%s'", smmr.Namespace))
	}

	// verify name == default
	if common.MemberRollName != smmr.Name {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("ServiceMeshMemberRoll must be named '%s'", common.MemberRollName))
	}

	smmrList := &maistrav1.ServiceMeshMemberRollList{}
	err = v.client.List(ctx, nil, smmrList)
	if err != nil {
		logger.Error(err, "error listing smmr resources")
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}

	// verify no duplicate members across all smmr resources
	namespacesAlreadyConfigured := sets.NewString()
	for _, othermr := range smmrList.Items {
		if othermr.Name == smmr.Name && othermr.Namespace == smmr.Namespace {
			continue
		}
		for _, member := range othermr.Spec.Members {
			namespacesAlreadyConfigured.Insert(member)
		}
	}

	for _, member := range smmr.Spec.Members {
		if namespacesAlreadyConfigured.Has(member) {
			return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "one or more members are already defined in another ServiceMeshMemberRoll")
		} else if smmr.Namespace == member {
			return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, "mesh project/namespace cannot be listed as a member")
		}
	}

	allowed, err := v.isUserAllowedToUpdatePods(common.NewContextWithLog(ctx, logger.WithValues("namespace", "<all>")), req, "")
	if err != nil {
		return admission.ErrorResponse(http.StatusInternalServerError, err)
	}
	if !allowed {
		// check each namespace separately, but only check newly added namespaces
		namespacesToCheck := sets.NewString(smmr.Spec.Members...)

		if req.AdmissionRequest.Operation == admissionv1.Update {
			oldSmmr := &maistrav1.ServiceMeshMemberRoll{}
			err := v.decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldSmmr)
			if err != nil {
				logger.Error(err, "error decoding old object in admission request")
				return admission.ErrorResponse(http.StatusBadRequest, err)
			}
			namespacesToCheck.Delete(oldSmmr.Spec.Members...)
		}

		for _, member := range namespacesToCheck.List() {
			allowed, err := v.isUserAllowedToUpdatePods(common.NewContextWithLog(ctx, logger.WithValues("namespace", member)), req, member)
			if err != nil {
				return admission.ErrorResponse(http.StatusInternalServerError, err)
			}
			if !allowed {
				return validationFailedResponse(http.StatusForbidden, metav1.StatusReasonBadRequest, fmt.Sprintf("user '%s' does not have permission to access project/namespace '%s'", req.AdmissionRequest.UserInfo.Username, member))
			}
		}
	}

	return admission.ValidationResponse(true, "")
}

func (v *MemberRollValidator) isUserAllowedToUpdatePods(ctx context.Context, req atypes.Request, member string) (bool, error) {
	log := common.LogFromContext(ctx)
	log.Info("Performing SAR check")
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   req.AdmissionRequest.UserInfo.Username,
			UID:    req.AdmissionRequest.UserInfo.UID,
			Extra:  convertUserInfoExtra(req.AdmissionRequest.UserInfo.Extra),
			Groups: req.AdmissionRequest.UserInfo.Groups,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:      "update",
				Group:     "",
				Resource:  "pods",
				Namespace: member,
			},
		},
	}
	err := v.client.Create(ctx, sar)
	if err != nil {
		log.Error(err, "error processing SubjectAccessReview")
		return false, err
	}
	return sar.Status.Allowed && !sar.Status.Denied, nil
}

func convertUserInfoExtra(extra map[string]authenticationv1.ExtraValue) map[string]authorizationv1.ExtraValue {
	converted := map[string]authorizationv1.ExtraValue{}
	for key, value := range extra {
		converted[key] = authorizationv1.ExtraValue(value)
	}
	return converted
}

// InjectClient injects the client.
func (v *MemberRollValidator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *MemberRollValidator) InjectDecoder(d atypes.Decoder) error {
	v.decoder = d
	return nil
}
