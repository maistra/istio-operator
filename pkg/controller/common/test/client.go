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
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	serializerjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// XXX: look for the XXX comments to see how this has changed from sigs.k8s.io/controller-runtime/pkg/client/fake/client.go

var (
	log = logf.KBLog.WithName("fake-client")
)

type fakeClient struct {
	tracker    testing.ObjectTracker
	scheme     *runtime.Scheme
	serializer runtime.Serializer
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
		err := tracker.Add(obj)
		if err != nil {
			log.Error(err, "failed to add object to fake client", "object", obj)
			os.Exit(1)
			return nil
		}
	}
	// XXX: use a serialize that corresponds with the actual scheme being used
	serializer := serializerjson.NewSerializer(serializerjson.DefaultMetaFactory, clientScheme, clientScheme, false)
	return &fakeClient{
		tracker: tracker,
		scheme:  clientScheme,
		serializer: serializer,
	}
}

func (c *fakeClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	o, err := c.tracker.Get(gvr, key.Namespace, key.Name)
	if err != nil {
		return err
	}
	j, err := json.Marshal(o)
	if err != nil {
		return err
	}
	_, _, err = c.serializer.Decode(j, nil, obj)
	return err
}

func (c *fakeClient) List(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
	gvk, err := getGVKFromList(list, c.scheme)
	if err != nil {
		// The old fake client required GVK info in Raw.TypeMeta, so check there
		// before giving up
		if opts.Raw == nil || opts.Raw.TypeMeta.APIVersion == "" || opts.Raw.TypeMeta.Kind == "" {
			return err
		}
		gvk = opts.Raw.TypeMeta.GroupVersionKind()
	}

	listGVK := schema.GroupVersionKind{
		Kind:    gvk.Kind + "List",
		Group:   gvk.Group,
		Version: gvk.Version,
	}
	ns := ""
	if opts != nil {
		ns = opts.Namespace
	}

	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	o, err := c.tracker.List(gvr, gvk, ns)
	if err != nil {
		return err
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

	if opts != nil && opts.LabelSelector != nil {
		return filterListItems(list, opts.LabelSelector)
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

func (c *fakeClient) Create(ctx context.Context, obj runtime.Object) error {
	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	return c.tracker.Create(gvr, obj, accessor.GetNamespace())
}

func (c *fakeClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOptionFunc) error {
	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	//TODO: implement propagation
	return c.tracker.Delete(gvr, accessor.GetNamespace(), accessor.GetName())
}

func (c *fakeClient) Update(ctx context.Context, obj runtime.Object) error {
	gvr, err := getGVRFromObject(obj, c.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	return c.tracker.Update(gvr, obj, accessor.GetNamespace())
}

func (c *fakeClient) Status() client.StatusWriter {
	return &fakeStatusWriter{client: c}
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
		return schema.GroupVersionKind{}, fmt.Errorf("non-list type %T (kind %q) passed as output", list, gvk)
	}
	// we need the non-list GVK, so chop off the "List" from the end of the kind
	gvk.Kind = gvk.Kind[:len(gvk.Kind)-4]
	return gvk, nil
}

type fakeStatusWriter struct {
	client *fakeClient
}

func (sw *fakeStatusWriter) Update(ctx context.Context, obj runtime.Object) error {
	// TODO(droot): This results in full update of the obj (spec + status). Need
	// a way to update status field only.
	return sw.client.Update(ctx, obj)
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
