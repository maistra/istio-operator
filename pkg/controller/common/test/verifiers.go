package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

// Verify creates a new ActionVerifierFactory for the given verb.
// Use "*" to match any verb.  Unless the filter is narrowed, e.g. using On(),
// the ActionVerifier created by this factory will match all resource types in
// all namespaces (i.e. the other filter fields are initialized to "*").
func Verify(verb string) *ActionVerifierFactory {
	return &ActionVerifierFactory{
		AbstractActionFilter: AbstractActionFilter{
			Verb:        verb,
			Namespace:   "*",
			Name:        "*",
			Resource:    "*",
			Subresource: "*",
		},
	}
}

// ActionVerifierFactory is a factory for creating common verifiers
type ActionVerifierFactory struct {
	AbstractActionFilter
}

// On initializes the resource and subresource name to which the created
// verifier should apply.  resource parameter should be specified using a slash
// between resource an subresource, e.g. deployments/status.  Use "*" to match
// all resources.
func (f *ActionVerifierFactory) On(resource string) *ActionVerifierFactory {
	f.AbstractActionFilter.On(resource)
	return f
}

// Version initializes the version whithin which the created verifier should apply.
// Use "*" to match all versions.
func (f *ActionVerifierFactory) Version(version string) *ActionVerifierFactory {
	f.AbstractActionFilter.Version(version)
	return f
}

// In initializes the namespace whithin which the created verifier should apply.
// Use "*" to match all namespaces.
func (f *ActionVerifierFactory) In(namespace string) *ActionVerifierFactory {
	f.AbstractActionFilter.In(namespace)
	return f
}

// Named initializes the name of the resource to which the created verifier
// should apply.  Use "*" to match all names.
func (f *ActionVerifierFactory) Named(name string) *ActionVerifierFactory {
	f.AbstractActionFilter.Named(name)
	return f
}

// IsSeen returns an ActionVerifier that verifies the specified action has occurred.
func (f *ActionVerifierFactory) IsSeen() ActionVerifier {
	return NewSimpleActionVerifier(f.Verb, f.Resource, f.Subresource, f.Namespace, f.Name,
		func(action clienttesting.Action) (bool, error) {
			return true, nil
		})
}

// VerifierTestFunc is used for testing an action, returning an error if the test failed.
type VerifierTestFunc func(action clienttesting.Action) error

// Passes returns an ActionVerifier that verifies the specified action has
// occurred and the test passes.
func (f *ActionVerifierFactory) Passes(test VerifierTestFunc) ActionVerifier {
	return NewSimpleActionVerifier(f.Verb, f.Resource, f.Subresource, f.Namespace, f.Name,
		func(action clienttesting.Action) (bool, error) {
			return true, test(action)
		})
}

/*
SimpleActionVerifier is a simple ActionVerifier that applies the validation
logic when verb/resource/subresource/name/namespace match an action.  The
verification logic is only executed once.  This can be used as the base for a
custom verifier by overriding the Handles() method, e.g.

	type CustomActionVerifier struct {
		test.SimpleActionVerifier
	}

	func (v *CustomActionVerifier) Handles(action clienttesting.Action) bool {
		if v.SimpleActionVerifier.Handles(action) {
			// custom handling logic
			return true
		}
		return false
	}

	customVerifier := &CustomActionVerifier{SimpleActionVerifier: test.VerifyAction(...)}
*/
type SimpleActionVerifier struct {
	AbstractActionFilter
	Verify ActionVerifierFunc
	fired  bool
	Notify chan struct{}
	t      *testing.T
}

var _ ActionVerifier = (*SimpleActionVerifier)(nil)

// NewSimpleActionVerifier returns a new ActionVerifier that filtering the
// specified verb, resource, etc., using the specified verifier function.
func NewSimpleActionVerifier(verb, resource, subresource, namespace, name string, verifier ActionVerifierFunc) *SimpleActionVerifier {
	return &SimpleActionVerifier{
		AbstractActionFilter: AbstractActionFilter{
			Namespace:   namespace,
			Name:        name,
			Verb:        verb,
			Resource:    resource,
			Subresource: subresource,
		},
		Verify: verifier,
		Notify: make(chan struct{}),
	}
}

// Handles returns true if the action matches the settings for this verifier
// (verb, resource, subresource, namespace, and name) and the verifier has not
// already been applied.
func (v *SimpleActionVerifier) Handles(action clienttesting.Action) bool {
	if v.fired {
		return false
	}
	return v.AbstractActionFilter.Handles(action)
}

// React to the action.  This method always returns false for handled, as it only
// verifies the action.  It does not perform the action.  If verification fails,
// it will register a fatal error with the test runner.
func (v *SimpleActionVerifier) React(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
	v.t.Helper()
	if handled, err := v.Verify(action); handled || err != nil {
		v.fired = true
		defer close(v.Notify)
		if err != nil {
			v.t.Error(err)
		}
	}
	return false, nil, nil
}

// Wait until the verification has completed.  Returns true if it timed out waiting for verification.
func (v *SimpleActionVerifier) Wait(timeout time.Duration) (timedout bool) {
	v.t.Helper()
	if timeout > 0 {
		select {
		case <-v.Notify:
		case <-time.After(timeout):
			v.t.Errorf("verify %s timed out", v.AbstractActionFilter.String())
			return true
		}
	} else {
		select {
		case <-v.Notify:
		}
	}
	return false
}

// InjectTestRunner initializes the test runner for the verifier.
func (v *SimpleActionVerifier) InjectTestRunner(t *testing.T) {
	v.t = t
}

// HasFired returns true if this verifier has fired
func (v *SimpleActionVerifier) HasFired() bool {
	return v.fired
}

// VerifyActions is a list of ActionVerifier objects which are applied in order.
func VerifyActions(verifiers ...ActionVerifier) ActionVerifier {
	return &verifyActions{verifiers: verifiers}
}

type verifyActions struct {
	mu        sync.RWMutex
	verifiers []ActionVerifier
}

var _ ActionVerifier = (*verifyActions)(nil)

// Handles tests the head of the list to see if the action should be verified.
func (v *verifyActions) Handles(action clienttesting.Action) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.verifiers) > 0 && v.verifiers[0].Handles(action)
}

// React verifies the action using the ActionVerifier at that head of the list.
func (v *verifyActions) React(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.verifiers[0].React(action)
	if v.verifiers[0].HasFired() {
		v.verifiers = v.verifiers[1:]
	}
	return false, nil, nil
}

// Wait for all ActionVerifier elements in this list to complete.
func (v *verifyActions) Wait(timeout time.Duration) (timedout bool) {
	start := time.Now()
	v.mu.RLock()
	verifiers := v.verifiers[:]
	v.mu.RUnlock()
	for _, verifier := range verifiers {
		if timedout := verifier.Wait(timeout - time.Now().Sub(start)); timedout {
			return true
		}
	}
	return false
}

// InjectTestRunner injects the test runner into each ActionVerifier in the list.
func (v *verifyActions) InjectTestRunner(t *testing.T) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	for _, verifier := range v.verifiers {
		verifier.InjectTestRunner(t)
	}
}

// HasFired returns true if this verifier has fired
func (v *verifyActions) HasFired() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.verifiers) == 0
}

func (v *verifyActions) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return fmt.Sprintf("verifyActions: %s", v.verifiers)
}
