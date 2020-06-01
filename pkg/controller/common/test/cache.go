package test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/testing"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// FakeCache is a mock implementation of a cache.Cache which is backed by a
// testing.ObjectTracker.
type FakeCache struct {
	client    client.Client
	scheme    *runtime.Scheme
	tracker   testing.ObjectTracker
	informers map[schema.GroupVersionKind]toolscache.SharedIndexInformer
	stop      <-chan struct{}
	resync    time.Duration
	mu        sync.RWMutex
	started   bool
	namespace string
}

var _ cache.Cache = (*FakeCache)(nil)

// NewCache returns a new FakeCache backed by the specified tracker.
func NewCache(opts cache.Options, tracker testing.ObjectTracker) (cache.Cache, error) {
	// client is used to simplify implemetation of some of the methods.
	// as everything is backed by the tracker, this shouldn't be an issue.
	cache := &FakeCache{
		client:    NewFakeClientWithSchemeAndTracker(opts.Scheme, tracker),
		scheme:    opts.Scheme,
		namespace: opts.Namespace,
		tracker:   tracker,
		informers: make(map[schema.GroupVersionKind]toolscache.SharedIndexInformer),
	}
	if opts.Resync != nil {
		cache.resync = *opts.Resync
	}
	return cache, nil
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
// obj must be a struct pointer so that obj can be updated with the response
// returned by the Server.
func (c *FakeCache) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return c.client.Get(ctx, key, obj)
}

// List retrieves list of objects for a given namespace and list options. On a
// successful call, Items field in the list will be populated with the
// result returned from the server.
func (c *FakeCache) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	return c.client.List(ctx, list, opts...)
}

// GetInformer fetches or constructs an informer for the given object that corresponds to a single
// API kind and resource.
func (c *FakeCache) GetInformer(ctx context.Context, obj runtime.Object) (cache.Informer, error) {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return nil, err
	}
	return c.internalGetInformer(gvk, obj)
}

// GetInformerForKind is similar to GetInformer, except that it takes a group-version-kind, instead
// of the underlying object.
func (c *FakeCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind) (cache.Informer, error) {
	obj, err := c.scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	return c.internalGetInformer(gvk, obj)
}

func (c *FakeCache) internalGetInformer(gvk schema.GroupVersionKind, obj runtime.Object) (toolscache.SharedIndexInformer, error) {
	i, ok := func() (toolscache.SharedIndexInformer, bool) {
		c.mu.RLock()
		defer c.mu.RUnlock()
		i, ok := c.informers[gvk]
		return i, ok
	}()
	if ok {
		return i, nil
	}

	var sync bool
	i, err := func() (toolscache.SharedIndexInformer, error) {
		c.mu.Lock()
		defer c.mu.Unlock()

		var ok bool
		i, ok := c.informers[gvk]
		if ok {
			return i, nil
		}

		var lw *toolscache.ListWatch
		lw, err := c.createListWatcher(gvk)
		if err != nil {
			return nil, err
		}
		i = toolscache.NewSharedIndexInformer(lw, obj, c.resync, toolscache.Indexers{
			toolscache.NamespaceIndex: toolscache.MetaNamespaceIndexFunc,
		})
		c.informers[gvk] = i

		if c.started {
			sync = true
			go i.Run(c.stop)
		}
		return i, nil
	}()
	if err != nil {
		return nil, err
	}

	if sync {
		// Wait for it to sync before returning the Informer so that folks don't read from a stale cache.
		if !toolscache.WaitForCacheSync(c.stop, i.HasSynced) {
			return nil, fmt.Errorf("failed waiting for %T Informer to sync", obj)
		}
	}

	return i, err
}

func (c *FakeCache) createListWatcher(gvk schema.GroupVersionKind) (*toolscache.ListWatch, error) {
	listGVK := gvk.GroupVersion().WithKind(gvk.Kind + "List")
	listObj, err := c.scheme.New(listGVK)
	if err != nil {
		return nil, err
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	return &toolscache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			res := listObj.DeepCopyObject()
			c.client.List(context.TODO(), res, &client.ListOptions{Namespace: c.namespace, Raw: &opts})
			return res, err
		},
		// Setup the watch function
		WatchFunc: func(_ metav1.ListOptions) (watch.Interface, error) {
			return c.tracker.Watch(gvr, c.namespace)
		},
	}, nil
}

// Start runs all the informers known to this cache until the given channel is closed.
// It blocks.
func (c *FakeCache) Start(stop <-chan struct{}) error {
	func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		// Set the stop channel so it can be passed to informers that are added later
		c.stop = stop

		// Start each informer
		for _, informer := range c.informers {
			go informer.Run(stop)
		}

		// Set started to true so we immediately start any informers added later.
		c.started = true
	}()
	<-stop
	return nil
}

// WaitForCacheSync waits for all the caches to sync.  Returns false if it could not sync a cache.
func (c *FakeCache) WaitForCacheSync(stop <-chan struct{}) bool {
	syncedFuncs := func() []toolscache.InformerSynced {
		c.mu.RLock()
		defer c.mu.RUnlock()
		syncedFuncs := make([]toolscache.InformerSynced, 0, len(c.informers))
		for _, informer := range c.informers {
			syncedFuncs = append(syncedFuncs, informer.HasSynced)
		}
		return syncedFuncs
	}()
	return toolscache.WaitForCacheSync(stop, syncedFuncs...)
}

// IndexField adds an index with the given field name on the given object type
// by using the given function to extract the value for that field.  If you want
// compatibility with the Kubernetes API server, only return one key, and only use
// fields that the API server supports.  Otherwise, you can return multiple keys,
// and "equality" in the field selector means that at least one key matches the value.
func (c *FakeCache) IndexField(ctx context.Context, obj runtime.Object, field string, extractValue client.IndexerFunc) error {
	informer, err := c.GetInformer(ctx, obj)
	if err != nil {
		return err
	}
	return indexByField(informer, field, extractValue)
}

// adapted from sigs.k8s.io/controller-runtime/pkg/cache/informer_cache.go
func indexByField(indexer cache.Informer, field string, extractor client.IndexerFunc) error {
	indexFunc := func(objRaw interface{}) ([]string, error) {
		// TODO(directxman12): check if this is the correct type?
		obj, isObj := objRaw.(runtime.Object)
		if !isObj {
			return nil, fmt.Errorf("object of type %T is not an Object", objRaw)
		}
		meta, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		ns := meta.GetNamespace()

		rawVals := extractor(obj)
		var vals []string
		if ns == "" {
			// if we're not doubling the keys for the namespaced case, just re-use what was returned to us
			vals = rawVals
		} else {
			// if we need to add non-namespaced versions too, double the length
			vals = make([]string, len(rawVals)*2)
		}
		for i, rawVal := range rawVals {
			// save a namespaced variant, so that we can ask
			// "what are all the object matching a given index *in a given namespace*"
			vals[i] = keyToNamespacedKey(ns, rawVal)
			if ns != "" {
				// if we have a namespace, also inject a special index key for listing
				// regardless of the object namespace
				vals[i+len(rawVals)] = keyToNamespacedKey("", rawVal)
			}
		}

		return vals, nil
	}

	return indexer.AddIndexers(toolscache.Indexers{fmt.Sprintf("field: %s", field): indexFunc})
}

// adapted from sigs.k8s.io/controller-runtime/pkg/cache/internal/cache_reader.go
func keyToNamespacedKey(ns string, baseKey string) string {
	return ns + "/" + baseKey
}
