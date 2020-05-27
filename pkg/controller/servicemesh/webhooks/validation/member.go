package validation

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1beta1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/maistra/istio-operator/pkg/controller/common"
	webhookcommon "github.com/maistra/istio-operator/pkg/controller/servicemesh/webhooks/common"
)

type MemberValidator struct {
	client  client.Client
	decoder *admission.Decoder
}

func NewMemberValidator() *MemberValidator {
	return &MemberValidator{}
}

var _ admission.Handler = (*MemberValidator)(nil)
var _ inject.Client = (*MemberValidator)(nil)
var _ admission.DecoderInjector = (*MemberValidator)(nil)

func (v *MemberValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := logf.Log.WithName("smm-validator").
		WithValues("ServiceMeshMember", webhookcommon.ToNamespacedName(&req.AdmissionRequest))
	smm := &maistrav1.ServiceMeshMember{}

	err := v.decoder.Decode(req, smm)
	if err != nil {
		logger.Error(err, "error decoding admission request")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// do we care about this object?
	if smm.ObjectMeta.DeletionTimestamp != nil {
		logger.Info("skipping deleted smm resource")
		return admission.Allowed("")
	}

	// verify name == default
	if common.MemberName != smm.Name {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("ServiceMeshMember must be named %q", common.MemberName))
	}

	if smm.Namespace == common.GetOperatorNamespace() {
		return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, fmt.Sprintf("namespace where operator is installed cannot be added to any mesh"))
	}

	if req.AdmissionRequest.Operation == admissionv1.Update {
		oldSmm := &maistrav1.ServiceMeshMember{}
		err := v.decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldSmm)
		if err != nil {
			logger.Error(err, "error decoding old object in admission request")
			return admission.Errored(http.StatusBadRequest, err)
		}

		if smm.Spec.ControlPlaneRef.Name != oldSmm.Spec.ControlPlaneRef.Name ||
			smm.Spec.ControlPlaneRef.Namespace != oldSmm.Spec.ControlPlaneRef.Namespace {
			logger.Info("Client tried to mutate ServiceMeshMember.spec.controlPlaneRef")
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("Mutation of .spec.controlPlaneRef isn't allowed"))
		}
	}

	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   req.AdmissionRequest.UserInfo.Username,
			UID:    req.AdmissionRequest.UserInfo.UID,
			Extra:  convertUserInfoExtra(req.AdmissionRequest.UserInfo.Extra),
			Groups: req.AdmissionRequest.UserInfo.Groups,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:      "use",
				Group:     "maistra.io",
				Resource:  "servicemeshcontrolplanes",
				Name:      smm.Spec.ControlPlaneRef.Name,
				Namespace: smm.Spec.ControlPlaneRef.Namespace,
			},
		},
	}

	err = v.client.Create(ctx, sar)
	if err != nil {
		logger.Error(err, "error processing SubjectAccessReview")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if !sar.Status.Allowed || sar.Status.Denied {
		return admission.Errored(http.StatusForbidden, fmt.Errorf("user '%s' does not have permission to use ServiceMeshControlPlane %s/%s", req.AdmissionRequest.UserInfo.Username, smm.Spec.ControlPlaneRef.Namespace, smm.Spec.ControlPlaneRef.Name))
	}

	return admission.Allowed("")
}

// InjectClient injects the client.
func (v *MemberValidator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (v *MemberValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
