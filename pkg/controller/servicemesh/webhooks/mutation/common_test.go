package mutation

import (
	"context"
	"encoding/json"
	"fmt"

	admission "k8s.io/api/admission/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	webhookadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	atypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

var ctx = common.NewContextWithLog(context.Background(), logf.Log)

var acceptWithNoMutation = webhookadmission.ValidationResponse(true, "")

func newCreateRequest(obj interface{}) atypes.Request {
	request := atypes.Request{
		AdmissionRequest: &admission.AdmissionRequest{
			Operation: admission.Create,
			Object:    toRawExtension(obj),
		},
	}
	return request
}

func newUpdateRequest(oldObj, newObj interface{}) atypes.Request {
	request := atypes.Request{
		AdmissionRequest: &admission.AdmissionRequest{
			Operation: admission.Update,
			Object:    toRawExtension(newObj),
			OldObject: toRawExtension(oldObj),
		},
	}
	return request
}

func toRawExtension(obj interface{}) runtime.RawExtension {
	memberJson, err := json.Marshal(obj)
	if err != nil {
		panic(fmt.Sprintf("Could not marshal object to JSON: %s", err))
	}

	return runtime.RawExtension{
		Raw: memberJson,
	}
}

func now() *meta.Time {
	now := meta.Now()
	return &now
}
