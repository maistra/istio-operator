package mutation

import (
	"context"
	"encoding/json"
	"fmt"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

var ctx = common.NewContextWithLog(context.Background(), logf.Log)

var acceptWithNoMutation = admission.Allowed("")

func newCreateRequest(obj interface{}) admission.Request {
	request := admission.Request{
		AdmissionRequest: admissionv1beta1.AdmissionRequest{
			Operation: admissionv1beta1.Create,
			Object:    toRawExtension(obj),
		},
	}
	return request
}

func newUpdateRequest(oldObj, newObj interface{}) admission.Request {
	request := admission.Request{
		AdmissionRequest: admissionv1beta1.AdmissionRequest{
			Operation: admissionv1beta1.Update,
			Object:    toRawExtension(newObj),
			OldObject: toRawExtension(oldObj),
		},
	}
	return request
}

func toRawExtension(obj interface{}) runtime.RawExtension {
	memberJSON, err := json.Marshal(obj)
	if err != nil {
		panic(fmt.Sprintf("Could not marshal object to JSON: %s", err))
	}

	return runtime.RawExtension{
		Raw: memberJSON,
	}
}

func now() *meta.Time {
	now := meta.Now()
	return &now
}
