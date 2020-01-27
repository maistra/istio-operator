package test

import (
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

// ReactorFactory provides a DSL for specifying clienttesting.Reactor objects
// used with a ControllerTestCase.
type ReactorFactory struct {
    AbstractActionFilter
}

// ReactTo creates a new factory applied to the specified verb.
func ReactTo(verb string) *ReactorFactory {
	return &ReactorFactory{
		AbstractActionFilter: AbstractActionFilter{
			Verb:        verb,
			Namespace:   "*",
			Name:        "*",
			Resource:    "*",
			Subresource: "*",
		},
	}
}

// On initializes the resource and subresource name to which the created
// verifier should apply.  resource parameter should be specified using a slash
// between resource an subresource, e.g. deployments/status.  Use "*" to match
// all resources.
func (f *ReactorFactory) On(resource string) *ReactorFactory {
    f.AbstractActionFilter.On(resource)
    return f
}

// In initializes the namespace whithin which the created verifier should apply.
// Use "*" to match all namespaces.
func (f *ReactorFactory) In(namespace string) *ReactorFactory {
    f.AbstractActionFilter.In(namespace)
    return f
}

// Named initializes the name of the resource to which the created verifier
// should apply.  Use "*" to match all names.
func (f *ReactorFactory) Named(name string) *ReactorFactory {
    f.AbstractActionFilter.Named(name)
    return f
}

// ReactionFunc is a specialized version of clienttesting.ReactionFunc, which
// includes an ObjectTracker that can be used in the reaction.
type ReactionFunc func (action clienttesting.Action, tracker clienttesting.ObjectTracker) (applied bool, handled bool, obj runtime.Object, err error)

// With returns a reactor that applies the reaction to the filtered action.
func (f *ReactorFactory) With(reaction ReactionFunc) clienttesting.Reactor {
    return f.WithSequence(reaction)
}

// WithSequence returns a reactor that applies a subsequent reaction to each
// subsequent action matching the filter, e.g. the first occurrence will invoke
// the first reaction, the second occurrence will invoke the second reaction,
// etc.
func (f *ReactorFactory) WithSequence(reactions ...ReactionFunc) clienttesting.Reactor {
    return &Reactor{
        AbstractActionFilter: f.AbstractActionFilter,
        reactions: reactions,
    }
}

// Reactor implments clienttesting.Reactor, but allows access to the underlying
// clienttesting.ObjectTracker used by the test runner.  This can be useful for
// modifying objects in the chain, primarily with intercepting list actions.
type Reactor struct {
    AbstractActionFilter
    tracker clienttesting.ObjectTracker
    reactions []ReactionFunc
}

var _ clienttesting.Reactor = (*Reactor)(nil)
var _ TrackerInjector = (*Reactor)(nil)

// Handles returns true if the action matches the settings for this reactor
// (verb, resource, subresource, namespace, and name) and the reactions list has
// not been exhausted.
func (r *Reactor) Handles(action clienttesting.Action) bool {
    return len(r.reactions) > 0 && r.AbstractActionFilter.Handles(action)
}

// React invokes the reaction at the front of the reactions list, then removes
// it from the list.
func (r *Reactor) React(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
    var applied bool
    reaction := r.reactions[0]
    defer func() {
        if applied {
            r.reactions = r.reactions[1:]
        }
    }()
    applied, handled, ret, err = reaction(action, r.tracker)
    return
}

// InjectTracker sets the tracker used by this Reactor
func (r *Reactor) InjectTracker(tracker clienttesting.ObjectTracker) {
    r.tracker = tracker
}
