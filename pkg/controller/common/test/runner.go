package test

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"

	"github.com/maistra/istio-operator/pkg/controller/common"
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
	mgr, tracker, err := NewManagerForControllerTest(testCase.StorageVersions, testCase.GroupResources...)
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

	dc := fake.FakeDiscovery{Fake: &tracker.Fake, FakedServerVersion: DefaultKubeVersion}
	enhancedMgr := common.NewEnhancedManager(mgr, &dc)
	for _, addController := range testCase.AddControllers {
		if err := addController(enhancedMgr); err != nil {
			t.Fatal(err)
		}
	}
	stop := StartManager(mgr, t)
	func() {
		t.Helper()
		defer stop()
		if len(testCase.Events) == 0 {
			t.Errorf("no events specified for test")
			return
		}
		failedTest := false
		for _, event := range testCase.Events {
			failedTest = !t.Run(event.Name, func(t *testing.T) {
				if failedTest {
					t.Skipf("skipping event because of previous test failure")
					return
				}
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
				if event.Verifier != nil {
					event.Verifier.InjectTestRunner(t)
				}
				// insert reactions.  these must come before any default reactions added for normal resource handling
				tracker.PrependReaction(event.Reactors...)
				// insert assertions.  these need to be before any reactors, as they do not actually handle events
				for _, assertion := range event.Assertions {
					tracker.PrependReaction(assertion)
				}
				// insert verifier.  this needs to be the first handler, as it verifies the event, but does not handle it
				if event.Verifier != nil {
					tracker.PrependReaction(event.Verifier)
				}
				if event.AssertExtraneousActions {
					// add failure for events occurring after validation should be complete
					tracker.PrependReaction(extraneousActionFilter)
				}
				if err := event.Execute(mgr, tracker); err != nil {
					t.Error(err)
				} else {
					// wait for the first event to show up on the queue
					mgr.WaitForFirstEvent()
					if event.Verifier == nil || !event.Verifier.Wait(event.Timeout) {
						// no need to process assertions if there was a problem with the event processing
						// just need to wait for Reconcile() to complete before processing assertions
						mgr.WaitForReconcileCompletion()
						for _, assertion := range event.Assertions {
							assertion.Assert(t)
						}
					}
				}
			}) || failedTest
		}
	}()
}

// NewManagerForControllerTest creates a new FakeManager that can be used for running controller tests.
// The returned EnhancedTracker is the same tracker used within the FakeManager and can be used for
// manipulating resources without going through the manager itself.
func NewManagerForControllerTest(storageVersions []schema.GroupVersion,
	groupResources ...*restmapper.APIGroupResources,
) (*FakeManager, *EnhancedTracker, error) {
	s := GetScheme()
	codecs := serializer.NewCodecFactory(s)
	tracker := NewEnhancedTracker(clienttesting.NewObjectTracker(s, codecs.UniversalDecoder()), s, storageVersions...)
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
	r.t.Fatalf("unexpected action occurred: %#v", action)
	return true, nil, errors.NewServiceUnavailable("test processing complete")
}
