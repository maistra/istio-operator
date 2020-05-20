package mutation

import (
	"encoding/json"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PatchResponse is a bridge for the webhooks using the old controller-runtime
// v0.1.x admission.PatchResponse() function.
func PatchResponse(original runtime.RawExtension, new runtime.Object) admission.Response {
	marshaledObject, err := json.Marshal(new)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(original.Raw, marshaledObject)
}