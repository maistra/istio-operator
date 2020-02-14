package validation

import (
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgtypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	admissiontypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

type namespaceFilter string

var watchNamespace namespaceFilter

func init() {
	watchNamespaceStr, _ := k8sutil.GetWatchNamespace()
	watchNamespace = namespaceFilter(watchNamespaceStr)
}

func (f namespaceFilter) watching(namespace string) bool {
	return len(f) == 0 || namespace == string(f)
}

func validationFailedResponse(httpStatusCode int32, reason metav1.StatusReason, message string) admissiontypes.Response {
	response := admission.ValidationResponse(false, string(reason))
	if len(reason) == 0 {
		response.Response.Result = &metav1.Status{}
	}
	response.Response.Result.Code = httpStatusCode
	response.Response.Result.Message = message
	return response
}

func toNamespacedName(req *admissionv1beta1.AdmissionRequest) pkgtypes.NamespacedName {
	return pkgtypes.NamespacedName{req.Namespace, req.Name}
}
