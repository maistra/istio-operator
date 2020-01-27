package test

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/recorder"
)

// FakeRecorderProvider provides a mock recorder.Provider that can be used with
// controller tests.
// TODO: consider exposing the ClientSet used with this so tests can query/listen for events
type FakeRecorderProvider struct {
	eventBroadcaster record.EventBroadcaster
	scheme           *runtime.Scheme
}

var _ recorder.Provider = (*FakeRecorderProvider)(nil)

// NewRecorderProvider returns a new FakeRecorderProvider.
func NewRecorderProvider(scheme *runtime.Scheme) recorder.Provider {
	clientSet := kubernetesfake.NewSimpleClientset()
	broadcaster := record.NewBroadcasterForTests(0)
	broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: &patchedEventInterface{
			EventInterface: clientSet.CoreV1().Events(""),
			Fake:           &clientSet.Fake,
			namespace:      "",
		},
	})
	return &FakeRecorderProvider{eventBroadcaster: broadcaster, scheme: scheme}
}

// GetEventRecorderFor returns an EventRecorder with given name.
func (p *FakeRecorderProvider) GetEventRecorderFor(name string) record.EventRecorder {
	return p.eventBroadcaster.NewRecorder(p.scheme, corev1.EventSource{Component: name})
}

type patchedEventInterface struct {
	typedcorev1.EventInterface
	*testing.Fake
	namespace string
}

var eventsResource = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"}
var eventsKind = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Event"}

// XXX: the generated version of this DOES NOT use the EventNamespace!!!!
func (c *patchedEventInterface) CreateWithEventNamespace(event *v1.Event) (*v1.Event, error) {
	action := testing.NewCreateAction(eventsResource, event.GetNamespace(), event)
	obj, err := c.Invokes(action, event)
	if obj == nil {
		return nil, err
	}

	return obj.(*v1.Event), err
}

// XXX: the generated version of this DOES NOT use the EventNamespace!!!!
func (c *patchedEventInterface) UpdateWithEventNamespace(event *v1.Event) (*v1.Event, error) {
	action := testing.NewUpdateAction(eventsResource, event.GetNamespace(), event)
	obj, err := c.Invokes(action, event)
	if obj == nil {
		return nil, err
	}

	return obj.(*v1.Event), err
}

// XXX: the generated version of this DOES NOT use the EventNamespace!!!!
func (c *patchedEventInterface) PatchWithEventNamespace(event *v1.Event, data []byte) (*v1.Event, error) {
	pt := types.StrategicMergePatchType
	action := testing.NewPatchAction(eventsResource, event.GetNamespace(), event.Name, pt, data)
	obj, err := c.Invokes(action, event)
	if obj == nil {
		return nil, err
	}

	return obj.(*v1.Event), err
}

// XXX: the generated version of this DOES NOT use the objOrRef namespace!!!!
func (c *patchedEventInterface) Search(scheme *runtime.Scheme, objOrRef runtime.Object) (*v1.EventList, error) {
	ref, err := reference.GetReference(scheme, objOrRef)
	if err != nil {
		return nil, err
	}
	if c.namespace != "" && ref.Namespace != c.namespace {
		return nil, fmt.Errorf("won't be able to find any events of namespace '%v' in namespace '%v'", ref.Namespace, c.namespace)
	}
	stringRefKind := string(ref.Kind)
	var refKind *string
	if stringRefKind != "" {
		refKind = &stringRefKind
	}
	stringRefUID := string(ref.UID)
	var refUID *string
	if stringRefUID != "" {
		refUID = &stringRefUID
	}
	fieldSelector := c.GetFieldSelector(&ref.Name, &ref.Namespace, refKind, refUID)
	return c.List(metav1.ListOptions{FieldSelector: fieldSelector.String()})
}
