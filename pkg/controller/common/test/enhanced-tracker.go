package test

import (
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/testing"
)

var dummyDefaultObject = &struct{ runtime.Object }{}

// EnhancedTracker is a testing.ObjectTracker that is implemented by a
// testing.Fake, which delegates to an embedded testing.ObjectTracker for
// unhandled actions (i.e. testing.ObjectReaction is always the last
// testing.Reaction in its ReactionChain).
type EnhancedTracker struct {
	testing.Fake
	testing.ObjectTracker
	Scheme         *runtime.Scheme
	Decoder        runtime.Decoder
	GroupVersioner runtime.GroupVersioner
}

var _ testing.ObjectTracker = (*EnhancedTracker)(nil)

// NewEnhancedTracker returns a new EnhancedTracker, backed by the delegate.
func NewEnhancedTracker(delegate testing.ObjectTracker, scheme *runtime.Scheme, storageVersions ...schema.GroupVersion) *EnhancedTracker {
	tracker := &EnhancedTracker{
		ObjectTracker:  delegate,
		Scheme:         scheme,
		Decoder:        serializer.NewCodecFactory(scheme).UniversalDecoder(),
		GroupVersioner: runtime.GroupVersioner(schema.GroupVersions(storageVersions)),
	}
	tracker.Fake.AddReactor("*", "*", testing.ObjectReaction(tracker))
	tracker.Fake.AddWatchReactor("*", func(action testing.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := tracker.Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})
	return tracker
}

// AddReactor adds a SimpleReactor to the end of the ReactionChain
func (t *EnhancedTracker) AddReactor(verb, resource string, reaction testing.ReactionFunc) {
	t.AddReaction(&testing.SimpleReactor{Verb: verb, Resource: resource, Reaction: reaction})
}

// AddReaction adds reactors to the end of the ReactionChain
func (t *EnhancedTracker) AddReaction(reactors ...testing.Reactor) {
	// inject ourself, if necessary
	for _, reactor := range reactors {
		if injectTracker, ok := reactor.(TrackerInjector); ok {
			injectTracker.InjectTracker(t)
		}
	}
	objectReactionPosition := len(t.ReactionChain) - 1
	objectReaction := t.ReactionChain[objectReactionPosition]
	t.ReactionChain = append(append(t.ReactionChain[:objectReactionPosition], reactors...), objectReaction)
}

// PrependReaction adds reactors to the front of the ReactionChain
func (t *EnhancedTracker) PrependReaction(reactors ...testing.Reactor) {
	// inject ourself, if necessary
	for _, reactor := range reactors {
		if injectTracker, ok := reactor.(TrackerInjector); ok {
			injectTracker.InjectTracker(t)
		}
	}
	t.ReactionChain = append(reactors, t.ReactionChain...)
}

// RemoveReaction removes the reactors from the ReactionChain
func (t *EnhancedTracker) RemoveReaction(reactors ...testing.Reactor) {
	for _, reactor := range reactors {
		for index, existing := range t.ReactionChain {
			if reactor == existing {
				t.ReactionChain = append(t.ReactionChain[:index], t.ReactionChain[index+1:]...)
				break
			}
		}
	}
}

// AddWatchReactor adds a SimpleWatchReactor to the end of the WatchReactionChain
func (t *EnhancedTracker) AddWatchReactor(resource string, reaction testing.WatchReactionFunc) {
	t.AddWatchReaction(&testing.SimpleWatchReactor{Resource: resource, Reaction: reaction})
}

// AddWatchReaction adds reactors to the end of the WatchReactionChain
func (t *EnhancedTracker) AddWatchReaction(reactors ...testing.WatchReactor) {
	// inject ourself, if necessary
	for _, reactor := range reactors {
		if injectTracker, ok := reactor.(TrackerInjector); ok {
			injectTracker.InjectTracker(t)
		}
	}
	objectReactionPosition := len(t.WatchReactionChain) - 1
	objectReaction := t.WatchReactionChain[objectReactionPosition]
	t.WatchReactionChain = append(append(t.WatchReactionChain[:objectReactionPosition], reactors...), objectReaction)
}

// PrependWatchReaction adds reactors to the front of the WatchReactionChain
func (t *EnhancedTracker) PrependWatchReaction(reactors ...testing.WatchReactor) {
	// inject ourself, if necessary
	for _, reactor := range reactors {
		if injectTracker, ok := reactor.(TrackerInjector); ok {
			injectTracker.InjectTracker(t)
		}
	}
	t.WatchReactionChain = append(reactors, t.WatchReactionChain...)
}

// AddProxyReaction adds reactors to the end of the ProxyReactionChain
func (t *EnhancedTracker) AddProxyReaction(reactors ...testing.ProxyReactor) {
	// inject ourself, if necessary
	for _, reactor := range reactors {
		if injectTracker, ok := reactor.(TrackerInjector); ok {
			injectTracker.InjectTracker(t)
		}
	}
	t.ProxyReactionChain = append(t.ProxyReactionChain, reactors...)
}

// PrependProxyReaction adds reactors to the front of the ProxyReactionChain
func (t *EnhancedTracker) PrependProxyReaction(reactors ...testing.ProxyReactor) {
	// inject ourself, if necessary
	for _, reactor := range reactors {
		if injectTracker, ok := reactor.(TrackerInjector); ok {
			injectTracker.InjectTracker(t)
		}
	}
	t.ProxyReactionChain = append(reactors, t.ProxyReactionChain...)
}

func (t *EnhancedTracker) yeild() {
	// yeild to allow watches to process the change before returning
	time.Sleep(5 * time.Millisecond)
}

func (t *EnhancedTracker) Add(obj runtime.Object) error {
	gvks, unversioned, err := t.Scheme.ObjectKinds(obj)
	if err != nil {
		return err
	} else if unversioned {
		return t.ObjectTracker.Add(obj)
	} else if len(gvks) == 0 {
		return fmt.Errorf("no registered kinds for %v", obj)
	}

	// convert to preferred version
	preferred, _, err := t.convertToPreferredType(obj, schema.GroupVersionResource{})
	if err != nil {
		return err
	}
	err = t.ObjectTracker.Add(preferred)
	return err
}

// Create creates the obj in the embedded ObjectTracker.  Before creating the
// object in the tracker, the object is converted to a known type if it is
// unstructured.  This allows registered watches to behave correctly
// (i.e. avoids type assertions).
func (t *EnhancedTracker) Create(gvr schema.GroupVersionResource, obj runtime.Object, ns string) (err error) {
	if unstObj, ok := obj.(*unstructured.Unstructured); ok {
		// reconstitute the object into its native form
		if obj, err = ConvertToTypedIfKnown(unstObj, t.Scheme, t.Decoder); err != nil {
			return err
		}
	}
	t.Scheme.Default(obj)
	preferred, preferredGVR, err := t.convertToPreferredType(obj, gvr)
	if err != nil {
		return err
	}
	err = t.ObjectTracker.Create(preferredGVR, preferred, ns)
	// yeild to allow watches to process the change before returning
	t.yeild()
	return err
}

// Update updates the obj in the embedded ObjectTracker.  Before updating the
// object in the tracker, the object is converted to a known type if it is
// unstructured.  This allows registered watches to behave correctly
// (i.e. avoids type assertions).
func (t *EnhancedTracker) Update(gvr schema.GroupVersionResource, obj runtime.Object, ns string) (err error) {
	if unstObj, ok := obj.(*unstructured.Unstructured); ok {
		// reconstitute the object into its native form
		if obj, err = ConvertToTypedIfKnown(unstObj, t.Scheme, t.Decoder); err != nil {
			return err
		}
	}
	preferred, preferredGVR, err := t.convertToPreferredType(obj, gvr)
	if err != nil {
		return err
	}
	err = t.ObjectTracker.Update(preferredGVR, preferred, ns)
	// yeild to allow watches to process the change before returning
	t.yeild()
	return err

}

// Delete deltes the obj in the embedded ObjectTracker.  Before returning, we
// yield the processor to ensure watches have a chance to run before.
func (t *EnhancedTracker) Delete(gvr schema.GroupVersionResource, ns, name string) (err error) {
	defer func() {
		// yeild to allow watches to process the change before returning
		t.yeild()
	}()
	err = t.ObjectTracker.Delete(gvr, ns, name)
	if !errors.IsNotFound(err) {
		return err
	}
	// maybe stored as a preferred version
	versions := t.Scheme.PrioritizedVersionsForGroup(gvr.Group)
	if len(versions) == 0 || versions[0].Version == gvr.Version {
		return err
	}
	// get the stored version
	preferredGVR := schema.GroupVersionResource{Group: gvr.Group, Resource: gvr.Resource, Version: versions[0].Version}
	err = t.ObjectTracker.Delete(preferredGVR, ns, name)
	return err
}

func (t *EnhancedTracker) Get(gvr schema.GroupVersionResource, ns, name string) (runtime.Object, error) {
	obj, err := t.ObjectTracker.Get(gvr, ns, name)
	if !errors.IsNotFound(err) {
		return obj, err
	}
	// maybe stored as a preferred version
	versions := t.Scheme.PrioritizedVersionsForGroup(gvr.Group)
	if len(versions) == 0 || versions[0].Version == gvr.Version {
		return obj, err
	}
	// get the stored version
	preferredGVR := schema.GroupVersionResource{Group: gvr.Group, Resource: gvr.Resource, Version: versions[0].Version}
	obj, err = t.ObjectTracker.Get(preferredGVR, ns, name)
	if err != nil {
		return obj, err
	}
	// convert to desired version
	gvks, unversioned, err := t.Scheme.ObjectKinds(obj)
	if err != nil {
		return obj, err
	} else if unversioned {
		return obj, nil
	} else if len(gvks) == 0 {
		return obj, fmt.Errorf("no registered kinds for %v", obj)
	}
	desiredGVK := schema.GroupVersionKind{Group: gvr.Group, Kind: gvks[0].Kind, Version: gvr.Version}
	desired, err := t.Scheme.New(desiredGVK)
	if err != nil {
		return obj, err
	}
	err = t.Scheme.Convert(obj, desired, nil)
	return desired, err
}

func (t *EnhancedTracker) List(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
	storageGVR := gvr
	storageKind, ok := t.GroupVersioner.KindForGroupVersionKinds([]schema.GroupVersionKind{gvk})
	if !ok {
		storageKind = gvk
	} else if t.Scheme.Recognizes(storageKind) {
		storageGVR = schema.GroupVersionResource{Group: gvr.Group, Resource: gvr.Resource, Version: storageKind.Version}
	} else {
		// no kind for preferred version
		// XXX: need to create v2 kinds for ServiceMeshMember, ServiceMeshMemberRoll, etc.
		storageKind = gvk
	}

	if !t.Scheme.Recognizes(storageKind) {
		// register the type
		listGVK := storageKind
		listGVK.Kind = listGVK.Kind + "List"
		t.Scheme.AddKnownTypeWithName(storageKind, &unstructured.Unstructured{})
		t.Scheme.AddKnownTypeWithName(listGVK, &unstructured.UnstructuredList{})
	}
	// get the stored version
	obj, err := t.ObjectTracker.List(storageGVR, storageKind, ns)
	if err != nil || storageKind == gvk {
		return obj, err
	}
	// convert to desired version
	listGVK := gvk
	listGVK.Kind = listGVK.Kind + "List"
	desired, err := t.Scheme.New(listGVK)
	if err != nil {
		return obj, err
	}
	err = t.Scheme.Convert(obj, desired, nil)
	return desired, err
}

func (t *EnhancedTracker) convertToPreferredType(source runtime.Object, gvr schema.GroupVersionResource) (runtime.Object, schema.GroupVersionResource, error) {
	target, err := t.Scheme.UnsafeConvertToVersion(source, t.GroupVersioner)
	if err != nil {
		if runtime.IsNotRegisteredError(err) {
			return source, gvr, nil
		}
		// assume conversion is not possible
		return source, gvr, nil
	}
	gvks, _, err := t.Scheme.ObjectKinds(target)
	if err != nil {
		return target, gvr, err
	}
	preferredGVR := gvr
	preferredGVR.Version = gvks[0].Version
	return target, preferredGVR, err
}

// ConvertToTypedIfKnown returns a typed object for the GVK of the unstructured
// object, if the type is known to the Scheme.  If the type is unknown, the source
// object is returned.  An error indicates whether or not the conversion was successful.
func ConvertToTypedIfKnown(source *unstructured.Unstructured, scheme *runtime.Scheme, decoder runtime.Decoder) (runtime.Object, error) {
	// TODO: we should try to discover the preferred kind from the resource type
	// This would allow resources to be converted appropriately, e.g. from apps.v1beta1.Deployment to apps.v1.Deployment
	obj, err := scheme.New(source.GroupVersionKind())
	if err != nil {
		return source, nil
	}
	j, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	if _, _, err = decoder.Decode(j, nil, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// TrackerInjector should be implemented by types that wish to have a
// testing.ObjectTracker injected into them.
type TrackerInjector interface {
	InjectTracker(tracker testing.ObjectTracker)
}
