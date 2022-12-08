package validation

import (
	"context"
	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	authentication "k8s.io/api/authentication/v1"
	authorization "k8s.io/api/authorization/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/maistra/istio-operator/controllers/common"
	"github.com/maistra/istio-operator/controllers/common/test"
)

var (
	ctx        = common.NewContextWithLog(context.Background(), logf.Log)
	testScheme = test.GetScheme()
)

var userInfo = authentication.UserInfo{
	Username: "joe-user",
	UID:      "some-UID",
	Groups:   []string{"some-group"},
	Extra: map[string]authentication.ExtraValue{
		"key": []string{"extra-value"},
	},
}

func createSubjectAccessReviewReactor(allowClusterScope, allowNamespaceScope bool,
	errorToReturn error,
) func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
	return func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(clienttesting.CreateAction)
		sar := createAction.GetObject().(*authorization.SubjectAccessReview)
		if sar.Spec.ResourceAttributes.Namespace == "" {
			sar.Status.Allowed = allowClusterScope
		} else {
			sar.Status.Allowed = allowNamespaceScope
		}
		return true, sar.DeepCopy(), errorToReturn
	}
}

func createCreateRequest(obj runtime.Object) admission.Request {
	request := createRequest(obj)
	request.Operation = admissionv1.Create
	request.UserInfo = userInfo
	return request
}

func createUpdateRequest(oldObj, newObj runtime.Object) admission.Request {
	request := createRequest(newObj)
	request.Operation = admissionv1.Update
	request.OldObject = toRawExtension(oldObj)
	request.UserInfo = userInfo
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
			UserInfo:  userInfo,
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
