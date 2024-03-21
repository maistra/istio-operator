// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import (
	"context"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/common"
	pkgerrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"istio.io/istio/pkg/util/sets"
)

const conflictRequeueDelay = 2 * time.Second

func HasFinalizer(obj client.Object) bool {
	for _, finalizer := range obj.GetFinalizers() {
		if finalizer == common.FinalizerName {
			return true
		}
	}
	return false
}

func RemoveFinalizer(ctx context.Context, obj client.Object, cl client.Client) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Removing finalizer")

	finalizers := sets.New(obj.GetFinalizers()...)
	finalizers.Delete(common.FinalizerName)
	obj.SetFinalizers(finalizers.UnsortedList())

	err := cl.Update(ctx, obj)
	if errors.IsNotFound(err) {
		log.Info("Resource no longer exists; nothing to do")
		return ctrl.Result{}, nil
	} else if errors.IsConflict(err) {
		log.Info("Conflict while removing finalizer; Requeuing reconciliation")
		return ctrl.Result{RequeueAfter: conflictRequeueDelay}, nil
	} else if err != nil {
		return ctrl.Result{}, pkgerrors.Wrapf(err, "could not remove finalizer from %s/%s", obj.GetNamespace(), obj.GetName())
	}

	log.Info("Finalizer removed")
	return ctrl.Result{}, nil
}

func AddFinalizer(ctx context.Context, obj client.Object, cl client.Client) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Adding finalizer")

	finalizers := sets.New(obj.GetFinalizers()...)
	finalizers.Insert(common.FinalizerName)
	obj.SetFinalizers(finalizers.UnsortedList())

	err := cl.Update(ctx, obj)
	if errors.IsNotFound(err) {
		log.Info("Resource no longer exists; nothing to do")
		return ctrl.Result{}, nil
	} else if errors.IsConflict(err) {
		log.Info("Conflict while adding finalizer; Requeuing reconciliation")
		return ctrl.Result{RequeueAfter: conflictRequeueDelay}, nil
	} else if err != nil {
		return ctrl.Result{}, pkgerrors.Wrapf(err, "Could not add finalizer to %s/%s", obj.GetNamespace(), obj.GetName())
	}

	log.Info("Finalizer added")
	return ctrl.Result{}, nil
}
