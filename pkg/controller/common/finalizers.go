package common

import (
	"context"
	"fmt"

	errors2 "github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const eventReasonFailedFinalizerRemoval = "FailedFinalizerRemoval"

type FinalizerFunc func(context.Context, runtime.Object) (mayContinue bool, err error)

func HandleFinalization(ctx context.Context, obj runtime.Object, finalizerFunc FinalizerFunc, cl client.Client, eventRecorder record.EventRecorder) (continueReconciliation bool, err error) {
	reqLogger := LogFromContext(ctx)

	oma, ok := obj.(meta.ObjectMetaAccessor)
	if !ok {
		panic("object does not implement ObjectMetaAccessor")
	}
	objectMeta := oma.GetObjectMeta()
	finalizers := sets.NewString(objectMeta.GetFinalizers()...)
	deleted := objectMeta.GetDeletionTimestamp() != nil
	if deleted {
		if !finalizers.Has(FinalizerName) {
			reqLogger.Info("Ignoring deleted object with no finalizer")
			return false, nil
		}

		continueReconciliation, err = finalizerFunc(ctx, obj)
		if err != nil || !continueReconciliation {
			return continueReconciliation, err
		}

		reqLogger.Info(fmt.Sprintf("Removing finalizer from %s", obj.GetObjectKind().GroupVersionKind().Kind))
		finalizers.Delete(FinalizerName)
		objectMeta.SetFinalizers(finalizers.List())
		err = cl.Update(ctx, obj)
		if err != nil {
			if errors.IsNotFound(err) {
				// We're reconciling a stale instance. The object no longer exists, so we're done.
				return false, nil
			}
			err = errors2.Wrapf(err, "Could not remove finalizer from %s/%s", objectMeta.GetNamespace(), objectMeta.GetName())
			eventRecorder.Event(obj, core.EventTypeWarning, eventReasonFailedFinalizerRemoval, err.Error())
			return false, err
		}

		return false, nil

	} else if !finalizers.Has(FinalizerName) {
		reqLogger.Info(fmt.Sprintf("Adding finalizer to %s", obj.GetObjectKind().GroupVersionKind().Kind))
		finalizers.Insert(FinalizerName)
		objectMeta.SetFinalizers(finalizers.List())
		err = cl.Update(ctx, obj)
		if errors.IsNotFound(err) {
			// Object was deleted manually before we could add the finalizer to it. This is not an error.
			return false, nil
		}
		return false, err
	}
	return true, nil
}
