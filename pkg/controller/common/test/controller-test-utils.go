package test

import (
	"context"
	"fmt"
	"testing"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/apis"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func GetScheme() *runtime.Scheme {
	s := scheme.Scheme
	if err := apis.AddToScheme(s); err != nil {
		panic(fmt.Sprintf("Could not add to scheme: %v", err))
	}
	if err := rbac.AddToScheme(s); err != nil {
		panic(fmt.Sprintf("Could not add to scheme: %v", err))
	}
	if err := apiextensionsv1beta1.AddToScheme(s); err != nil {
		panic(fmt.Sprintf("Could not add to scheme: %v", err))
	}
	s.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	}, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinitionList",
	}, &unstructured.UnstructuredList{})
	return s
}

func CreateClient(clientObjects ...runtime.Object) (client.Client, *EnhancedTracker) {
	s := GetScheme()
	codecs := serializer.NewCodecFactory(s)
	tracker := clienttesting.NewObjectTracker(s, codecs.UniversalDecoder())
	enhancedTracker := NewEnhancedTracker(tracker, s)
	cl := NewFakeClientWithSchemeAndTracker(s, enhancedTracker, clientObjects...)
	return cl, enhancedTracker
}

func GetObject(ctx context.Context, cl client.Client, objectKey client.ObjectKey, into runtime.Object) runtime.Object {
	err := cl.Get(ctx, objectKey, into)
	if err != nil {
		// we don't expect any errors, since we're calling Get on a fake client, but let's panic if one does occur
		panic(fmt.Sprintf("Unexpected error: %v", err))
	}
	return into
}

func GetUpdatedObject(ctx context.Context, cl client.Client, objectMeta meta.ObjectMeta, into runtime.Object) runtime.Object {
	err := cl.Get(ctx, getNamespacedName(objectMeta), into)
	if err != nil {
		// we don't expect any errors, since we're calling Get on a fake client, but let's panic if one does occur
		panic(fmt.Sprintf("Unexpected error: %v", err))
	}
	return into
}

func AssertObjectExists(ctx context.Context, cl client.Client, namespacedName types.NamespacedName, into runtime.Object, message string, t *testing.T) {
	err := cl.Get(ctx, namespacedName, into)
	if err != nil {
		if apierrors.IsNotFound(err) {
			t.Fatal(message)
		} else {
			// we don't expect any errors, since we're calling Get on a fake client, but let's panic if one does occur
			panic(fmt.Sprintf("Unexpected error: %v", err))
		}
	}
}

func AssertNotFound(ctx context.Context, cl client.Client, namespacedName types.NamespacedName, into runtime.Object, message string, t *testing.T) {
	err := cl.Get(ctx, namespacedName, into)
	if err == nil {
		t.Fatal(message)
	} else {
		if apierrors.IsNotFound(err) {
			// this is the expected outcome
		} else {
			// we don't expect any errors, since we're calling Get on a fake client, but let's panic if one does occur
			panic(fmt.Sprintf("Unexpected error: %v", err))
		}
	}
}

func AssertNumberOfWriteActions(t *testing.T, actions []clienttesting.Action, expected int) {
	count := 0
	for _, act := range actions {
		if isWriteAction(act) {
			count++
		}
	}
	assert.Equals(count, expected, "Unexpected number of write actions", t)
}

func isWriteAction(action clienttesting.Action) bool {
	return action.GetVerb() == "create" || action.GetVerb() == "update" || action.GetVerb() == "patch" ||
		action.GetVerb() == "delete" || action.GetVerb() == "delete-collection"
}

func AssertNumberOfActions(t *testing.T, actions []clienttesting.Action, expected int) {
	assert.Equals(len(actions), expected, "Unexpected number of client actions", t)
}

func AssertGetAction(t *testing.T, action clienttesting.Action, objectMeta meta.ObjectMeta, obj runtime.Object) {
	assert.Equals(action.GetVerb(), "get", "Unexpected client action verb", t)

	act := action.(clienttesting.GetAction)
	assert.Equals(act.GetName(), objectMeta.Name, "Unexpected resource name in client action", t)
	assert.Equals(act.GetNamespace(), objectMeta.Namespace, "Unexpected resource namespace in client action", t)
}

func getNamespacedName(obj meta.ObjectMeta) types.NamespacedName {
	return types.NamespacedName{Namespace: obj.Namespace, Name: obj.Name}
}

func ClientFailsOn(verb string, resource string) clienttesting.ReactionFunc {
	return func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.Matches(verb, resource) {
			return true, nil, fmt.Errorf("some error")
		}
		return false, nil, nil
	}
}

func ClientFails() clienttesting.ReactionFunc {
	return func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("some error")
	}
}

func On(verb string, resource string, reaction clienttesting.ReactionFunc) clienttesting.Reactor {
	return &clienttesting.SimpleReactor{
		Verb:     verb,
		Resource: resource,
		Reaction: reaction,
	}
}

func ClientReturnsNotFound(group, kind, name string) clienttesting.ReactionFunc {
	return func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{
			Group:    group,
			Resource: kind,
		}, name)
	}
}

func AttemptNumber(attempt int, reaction clienttesting.ReactionFunc) clienttesting.ReactionFunc {
	count := 0
	return func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		count++
		if count == attempt {
			return reaction(action)
		}
		return false, nil, nil
	}
}

func PanicOnError(err error) {
	if err != nil {
		panic(fmt.Sprintf("Unexpected error: %v", err))
	}
}
