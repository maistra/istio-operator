package validation

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	admissiontypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

func validationFailedResponse(httpStatusCode int32, reason metav1.StatusReason, message string) admissiontypes.Response {
	response := admission.ValidationResponse(false, string(reason))
	if len(reason) == 0 {
		response.Response.Result = &metav1.Status{}
	}
	response.Response.Result.Code = httpStatusCode
	response.Response.Result.Message = message
	return response
}
