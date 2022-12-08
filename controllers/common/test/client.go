/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	serializerjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// XXX: look for the XXX comments to see how this has changed from sigs.k8s.io/controller-runtime/pkg/client/fake/client.go

var log = logf.Log.WithName("fake-client")

type fakeClient struct {
	*testing.Fake
	scheme     *runtime.Scheme
	serializer runtime.Serializer
	mapper 		meta.RESTMapper
}

var _ client.Client = &fakeClient{}

// NewFakeClient creates a new fake client for testing.
// You can choose to initialize it with a slice of runtime.Object.
func NewFakeClient(initObjs ...runtime.Object) client.Client {
	return NewFakeClientWithScheme(scheme.Scheme, initObjs...)
}

// NewFakeClientWithScheme creates a new fake client with the given scheme
// for testing.
// You can choose to initialize it with a slice of runtime.Object.
func NewFakeClientWithScheme(clientScheme *runtime.Scheme, initObjs ...runtime.Object) client.Client {
	// XXX: use codecs corresponding to the actual scheme
	codecs := serializer.NewCodecFactory(clientScheme)
	tracker := testing.NewObjectTracker(clientScheme, codecs.UniversalDecoder())
	return NewFakeClientWithSchemeAndTracker(clientScheme, tracker, initObjs...)
}

// NewFakeClientWithSchemeAndTracker creates a new fake client with the
// given scheme and tracker for testing.
// You can choose to initialize it with a slice of runtime.Object.
func NewFakeClientWithSchemeAndTracker(clientScheme *runtime.Scheme, tracker testing.ObjectTracker, initObjs ...runtime.Object) client.Client {
	for _, obj := range initObjs {
		if obj == nil || reflect.ValueOf(obj).IsNil() {
			continue
		}
		err := tracker.Add(obj)
		if err != nil {
			log.Error(err, "failed to add object to fake client", "object", obj)
			os.Exit(1)
			return nil
		}
	}
	// XXX: use a serialize that corresponds with the actual scheme being used
	serializer := serializerjson.NewSerializer(serializerjson.DefaultMetaFactory, clientScheme, clientScheme, false)
	var enhancedTracker *EnhancedTracker
	var ok bool
	if enhancedTracker, ok = tracker.(*EnhancedTracker); !ok {
		enhancedTracker = NewEnhancedTracker(tracker, clientScheme)
	}
	return &fakeClient{
		Fake:       &enhancedTracker.Fake,
		scheme:     clientScheme,
		serializer: serializer,
	}
}

func (c *fakeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object,  opts ...client.GetOption) error {
	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	o, err := c.Invokes(testing.NewGetAction(gvr, key.Namespace, key.Name), nil)
	if err != nil {
		return err
	}
	if o == nil {
		return errors.NewNotFound(gvr.GroupResource(), key.Name)
	}
	return c.copyObject(o, obj)
}

func (c *fakeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	gvk, err := getGVKFromList(list, c.scheme)
	if err != nil {
		return err
	}

	listGVK := schema.GroupVersionKind{
		Kind:    gvk.Kind + "List",
		Group:   gvk.Group,
		Version: gvk.Version,
	}

	listOpts := client.ListOptions{}
	listOpts.ApplyOptions(opts)

	ns := listOpts.Namespace

	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	o, err := c.Invokes(testing.NewListAction(gvr, gvk, ns, *listOpts.AsListOptions()), nil)
	if err != nil {
		return err
	}
	if o == nil {
		return errors.NewInternalError(fmt.Errorf("no resource returned by Fake"))
	}
	j, err := json.Marshal(o)
	if err != nil {
		return err
	}
	// XXX: send a proper default GVK, to prevent errors when using unstructured, when the GVK may end up uninitialized
	_, _, err = c.serializer.Decode(j, &listGVK, list)
	if err != nil {
		return err
	}

	if opts != nil && listOpts.LabelSelector != nil {
		return filterListItems(list, listOpts.LabelSelector)
	}
	return nil
}

func filterListItems(list runtime.Object, labSel labels.Selector) error {
	objs, err := meta.ExtractList(list)
	if err != nil {
		return err
	}
	filteredObjs, err := FilterWithLabels(objs, labSel)
	if err != nil {
		return err
	}
	err = meta.SetList(list, filteredObjs)
	if err != nil {
		return err
	}
	return nil
}

func (c *fakeClient) Create(ctx context.Context, obj  client.Object, opts ...client.CreateOption) error {
	createOptions := &client.CreateOptions{}
	createOptions.ApplyOptions(opts)

	for _, dryRunOpt := range createOptions.DryRun {
		if dryRunOpt == metav1.DryRunAll {
			return nil
		}
	}

	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	o, err := c.Invokes(testing.NewCreateAction(gvr, accessor.GetNamespace(), obj), nil)
	if err != nil {
		return err
	}
	return c.copyObject(o, obj)
}

func (c *fakeClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	// TODO: implement propagation
	_, err = c.Invokes(testing.NewDeleteAction(gvr, accessor.GetNamespace(), accessor.GetName()), nil)
	return err
}

func (c *fakeClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return c.internalUpdate("", obj)
}

func (c *fakeClient) internalUpdate(subresource string, obj runtime.Object) error {
	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	o, err := c.Invokes(testing.NewUpdateSubresourceAction(gvr, subresource, accessor.GetNamespace(), obj), nil)
	if err != nil {
		return err
	}
	return c.copyObject(o, obj)
}

// Patch patches the given obj in the Kubernetes cluster. obj must be a
// struct pointer so that obj can be updated with the content returned by the Server.
func (c *fakeClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.internalPatch("", obj, patch, opts...)
}

func (c *fakeClient) internalPatch(subresource string, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	patchOptions := &client.PatchOptions{}
	patchOptions.ApplyOptions(opts)

	for _, dryRunOpt := range patchOptions.DryRun {
		if dryRunOpt == metav1.DryRunAll {
			return nil
		}
	}

	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	data, err := patch.Data(obj)
	if err != nil {
		return err
	}

	o, err := c.Invokes(testing.NewPatchSubresourceAction(gvr, accessor.GetNamespace(), accessor.GetName(), patch.Type(), data, subresource), nil)
	if err != nil {
		return err
	}

	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return err
	}
	ta, err := meta.TypeAccessor(o)
	if err != nil {
		return err
	}
	ta.SetKind(gvk.Kind)
	ta.SetAPIVersion(gvk.GroupVersion().String())

	j, err := json.Marshal(o)
	if err != nil {
		return err
	}
	decoder := scheme.Codecs.UniversalDecoder()
	_, _, err = decoder.Decode(j, nil, obj)
	return err
}

// DeleteAllOf deletes all objects of the given type matching the given options.
func (c *fakeClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return err
	}

	dcOptions := client.DeleteAllOfOptions{}
	dcOptions.ApplyOptions(opts)

	listOptions := client.ListOptions{
		LabelSelector: dcOptions.LabelSelector,
		Namespace:     dcOptions.Namespace,
		FieldSelector: dcOptions.FieldSelector,
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	o, err := c.Invokes(testing.NewListAction(gvr, gvk, listOptions.Namespace, *listOptions.AsListOptions()), nil)
	if err != nil {
		return err
	}

	objs, err := meta.ExtractList(o)
	if err != nil {
		return err
	}
	filteredObjs, err := FilterWithLabels(objs, dcOptions.LabelSelector)
	if err != nil {
		return err
	}
	for _, o := range filteredObjs {
		accessor, err := meta.Accessor(o)
		if err != nil {
			return err
		}
		_, err = c.Invokes(testing.NewDeleteAction(gvr, accessor.GetNamespace(), accessor.GetName()), nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *fakeClient) Status() client.StatusWriter {
	return &fakeStatusWriter{client: c}
}

func (c *fakeClient) copyObject(source, target runtime.Object) error {
	if source == nil {
		return errors.NewInternalError(fmt.Errorf("no resource returned by fake"))
	}
	j, err := runtime.Encode(c.serializer, source)
	if err != nil {
		return err
	}
	return runtime.DecodeInto(c.serializer, j, target)
}

// RESTMapper returns the scheme this client is using.
func (c *fakeClient) RESTMapper() meta.RESTMapper {
	return c.mapper
}

// Scheme returns the scheme this client is using.
func (c *fakeClient) Scheme() *runtime.Scheme {
	return c.scheme
}

func getGVRFromObject(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionResource, error) {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	return gvr, nil
}

func getGVKFromList(list runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
	gvk, err := apiutil.GVKForObject(list, scheme)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	if gvk.Kind == "List" {
		return schema.GroupVersionKind{}, fmt.Errorf("cannot derive GVK for generic List type %T (kind %q)", list, gvk)
	}

	if !strings.HasSuffix(gvk.Kind, "List") {
		// XXX: the real client does not produce this error. Revert if we should update our usage for listing.
		// return schema.GroupVersionKind{}, fmt.Errorf("non-list type %T (kind %q) passed as output", list, gvk)
		return gvk, nil
	}
	// we need the non-list GVK, so chop off the "List" from the end of the kind
	gvk.Kind = gvk.Kind[:len(gvk.Kind)-4]
	return gvk, nil
}

type fakeStatusWriter struct {
	client *fakeClient
}

func (sw *fakeStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	// TODO(droot): This results in full update of the obj (spec + status). Need
	// a way to update status field only.
	return sw.client.internalUpdate("status", obj)
}

// Patch patches the given object's subresource. obj must be a struct
// pointer so that obj can be updated with the content returned by the
// Server.
func (sw *fakeStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return sw.client.internalPatch("status", obj, patch, opts...)
}

// from sigs.k8s.io/controller-runtime/pkg/internal/objectutil

// FilterWithLabels returns a copy of the items in objs matching labelSel
func FilterWithLabels(objs []runtime.Object, labelSel labels.Selector) ([]runtime.Object, error) {
	outItems := make([]runtime.Object, 0, len(objs))
	for _, obj := range objs {
		meta, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		if labelSel != nil {
			lbls := labels.Set(meta.GetLabels())
			if !labelSel.Matches(lbls) {
				continue
			}
		}
		outItems = append(outItems, obj.DeepCopyObject())
	}
	return outItems, nil
}
