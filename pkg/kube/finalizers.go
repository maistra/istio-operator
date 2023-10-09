package kube

import (
	"context"
	"fmt"

	pkgerrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"maistra.io/istio-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/util/sets"
)

func HasFinalizer(obj client.Object) bool {
	objectMeta := getObjectMeta(obj)
	finalizers := sets.New(objectMeta.GetFinalizers()...)
	return finalizers.Contains(common.FinalizerName)
}

func RemoveFinalizer(ctx context.Context, obj client.Object, cl client.Client) error {
	reqLogger := common.LogFromContext(ctx)
	reqLogger.Info(fmt.Sprintf("Removing finalizer from %s", obj.GetObjectKind().GroupVersionKind().Kind))

	objectMeta := getObjectMeta(obj)
	finalizers := sets.New(objectMeta.GetFinalizers()...)
	finalizers.Delete(common.FinalizerName)
	objectMeta.SetFinalizers(finalizers.UnsortedList())

	err := cl.Update(ctx, obj)
	if errors.IsNotFound(err) {
		// We're reconciling a stale instance. The object no longer exists, so we're done.
		return nil
	} else if err != nil {
		return pkgerrors.Wrapf(err, "Could not remove finalizer from %s/%s", objectMeta.GetNamespace(), objectMeta.GetName())
	}
	return nil
}

func AddFinalizer(ctx context.Context, obj client.Object, cl client.Client) error {
	reqLogger := common.LogFromContext(ctx)
	reqLogger.Info(fmt.Sprintf("Adding finalizer to %s", obj.GetObjectKind().GroupVersionKind().Kind))

	objectMeta := getObjectMeta(obj)
	finalizers := sets.New(objectMeta.GetFinalizers()...)
	finalizers.Insert(common.FinalizerName)
	objectMeta.SetFinalizers(finalizers.UnsortedList())

	err := cl.Update(ctx, obj)
	if errors.IsNotFound(err) {
		// Object was deleted manually before we could add the finalizer to it. This is not an error.
		return nil
	} else if err != nil {
		return pkgerrors.Wrapf(err, "Could not add finalizer to %s/%s", objectMeta.GetNamespace(), objectMeta.GetName())
	}
	return nil
}

func getObjectMeta(obj client.Object) meta.Object {
	oma, ok := obj.(meta.ObjectMetaAccessor)
	if !ok {
		panic("object does not implement ObjectMetaAccessor")
	}
	return oma.GetObjectMeta()
}
