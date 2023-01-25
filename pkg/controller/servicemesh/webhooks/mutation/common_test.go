package mutation

import (
	"context"
	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test"
)

var (
	ctx        = common.NewContextWithLog(context.Background(), logf.Log)
	testScheme = test.GetScheme()
)

var acceptWithNoMutation = admission.Allowed("")

func newCreateRequest(obj runtime.Object) admission.Request {
	request := createRequest(obj)
	request.Operation = admissionv1.Create
	return request
}

func newUpdateRequest(oldObj, newObj runtime.Object) admission.Request {
	request := createRequest(newObj)
	request.Operation = admissionv1.Update
	request.OldObject = toRawExtension(oldObj)
	return request
}

func createRequest(obj runtime.Object) admission.Request {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		panic(err)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Kind:      metaGVKForObject(obj),
			Name:      metaObj.GetName(),
			Namespace: metaObj.GetNamespace(),
			Object:    toRawExtension(obj),
		},
	}
}

func metaGVKForObject(obj runtime.Object) metav1.GroupVersionKind {
	gvks, _, err := testScheme.ObjectKinds(obj)
	if err != nil {
		panic(err)
	} else if len(gvks) == 0 {
		panic(fmt.Errorf("could not get GVK for object: %T", obj))
	}
	return metav1.GroupVersionKind{Group: gvks[0].Group, Kind: gvks[0].Kind, Version: gvks[0].Version}
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

func now() *metav1.Time {
	now := metav1.Now()
	return &now
}
