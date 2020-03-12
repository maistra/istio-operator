package test

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/recorder"
)

// FakeManager is a manager that can be used for testing
type FakeManager struct {
	manager.Manager
	recorderProvider recorder.Provider
	reconcileWG sync.WaitGroup
}

var _ manager.Manager = (*FakeManager)(nil)

// StartManager starts the manager and returns a function that can be used to
// stop the manager.
func StartManager(mgr manager.Manager, t *testing.T) func() {
	t.Helper()
	stopChannel := make(chan struct{})
	startChannel := make(chan struct{})
	go func() {
		t.Helper()
		if err := mgr.Start(stopChannel); err != nil {
			t.Fatalf("Uexpected error returned from manager.Manager: %v", err)
		}
		t.Logf("manager stopped")
		close(startChannel)
	}()
	return func() {
		close(stopChannel)
		select {
		case <-startChannel:
		}
	}
}

// NewManager returns a new FakeManager that can be used for testing.
func NewManager(scheme *runtime.Scheme, tracker clienttesting.ObjectTracker, groupResources ...*restmapper.APIGroupResources) (*FakeManager, error) {
	options := NewManagerOptions(scheme, tracker, groupResources...)
	delegate, err := manager.New(&rest.Config{}, options)
	if err != nil {
		return nil, err
	}
	return &FakeManager{
		Manager:          delegate,
		recorderProvider: NewRecorderProvider(scheme),
		reconcileWG: sync.WaitGroup{},
	}, nil
}

// GetRecorder returns a new EventRecorder for the provided name
func (m *FakeManager) GetRecorder(name string) record.EventRecorder {
	return m.recorderProvider.GetEventRecorderFor(name)
}

// Add intercepts the Add() call by wrapping the Reconciler for any Controller
// with code that works with a WaitGroup. This allows reconciles to be tracked
// by the Manager.  Tests should use FakeManager.WaitForReconcileCompletion()
// to wait for all active Reconcile() invocations to fully complete before
// asserting results.
func (m *FakeManager) Add(runnable manager.Runnable) error {
	if controller, ok := runnable.(controller.Controller); ok {
		value := reflect.ValueOf(controller)
		doValue := value.Elem().FieldByName("Do")
		if reconciler, ok := doValue.Elem().Interface().(reconcile.Reconciler); ok {
			reconciler = &trackingReconciler{Reconciler: reconciler, wg: &m.reconcileWG}
			doValue.Set(reflect.ValueOf(reconciler))
		}
	}
	return m.Manager.Add(runnable)
}

// WaitForReconcileCompletion waits for all active reconciliations to complete.
// This includes reconcilations that may have started after this function was
// called, but prior to other active reconcilations completing. For example,
// when testing multiple controllers, the actions of one controller may trigger
// a reconcilation by another.  This function will wait until the controllers
// have settled to an idle state.  There is a danger that controllers could
// bounce events back and forth, causing this call to effectively hang.
func (m *FakeManager) WaitForReconcileCompletion() {
	m.reconcileWG.Wait()
}

type trackingReconciler struct {
	reconcile.Reconciler
	wg *sync.WaitGroup
}

func (r *trackingReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// XXX: should we introduce a condition variable that prevents entering a
	// a new reconcile once a caller has begun waiting for completion?
	r.wg.Add(1)
	defer r.wg.Done()
	return r.Reconciler.Reconcile(request)
}

// NewManagerOptions returns a set of options that create a "normal" manager for
// use with unit tests.  Typically, tests should use a FakeManager, which
// provides additional capabilities.
func NewManagerOptions(scheme *runtime.Scheme, tracker clienttesting.ObjectTracker, groupResources ...*restmapper.APIGroupResources) manager.Options {
	var enhancedTracker *EnhancedTracker
	var ok bool
	if enhancedTracker, ok = tracker.(*EnhancedTracker); !ok {
		enhancedTracker = NewEnhancedTracker(tracker, scheme)
	}
	enhancedTracker.AddReactor("create", "*", NewCreateSimulator(enhancedTracker))
	enhancedTracker.AddReactor("update", "*", NewUpdateSimulator(enhancedTracker))
	enhancedTracker.AddReactor("delete", "*", NewDeleteSimulator(enhancedTracker))
	return manager.Options{
		Scheme:             scheme,
		MapperProvider:     newMapper(groupResources...),
		LeaderElection:     false,
		NewCache:           newCacheFunc(enhancedTracker),
		NewClient:          newClientFunc(scheme, enhancedTracker),
		MetricsBindAddress: "0", // disable metrics listener
	}
}

func newMapper(groupResources ...*restmapper.APIGroupResources) func(c *rest.Config) (meta.RESTMapper, error) {
	return func(_ *rest.Config) (meta.RESTMapper, error) {
		return restmapper.NewDiscoveryRESTMapper(groupResources), nil
	}
}

func newClientFunc(clientScheme *runtime.Scheme, tracker clienttesting.ObjectTracker) manager.NewClientFunc {
	return func(_ cache.Cache, _ *rest.Config, _ client.Options) (client.Client, error) {
		return NewFakeClientWithSchemeAndTracker(clientScheme, tracker), nil
	}
}

func newCacheFunc(tracker clienttesting.ObjectTracker) manager.NewCacheFunc {
	return func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
		return NewCache(opts, tracker)
	}
}

// ReconcilerInvocation encapsulates the request and response that were sent through a reconcile.Reconciler
type ReconcilerInvocation struct {
	reconcile.Request
	reconcile.Result
	Error error
}

// WrapReconciler wraps the reconciler with a channel that is notified of the request/response of the reconcile.Reconciler.
// This may be useful for synchronizing test cases around a controller (i.e. know when an event has been processed.). Using
// the controller manager test utilities (e.g. ActionVerifier) allows for more advanced scripting, e.g. when a single event
// may cause multiple passes through the reconcile.Reconciler.
func WrapReconciler(reconciler reconcile.Reconciler, resultChannel chan ReconcilerInvocation) reconcile.Reconciler {
	return reconcile.Func(func(request reconcile.Request) (reconcile.Result, error) {
		invocation := ReconcilerInvocation{Request: request}
		invocation.Result, invocation.Error = reconciler.Reconcile(request)
		resultChannel <- invocation
		return invocation.Result, invocation.Error
	})
}

// NewCreateSimulator returns a ReactionFunc that simulates the work done by the api
// server to initialize a newly created object.
// TODO: see if there is some defaulting mechanism that we can use instead of this custom code.
func NewCreateSimulator(tracker clienttesting.ObjectTracker) clienttesting.ReactionFunc {
	return clienttesting.ReactionFunc(func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction, ok := action.(clienttesting.CreateAction)
		if !ok {
			return true, nil, fmt.Errorf("CreateSimulator can only be used with 'create' actions")
		}
		obj := createAction.GetObject()
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return true, nil, err
		}
		// XXX: initialize common fields
		accessor.SetGeneration(1)
		accessor.SetResourceVersion(fmt.Sprintf("%d", rand.Int()))
		if len(accessor.GetSelfLink()) == 0 {
			typeObj, err := meta.TypeAccessor(obj)
			if err != nil {
				return true, nil, err
			}
			accessor.SetSelfLink(fmt.Sprintf("dummy/%s/%s/%s/%s", createAction.GetResource().GroupResource(), typeObj.GetKind(), accessor.GetNamespace(), accessor.GetName()))
			accessor.SetCreationTimestamp(metav1.Now())
		}
		err = tracker.Create(createAction.GetResource(), obj, accessor.GetNamespace())
		return true, obj, err
	})
}

// NewDeleteSimulator returns a ReactionFunc that simulates delete processing in the api server.
// For example, if the object contains a non-empty finalizer list, the request is treated
// as an update, which adds a deletion timestamp to the resource.
func NewDeleteSimulator(tracker clienttesting.ObjectTracker) clienttesting.ReactionFunc {
	return clienttesting.ReactionFunc(func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		deleteAction, ok := action.(clienttesting.DeleteAction)
		if !ok {
			return true, nil, fmt.Errorf("DeleteSimulator can only be used with 'delete' actions")
		}
		obj, err := tracker.Get(deleteAction.GetResource(), deleteAction.GetNamespace(), deleteAction.GetName())
		if err != nil {
			return true, nil, err
		}
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return true, nil, err
		}
		// XXX: correct deletion processing
		if len(accessor.GetFinalizers()) > 0 {
			if accessor.GetDeletionTimestamp() == nil {
				now := metav1.Now()
				accessor.SetDeletionTimestamp(&now)
				err = tracker.Update(deleteAction.GetResource(), obj, deleteAction.GetNamespace())
			}
			// not sure if this is correct, but the object is already marked for deletion, but still has finalizers registered
			return true, obj, err
		}
		err = tracker.Delete(deleteAction.GetResource(), accessor.GetNamespace(), accessor.GetName())
		return true, obj, err
	})
}

// NewUpdateSimulator creates a ReactionFunc that simulates the update processing in the api
// server, e.g. incrementing the generation field.
func NewUpdateSimulator(tracker clienttesting.ObjectTracker) clienttesting.ReactionFunc {
	return clienttesting.ReactionFunc(func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		updateAction, ok := action.(clienttesting.UpdateAction)
		if !ok {
			return true, nil, fmt.Errorf("UpdateSimulator can only be used with 'update' actions")
		}
		obj := updateAction.GetObject()
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return true, nil, err
		}
		// XXX: update fields that would get modified by the api server on an update
		accessor.SetResourceVersion(fmt.Sprintf("%d", rand.Int()))
		if accessor.GetDeletionTimestamp() == nil && len(updateAction.GetSubresource()) == 0 {
			// XXX: this should only be done if the .spec field actually changes.
			// i don't think we should do this if finalizers, annotations, labels, etc. are updated
			accessor.SetGeneration(accessor.GetGeneration() + 1)
		}
		err = tracker.Update(updateAction.GetResource(), obj, accessor.GetNamespace())
		return true, obj, err
	})
}
