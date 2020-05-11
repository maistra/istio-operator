package test

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"

	"github.com/maistra/istio-operator/pkg/apis"
)

// RunControllerTestCase executes each test case using a new manager.Manager
func RunControllerTestCase(t *testing.T, testCase ControllerTestCase) {
	t.Helper()
	utilruntime.ErrorHandlers = append(utilruntime.ErrorHandlers, func(err error) { t.Errorf("unhandled error occurred in k8s: %v", err) })
	defer func() {
		// XXX: this is pretty sketchy...
		utilruntime.ErrorHandlers = utilruntime.ErrorHandlers[:len(utilruntime.ErrorHandlers)-1]
	}()
	if testCase.ConfigureGlobals != nil {
		testCase.ConfigureGlobals()
	}
	mgr, tracker, err := NewManagerForControllerTest(testCase.GroupResources...)
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range testCase.Resources {
		// XXX: should we use client.Create() or tracker.Add()?
		// client.Create() has side effects, like adding creation time, resource version, etc.
		if err := mgr.GetClient().Create(context.TODO(), resource); err != nil {
			t.Fatal(err)
		}
	}
	for _, addController := range testCase.AddControllers {
		if err := addController(mgr); err != nil {
			t.Fatal(err)
		}
	}
	stop := StartManager(mgr, t)
	func() {
		t.Helper()
		defer stop()
		for _, event := range testCase.Events {
			t.Run(event.Name, func(t *testing.T) {
				t.Helper()
				extraneousActionFilter := &extraneousActionFailure{verifier: event.Verifier, t: t}
				defer func() {
					if event.AssertExtraneousActions {
						tracker.RemoveReaction(extraneousActionFilter)
					}
					tracker.RemoveReaction(event.Verifier)
					for _, assertion := range event.Assertions {
						tracker.RemoveReaction(assertion)
					}
					tracker.RemoveReaction(event.Reactors...)
				}()
				// inject the test runner into the verifier
				event.Verifier.InjectTestRunner(t)
				// insert reactions.  these must come before any default reactions added for normal resource handling
				tracker.PrependReaction(event.Reactors...)
				// insert assertions.  these need to be before any reactors, as they do not actually handle events
				for _, assertion := range event.Assertions {
					tracker.PrependReaction(assertion)
				}
				// insert verifier.  this needs to be the first handler, as it verifies the event, but does not handle it
				tracker.PrependReaction(event.Verifier)
				if event.AssertExtraneousActions {
					// add failure for events occurring after validation should be complete
					tracker.PrependReaction(extraneousActionFilter)
				}
				if err := event.Execute(mgr, tracker); err != nil {
					t.Fatal(err)
				}
				startTime := time.Now()
				if !event.Verifier.Wait(event.Timeout) {
					// no need to process assertions if there was a problem with the event processing
					// just need to wait for Reconcile() to complete before processing assertions
					if !mgr.WaitForReconcileCompletion(event.Timeout - time.Since(startTime)) {
						for _, assertion := range event.Assertions {
							assertion.Assert(t)
						}
					} else {
						t.Fatal("timed out waiting for empty reconciliation queue")
					}
				}
			})
		}
	}()
}

// NewManagerForControllerTest creates a new FakeManager that can be used for running controller tests.
// The returned EnhancedTracker is the same tracker used within the FakeManager and can be used for
// manipulating resources without going through the manager itself.
func NewManagerForControllerTest(groupResources ...*restmapper.APIGroupResources) (*FakeManager, *EnhancedTracker, error) {
	s := runtime.NewScheme()
	err := apis.AddToScheme(s)
	if err != nil {
		return nil, nil, err
	}
	if err := appsv1.AddToScheme(s); err != nil {
		return nil, nil, err
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
	codecs := serializer.NewCodecFactory(s)
	tracker := NewEnhancedTracker(clienttesting.NewObjectTracker(s, codecs.UniversalDecoder()), s)
	mgr, err := NewManager(s, tracker, groupResources...)
	return mgr, tracker, err
}

type extraneousActionFailure struct {
	verifier ActionVerifier
	t        *testing.T
}

var _ clienttesting.Reactor = (*extraneousActionFailure)(nil)

func (r *extraneousActionFailure) Handles(action clienttesting.Action) bool {
	return r.verifier.HasFired()
}
func (r *extraneousActionFailure) React(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
	r.t.Fatalf("unexpected action ocurred: %#v", action)
	return true, nil, errors.NewServiceUnavailable("test processing complete")
}
