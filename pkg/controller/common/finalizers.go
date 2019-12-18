package common

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
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

type FinalizerFunc func(runtime.Object, logr.Logger) (mayContinue bool, err error)

func HandleFinalization(finalizerFunc FinalizerFunc, obj runtime.Object, cl client.Client, eventRecorder record.EventRecorder, reqLogger logr.Logger) (continueReconciliation bool, err error) {
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

		continueReconciliation, err = finalizerFunc(obj, reqLogger)
		if err != nil || !continueReconciliation {
			return continueReconciliation, err
		}

		reqLogger.Info(fmt.Sprintf("Removing finalizer from %s", obj.GetObjectKind().GroupVersionKind().Kind))
		finalizers.Delete(FinalizerName)
		objectMeta.SetFinalizers(finalizers.List())
		err = cl.Update(context.TODO(), obj)
		if err != nil {
			if errors.IsNotFound(err) || errors.IsConflict(err) {
				// We're reconciling a stale instance. If the object no longer exists, we're done. If there was a
				// conflict, we'll receive another watch event, which will trigger another reconciliation. We'll remove
				// the finalizer then.
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
		err = cl.Update(context.TODO(), obj)
		if err != nil {
			if errors.IsNotFound(err) {
				// Object was deleted manually before we could add the finalizer to it. This is not an error.
				return false, nil
			} else if errors.IsConflict(err) {
				// Object was created and immediately updated, before the controller was able to add the finalizer.
				// The instance we're reconciling is stale, hence the Conflict error. When the update watch event
				// arrives, another reconciliation will be triggered, which means we don't need to do anything here.
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}
