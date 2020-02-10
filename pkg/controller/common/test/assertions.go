package test

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

// Assert creates a new ActionAssertionFactory for the given verb.
// Use "*" to match any verb.  Unless the filter is narrowed, e.g. using On(),
// the ActionAssertion created by this factory will match all resource types in
// all namespaces (i.e. the other filter fields are initialized to "*").
func Assert(verb string) *ActionAssertionFactory {
	return &ActionAssertionFactory{
		AbstractActionFilter: AbstractActionFilter{
			Verb:        verb,
			Namespace:   "*",
			Name:        "*",
			Resource:    "*",
			Subresource: "*",
		},
	}
}

// ActionAssertionFactory serves as a factory for building ActionAssertion types
// that filter actions based on verb, resource, subresource, namespace, and name.
type ActionAssertionFactory struct {
	AbstractActionFilter
}

// On initializes the resource and subresource name to which the created
// assertion should apply.  resource parameter should be specified using a slash
// between resource an subresource, e.g. deployments/status.  Use "*" to match
// all resources.
func (f *ActionAssertionFactory) On(resource string) *ActionAssertionFactory {
    f.AbstractActionFilter.On(resource)
    return f
}

// In initializes the namespace whithin which the created assertion should apply.
// Use "*" to match all namespaces.
func (f *ActionAssertionFactory) In(namespace string) *ActionAssertionFactory {
    f.AbstractActionFilter.In(namespace)
    return f
}

// Named initializes the name of the resource to which the created assertion
// should apply.  Use "*" to match all names.
func (f *ActionAssertionFactory) Named(name string) *ActionAssertionFactory {
    f.AbstractActionFilter.Named(name)
    return f
}

// SeenCountIs returns an ActionAssertion object that asserts the specified event
// has been seen the expected number of times.
func (f *ActionAssertionFactory) SeenCountIs(expected int) ActionAssertion {
	return &ActionSeenCountAssertion{AbstractActionFilter: f.AbstractActionFilter, Expected: expected}
}

// IsSeen returns an ActionAssertion object that asserts the specified event
// has been seen.
func (f *ActionAssertionFactory) IsSeen() ActionAssertion {
	return &ActionIsSeenAssertion{AbstractActionFilter: f.AbstractActionFilter}
}

// IsNotSeen returns an ActionAssertion object that asserts the specified
// event has not been seen.
func (f *ActionAssertionFactory) IsNotSeen() ActionAssertion {
	return &ActionNotSeenAssertion{ActionIsSeenAssertion: ActionIsSeenAssertion{AbstractActionFilter: f.AbstractActionFilter}}
}

// ActionSeenCountAssertion asserts that a required number of actions matching its ActionAssertionFilter have been seen.
type ActionSeenCountAssertion struct {
	// AbstractActionFilter filters the actions to be included in the count.
	AbstractActionFilter
	// Expected number of actions passing the filter
	Expected int
	// seen is the number of actions that passed the filter
	seen int
}

var _ ActionAssertion = (*ActionSeenCountAssertion)(nil)

// React increments the number of seen actions.
func (a *ActionSeenCountAssertion) React(_ clienttesting.Action) (bool, runtime.Object, error) {
	a.seen++
	return false, nil, nil
}

// Assert that the number of actions seen matches the Expected number.
func (a *ActionSeenCountAssertion) Assert(t *testing.T) {
	t.Helper()
	if a.seen != a.Expected {
		t.Errorf("unexpected number of '%s' actions: expected %d, saw %d.", a.AbstractActionFilter.String(), a.Expected, a.seen)
	}
}

// ActionIsSeenAssertion asserts that an action matching its ActionAssertionFilter has been seen.
type ActionIsSeenAssertion struct {
	AbstractActionFilter
	seen bool
}

var _ ActionAssertion = (*ActionIsSeenAssertion)(nil)

// React marks the action as being seen.
func (a *ActionIsSeenAssertion) React(_ clienttesting.Action) (bool, runtime.Object, error) {
	a.seen = true
	return false, nil, nil
}

// Assert that the action has been seen.
func (a *ActionIsSeenAssertion) Assert(t *testing.T) {
	if !a.seen {
		t.Helper()
		t.Errorf("expected '%s' action not seen.", a.AbstractActionFilter.String())
	}
}

// ActionNotSeenAssertion asserts that an action matching its ActionAssertionFilter has not been seen.
type ActionNotSeenAssertion struct {
	ActionIsSeenAssertion
}

var _ ActionAssertion = (*ActionNotSeenAssertion)(nil)

// Assert that the action has not been seen.
func (a *ActionNotSeenAssertion) Assert(t *testing.T) {
	if a.seen {
		t.Helper()
		t.Errorf("unexpected '%s' action was seen.", a.AbstractActionFilter.String())
	}
}
