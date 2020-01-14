package test

import (
	"sync"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/testing"
)

type ReactFunc func(testing.Action) (handled bool, err error)

type EnhancedTracker struct {
	sync.RWMutex
	actions  []testing.Action // these may be castable to other types, but "Action" is the minimum
	reactors []ReactFunc

	delegate testing.ObjectTracker
}

func NewEnhancedTracker(delegate testing.ObjectTracker) EnhancedTracker {
	return EnhancedTracker{
		delegate: delegate,
	}
}

func (t *EnhancedTracker) AddReactor(reactor ReactFunc) {
	t.reactors = append(t.reactors, reactor)
}

func (t *EnhancedTracker) Add(obj runtime.Object) error {
	return t.delegate.Add(obj)
}

func (t *EnhancedTracker) Get(gvr schema.GroupVersionResource, ns, name string) (runtime.Object, error) {
	action := testing.NewGetAction(gvr, ns, name)
	t.recordAction(action)
	handled, err := t.invokeReactors(action)
	if handled || err != nil {
		return nil, err
	}
	return t.delegate.Get(gvr, ns, name)
}

func (t *EnhancedTracker) Create(gvr schema.GroupVersionResource, obj runtime.Object, ns string) error {
	action := testing.NewCreateAction(gvr, ns, obj)
	t.recordAction(action)
	handled, err := t.invokeReactors(action)
	if handled || err != nil {
		return err
	}
	return t.delegate.Create(gvr, obj, ns)
}

func (t *EnhancedTracker) invokeReactors(action testing.Action) (handled bool, err error) {
	for _, f := range t.reactors {
		handled, err := f(action)
		if handled || err != nil {
			return handled, err
		}
	}
	return false, nil
}

func (t *EnhancedTracker) Update(gvr schema.GroupVersionResource, obj runtime.Object, ns string) error {
	action := testing.NewUpdateAction(gvr, ns, obj)
	t.recordAction(action)
	handled, err := t.invokeReactors(action)
	if handled || err != nil {
		return err
	}
	return t.delegate.Update(gvr, obj, ns)
}

func (t *EnhancedTracker) List(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
	action := testing.NewListAction(gvr, gvk, ns, meta.ListOptions{})
	t.recordAction(action)
	handled, err := t.invokeReactors(action)
	if handled || err != nil {
		return nil, err
	}
	return t.delegate.List(gvr, gvk, ns)
}

func (t *EnhancedTracker) Delete(gvr schema.GroupVersionResource, ns, name string) error {
	action := testing.NewDeleteAction(gvr, ns, name)
	t.recordAction(action)
	handled, err := t.invokeReactors(action)
	if handled || err != nil {
		return err
	}
	return t.delegate.Delete(gvr, ns, name)
}

func (t *EnhancedTracker) Watch(gvr schema.GroupVersionResource, ns string) (watch.Interface, error) {
	action := testing.NewWatchAction(gvr, ns, nil)
	t.recordAction(action)
	handled, err := t.invokeReactors(action)
	if handled || err != nil {
		return nil, err
	}
	return t.delegate.Watch(gvr, ns)
}

func (t *EnhancedTracker) recordAction(action testing.Action) {
	t.Lock()
	defer t.Unlock()
	t.actions = append(t.actions, action.DeepCopy())
}

// ClearActions clears the history of actions called on the fake client.
func (t *EnhancedTracker) ClearActions() {
	t.Lock()
	defer t.Unlock()
	t.actions = make([]testing.Action, 0)
}

// Actions returns a chronologically ordered slice fake actions called on the
// fake client.
func (t *EnhancedTracker) Actions() []testing.Action {
	t.RLock()
	defer t.RUnlock()
	fa := make([]testing.Action, len(t.actions))
	copy(fa, t.actions)
	return fa
}
