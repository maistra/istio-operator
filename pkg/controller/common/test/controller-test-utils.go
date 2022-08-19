package test

import (
	"context"
	"fmt"
	"testing"

	arv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	clienttesting "k8s.io/client-go/testing"
	"maistra.io/api/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/apis"
	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

func GetScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := apis.AddToScheme(s); err != nil {
		panic(fmt.Sprintf("Could not add to scheme: %v", err))
	}
	if err := arv1beta1.AddToScheme(s); err != nil {
		panic(fmt.Sprintf("Could not add to scheme: %v", err))
	}
	if err := s.SetVersionPriority(v2.SchemeGroupVersion, v1.SchemeGroupVersion, v1alpha1.SchemeGroupVersion); err != nil {
		panic(err)
	}
	return s
}

func CreateClient(clientObjects ...runtime.Object) (client.Client, *EnhancedTracker) {
	return CreateClientWithScheme(GetScheme(), clientObjects...)
}

func CreateClientWithScheme(s *runtime.Scheme, clientObjects ...runtime.Object) (client.Client, *EnhancedTracker) {
	codecs := serializer.NewCodecFactory(s)
	tracker := clienttesting.NewObjectTracker(s, codecs.UniversalDecoder())
	enhancedTracker := NewEnhancedTracker(tracker, s, v2.SchemeGroupVersion)
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
	err := cl.Get(ctx, common.ToNamespacedName(&objectMeta), into)
	if err != nil {
		// we don't expect any errors, since we're calling Get on a fake client, but let's panic if one does occur
		panic(fmt.Sprintf("Unexpected error: %v", err))
	}
	return into
}

func AssertObjectExists(ctx context.Context, cl client.Client, namespacedName types.NamespacedName, into runtime.Object, message string, t *testing.T) {
	t.Helper()
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
	t.Helper()
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
	t.Helper()
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
	t.Helper()
	assert.Equals(len(actions), expected, "Unexpected number of client actions", t)
}

func AssertGetAction(t *testing.T, action clienttesting.Action, objectMeta meta.ObjectMeta, obj runtime.Object) {
	t.Helper()
	assert.Equals(action.GetVerb(), "get", "Unexpected client action verb", t)

	act := action.(clienttesting.GetAction)
	assert.Equals(act.GetName(), objectMeta.Name, "Unexpected resource name in client action", t)
	assert.Equals(act.GetNamespace(), objectMeta.Namespace, "Unexpected resource namespace in client action", t)
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
		return true, nil, fmt.Errorf("error on %s %v", action.GetVerb(), action.GetResource())
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
