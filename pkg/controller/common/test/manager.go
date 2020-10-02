package test

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
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
		defer close(startChannel)
		if err := mgr.Start(stopChannel); err != nil {
			t.Errorf("Uexpected error returned from manager.Manager: %v", err)
			return
		}
		t.Logf("manager.Manager stopped cleanly")
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
func NewManager(scheme *runtime.Scheme, tracker clienttesting.ObjectTracker, groupResources ...*restmapper.APIGroupResources) (*FakeManager, error) {
	// Known cluster resources
	clusterKinds := sets.NewString(
		"Namespace",
		"CustomResourceDefinition",
		"ClusterRole",
		"ClusterRoleBinding",
	)
	// Initialize resource mapper with known kinds
	specifiedGroups := sets.NewString()
	for _, group := range groupResources {
		specifiedGroups.Insert(group.Group.Name)
	}

	autoGroups := map[string]struct {
		versions  sets.String
		resources map[string][]metav1.APIResource
	}{}
	for kind := range scheme.AllKnownTypes() {
		if specifiedGroups.Has(kind.Group) || strings.HasSuffix(kind.Kind, "List") {
			continue
		}
		group, ok := autoGroups[kind.Group]
		if !ok {
			group = struct {
				versions  sets.String
				resources map[string][]metav1.APIResource
			}{
				versions:  sets.NewString(),
				resources: make(map[string][]metav1.APIResource),
			}
			autoGroups[kind.Group] = group
		}
		plural, singular := meta.UnsafeGuessKindToResource(kind)
		group.versions.Insert(kind.Version)
		group.resources[kind.Version] = append(group.resources[kind.Version], metav1.APIResource{
			Name:         plural.Resource,
			SingularName: singular.Resource,
			Namespaced:   clusterKinds.Has(kind.Kind),
			Kind:         kind.Kind,
		})
	}
	for group, resources := range autoGroups {
		versions := make([]metav1.GroupVersionForDiscovery, resources.versions.Len())
		for index, version := range resources.versions.List() {
			versions[index] = metav1.GroupVersionForDiscovery{
				Version: version,
			}
		}
		groupResources = append(groupResources, &restmapper.APIGroupResources{
			Group: metav1.APIGroup{
				Name:     group,
				Versions: versions,
			},
			VersionedResources: resources.resources,
		})
	}

	options := NewManagerOptions(scheme, tracker, groupResources...)
	delegate, err := manager.New(&rest.Config{}, options)
	if err != nil {
		return nil, err
	}
	return &FakeManager{
		Manager:          delegate,
		recorderProvider: NewRecorderProvider(scheme),
		requestTracker:   requestTracker{cond: sync.NewCond(&sync.Mutex{})},
	}, nil
}

// GetEventRecorderFor returns a new EventRecorder for the provided name
func (m *FakeManager) GetEventRecorderFor(name string) record.EventRecorder {
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
		queueValue := value.Elem().FieldByName("MakeQueue")
		if !queueValue.IsValid() {
			panic(fmt.Errorf("cannot hook controller.Queue"))
		}
		if makeQ, ok := queueValue.Interface().(func() workqueue.RateLimitingInterface); ok {
			makeQ = newTrackingQueue(makeQ, &m.requestTracker)
			queueValue.Set(reflect.ValueOf(makeQ))
		}
	}
	return m.Manager.Add(runnable)
}

// WaitForFirstEvent waits until an event is seen by the manager.  This prevents
// immediate completion of tests when no verifiers have been registered.
func (m *FakeManager) WaitForFirstEvent() {
	waiting := true
	for waiting {
		func() {
			m.requestTracker.cond.L.Lock()
			defer m.requestTracker.cond.L.Unlock()
			if m.requestTracker.started {
				waiting = false
			} else {
				m.requestTracker.cond.Wait()
			}
		}()
	}
}

// WaitForReconcileCompletion waits for all active reconciliations to complete.
// This includes reconcilations that may have started after this function was
// called, but prior to other active reconcilations completing. For example,
// when testing multiple controllers, the actions of one controller may trigger
// a reconcilation by another.  This function will wait until the controllers
// have settled to an idle state.  There is a danger that controllers could
// bounce events back and forth, causing this call to effectively hang.
func (m *FakeManager) WaitForReconcileCompletion() {
	m.requestTracker.Wait()
}

type requestTracker struct {
	cond    *sync.Cond
	count   int
	started bool
}

func (rt *requestTracker) Increment() {
	rt.cond.L.Lock()
	defer rt.cond.L.Unlock()
	rt.count++
	if !rt.started {
		rt.started = true
		rt.cond.Broadcast()
	}
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

func newTrackingQueue(makeQ func() workqueue.RateLimitingInterface, requestTracker *requestTracker) func() workqueue.RateLimitingInterface {
	return func() workqueue.RateLimitingInterface {
		return &trackingQueue{
			RateLimitingInterface: makeQ(),
			requestTracker:        requestTracker,
			cond:                  sync.NewCond(&sync.Mutex{}),
		}
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

func newCacheFunc(tracker clienttesting.ObjectTracker) cache.NewCacheFunc {
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
