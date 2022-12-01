package mutation

import (
	"encoding/json"
	"net/http"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PatchResponse is a bridge for the webhooks using the old controller-runtime
// v0.1.x admission.PatchResponse() function.
func PatchResponse(original runtime.RawExtension, newObj runtime.Object) admission.Response {
	marshaledObject, err := json.Marshal(newObj)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	resp := admission.PatchResponseFromRaw(original.Raw, marshaledObject)
	if resp.Allowed && resp.Result == nil {
		resp.Result = &v1.Status{
			Code: int32(http.StatusOK),
		}
	}
	resp.PatchType = nil
	return resp
}
