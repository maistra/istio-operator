package istiorevision

import (
	"context"
	"fmt"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// This file is a copy of sigs.k8s.io/controller-runtime/pkg/handler/enqueue_mapped.go, but with added logging

func EnqueueRequestsFromMapFunc(fn handler.MapFunc) handler.EventHandler {
	return &enqueueRequestsFromMapFunc{
		toRequests: fn,
	}
}

type empty struct{}

var _ handler.EventHandler = &enqueueRequestsFromMapFunc{}

type enqueueRequestsFromMapFunc struct {
	// Mapper transforms the argument into a slice of keys to be reconciled
	toRequests handler.MapFunc
}

// Create implements EventHandler.
func (e *enqueueRequestsFromMapFunc) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]empty{}
	e.mapAndEnqueue(ctx, q, evt.Object, reqs)
}

// Update implements EventHandler.
func (e *enqueueRequestsFromMapFunc) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]empty{}
	e.mapAndEnqueue(ctx, q, evt.ObjectOld, reqs)
	e.mapAndEnqueue(ctx, q, evt.ObjectNew, reqs)
}

// Delete implements EventHandler.
func (e *enqueueRequestsFromMapFunc) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]empty{}
	e.mapAndEnqueue(ctx, q, evt.Object, reqs)
}

// Generic implements EventHandler.
func (e *enqueueRequestsFromMapFunc) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]empty{}
	e.mapAndEnqueue(ctx, q, evt.Object, reqs)
}

func (e *enqueueRequestsFromMapFunc) mapAndEnqueue(ctx context.Context, q workqueue.RateLimitingInterface, object client.Object, reqs map[reconcile.Request]empty) {
	log := logf.FromContext(ctx).WithName("ctrlr").WithName("istiorev")
	for _, req := range e.toRequests(ctx, object) {
		_, ok := reqs[req]
		if !ok {
			q.Add(req)
			reqs[req] = empty{}
			log.V(2).Info("Enqueueing IstioRevision due to object update",
				"kind", object.GetObjectKind().GroupVersionKind().Kind,
				"object", fmt.Sprintf("%s/%s", object.GetNamespace(), object.GetName()),
				"IstioRevision", req.Name)
		}
	}
}
