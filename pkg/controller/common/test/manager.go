package test

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
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
	ObjectTracker    *EnhancedTracker
	recorderProvider recorder.Provider
	requestTracker   requestTracker
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
		t.Logf("starting manager.Manager")
		if err := mgr.Start(stopChannel); err != nil {
			t.Fatalf("Uexpected error returned from manager.Manager: %v", err)
		}
		t.Logf("manager.Manager stopped cleanly")
		close(startChannel)
	}()

	mgr.GetCache().WaitForCacheSync(stopChannel)
	t.Logf("manager Cache synchronized")
	return func() {
		close(stopChannel)
		select {
		case <-startChannel:
		}
	}
}

// NewManager returns a new FakeManager that can be used for testing.
func NewManager(scheme *runtime.Scheme, tracker *EnhancedTracker, groupResources ...*restmapper.APIGroupResources) (*FakeManager, error) {
	options := NewManagerOptions(scheme, tracker, groupResources...)
	delegate, err := manager.New(&rest.Config{}, options)
	if err != nil {
		return nil, err
	}
	return &FakeManager{
		Manager:          delegate,
		ObjectTracker:    tracker,
		recorderProvider: NewRecorderProvider(scheme),
		requestTracker:   requestTracker{cond: sync.NewCond(&sync.Mutex{})},
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
		queueValue := value.Elem().FieldByName("Queue")
		if !queueValue.IsValid() {
			panic(fmt.Errorf("cannot hook controller.Queue"))
		}
		if q, ok := queueValue.Elem().Interface().(workqueue.RateLimitingInterface); ok {
			tq := newTrackingQueue(q, &m.requestTracker)
			queueValue.Set(reflect.ValueOf(tq))
		}
		controllerName := value.Elem().FieldByName("Name").String()
		doValue := value.Elem().FieldByName("Do")
		if !doValue.IsValid() {
			panic(fmt.Errorf("cannot hook controller.Do"))
		}
		if r, ok := doValue.Elem().Interface().(reconcile.Reconciler); ok {
			r = &reconcilerWrapper{
				Reconciler:     r,
				manager:        m,
				controllerName: controllerName,
			}
			doValue.Set(reflect.ValueOf(r))
		}
	}
	return m.Manager.Add(runnable)
}

// ReconcileRequestAction is a Fake action that is sent before a request is sent
// to a Reconciler.  This can be used to assert the generation of a request and
// a Reactor can be used to prevent the request from being sent to the Reconciler
// by handling the action and returning an error.
type ReconcileRequestAction interface {
	clienttesting.Action
	// GetName returns the controller name
	GetName() string
	// GetReconcileRequest returns the reconcile.Request
	GetReconcileRequest() reconcile.Request
}

// ReconcileResultAction is a Fake action that is sent when a result is received
// from a Reconciler.  This can be used to assert the result of a reconcile and
// a Reactor can be used to modify the error returned to the controller.
type ReconcileResultAction interface {
	ReconcileRequestAction
	// GetReconcileResult returns the result of the reconcile
	GetReconcileResult() (reconcile.Result, error)
}

// NewReconcileRequestAction returns a new ReconcileRequestAction.
func NewReconcileRequestAction(controllerName string, req reconcile.Request) ReconcileRequestAction {
	return &reconcileRequestAction{
		ActionImpl: clienttesting.ActionImpl{
			Resource: schema.GroupVersionResource{
				Group:    "testing.reconciler",
				Resource: "requests",
				Version:  "v1",
			},
			Verb: "reconcile",
		},
		name: controllerName,
		request: req,
	}
}

// NewReconcileResultAction returns a new ReconcileResultAction.
func NewReconcileResultAction(controllerName string, req reconcile.Request, result reconcile.Result, err error) ReconcileResultAction {
	return &reconcileResultAction{
		reconcileRequestAction: reconcileRequestAction{
			ActionImpl: clienttesting.ActionImpl{
				Resource: schema.GroupVersionResource{
					Group:    "testing.reconciler",
					Resource: "results",
					Version:  "v1",
				},
				Verb: "reconcile",
			},
			name: controllerName,
			request: req,
		},
		result: result,
		err:    err,
	}
}

type reconcileRequestAction struct {
	clienttesting.ActionImpl
	name    string
	request reconcile.Request
}

var _ ReconcileRequestAction = (*reconcileRequestAction)(nil)

func (r *reconcileRequestAction) GetName() string {
	return r.name
}

func (r *reconcileRequestAction) GetReconcileRequest() reconcile.Request {
	return r.request
}

func (r *reconcileRequestAction) DeepCopy() clienttesting.Action {
	ret := *r
	return &ret
}

type reconcileResultAction struct {
	reconcileRequestAction
	result  reconcile.Result
	err     error
}

var _ ReconcileResultAction = (*reconcileResultAction)(nil)

func (r *reconcileResultAction) GetName() string {
	return r.name
}

func (r *reconcileResultAction) GetReconcileRequest() reconcile.Request {
	return r.request
}

func (r *reconcileResultAction) GetReconcileResult() (reconcile.Result, error) {
	return r.result, r.err
}

func (r *reconcileResultAction) DeepCopy() clienttesting.Action {
	ret := *r
	return &ret
}

type reconcilerWrapper struct {
	reconcile.Reconciler
	manager        *FakeManager
	controllerName string
}

func (r *reconcilerWrapper) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	_, err := r.manager.ObjectTracker.Invokes(NewReconcileRequestAction(r.controllerName, req), nil)
	// TODO: consider allowing the reactor to return a complete reconcile.Result and "real" error
	if err != nil {
		return reconcile.Result{}, nil
	}
	result, reconcileRrr := r.Reconciler.Reconcile(req)
	_, err = r.manager.ObjectTracker.Invokes(NewReconcileResultAction(r.controllerName, req, result, reconcileRrr), nil)
	// TODO: consider creating runtime.Object for request/result, allowing the result to be overridden
	// currently, we do allow the caller to override the error
	// by default, error returned by tracker will be nil, which prevents additional requests
	return result, err
}

// WaitForReconcileCompletion waits for all active reconciliations to complete.
// This includes reconcilations that may have started after this function was
// called, but prior to other active reconcilations completing. For example,
// when testing multiple controllers, the actions of one controller may trigger
// a reconcilation by another.  This function will wait until the controllers
// have settled to an idle state.  There is a danger that controllers could
// bounce events back and forth, causing this call to effectively hang.
func (m *FakeManager) WaitForReconcileCompletion(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		m.requestTracker.Wait()
		close(done)
	}()
	select {
	case <-done:
		return false
	case <-time.After(timeout):
		return true
	}
}

type requestTracker struct {
	cond  *sync.Cond
	count int
}

func (rt *requestTracker) Increment() {
	rt.cond.L.Lock()
	defer rt.cond.L.Unlock()
	rt.count++
}

func (rt *requestTracker) Decrement() {
	rt.cond.L.Lock()
	defer rt.cond.L.Unlock()
	rt.count--
	if rt.count == 0 {
		rt.cond.Broadcast()
	}
}

func (rt *requestTracker) Wait() {
	waiting := true
	for waiting {
		func() {
			rt.cond.L.Lock()
			defer rt.cond.L.Unlock()
			if rt.count == 0 {
				waiting = false
			} else {
				rt.cond.Wait()
			}
		}()
	}
}

// trackingQueue is designed to increment message count when messages are added
// and decrement message count when processing is completed.  Because the
// standard workqueue doesn't add messages if they match ones already in the
// queue and delays adding messages if they match ones already being processed,
// we can't simply track Add()/Done(), and Len() will not include any messages
// being deferred until processing is complete.  Because of this, we simply
// wrap the queue and provide our own condition using in Add(), Get(), and
// Done().  Doing this allows us to use Len() to appropriately increment or
// decrement the counter.
type trackingQueue struct {
	workqueue.RateLimitingInterface
	cond           *sync.Cond
	requestTracker *requestTracker
}

func newTrackingQueue(q workqueue.RateLimitingInterface, requestTracker *requestTracker) *trackingQueue {
	return &trackingQueue{
		RateLimitingInterface: q,
		requestTracker:        requestTracker,
		cond:                  sync.NewCond(&sync.Mutex{}),
	}
}

func (q *trackingQueue) Add(item interface{}) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	oldLen := q.Len()
	q.RateLimitingInterface.Add(item)

	if oldLen != q.Len() {
		// item was added to queue
		q.requestTracker.Increment()
		// notify processors
		q.cond.Signal()
	}
	// else we don't need to notify because the item is already in the queue or
	// will be added back when its processor calls Done()
}

func (q *trackingQueue) AddRateLimited(item interface{}) {
	// No rate limiting for tests, i.e. no backoff, no delay.  we want tests to execute consistently.
	q.Add(item)
}

func (q *trackingQueue) AddAfter(item interface{}, _ time.Duration) {
	// No delays
	q.Add(item)
}

func (q *trackingQueue) Get() (item interface{}, shutdown bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if q.Len() == 0 {
		// wait for an item to appear on the queue
		q.cond.Wait()
	}
	return q.RateLimitingInterface.Get()
}

func (q *trackingQueue) Done(item interface{}) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	oldLen := q.Len()
	q.RateLimitingInterface.Done(item)
	if oldLen == q.Len() {
		// If the length of the queue didn't change, the item was not added back
		// onto the queue
		q.requestTracker.Decrement()
	} else {
		// item was added back onto queue, notify processors
		q.cond.Signal()
	}
}

func (q *trackingQueue) Shutdown() {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	q.RateLimitingInterface.ShutDown()
	q.cond.Broadcast()
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
		// initialize common fields
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
		// correct deletion processing
		if len(accessor.GetFinalizers()) > 0 {
			if accessor.GetDeletionTimestamp() == nil {
				now := metav1.Now()
				accessor.SetDeletionTimestamp(&now)
				err = tracker.Update(deleteAction.GetResource(), obj, deleteAction.GetNamespace())
			}
			// XXX: not sure if this is correct, the object is already marked for deletion, but still has finalizers registered
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
		// update fields that would get modified by the api server on an update
		accessor.SetResourceVersion(fmt.Sprintf("%d", rand.Int()))
		if accessor.GetDeletionTimestamp() == nil && len(updateAction.GetSubresource()) == 0 {
			existingObj, err := tracker.Get(action.GetResource(), accessor.GetNamespace(), accessor.GetName())
			if err == nil && specChanged(obj, existingObj) {
				// this should only be done if the .spec field actually changes.
				accessor.SetGeneration(accessor.GetGeneration() + 1)
			}
		}
		err = tracker.Update(updateAction.GetResource(), obj, accessor.GetNamespace())
		return true, obj, err
	})
}

func specChanged(new, old runtime.Object) (changed bool) {
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("panic'd checking spec fields")
				changed = true
			}
		}()
		newSpec := reflect.ValueOf(new).Elem().FieldByName("Spec")
		oldSpec := reflect.ValueOf(old).Elem().FieldByName("Spec")
		if newSpec.IsValid() {
			if newSpec.CanInterface() {
				changed = !reflect.DeepEqual(newSpec.Interface(), oldSpec.Interface())
			} else {
				changed = !reflect.DeepEqual(newSpec.Elem().Interface(), oldSpec.Elem().Interface)
			}
		}
	}()
	return
}
