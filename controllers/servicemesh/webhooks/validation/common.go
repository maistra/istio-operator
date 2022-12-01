package validation

import (
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func validationFailedResponse(httpStatusCode int32, reason metav1.StatusReason, message string) admission.Response {
	response := admission.Denied(string(reason))
	if len(reason) == 0 {
		response.Result = &metav1.Status{}
	}
	response.Result.Code = httpStatusCode
	response.Result.Message = message
	return response
}

func badRequest(message string) admission.Response {
	return validationFailedResponse(http.StatusBadRequest, metav1.StatusReasonBadRequest, message)
}

func forbidden(message string) admission.Response {
	return validationFailedResponse(http.StatusForbidden, metav1.StatusReasonForbidden, message)
}
