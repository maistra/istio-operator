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

	"github.com/istio-ecosystem/sail-operator/pkg/common"
	pkgerrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"istio.io/istio/pkg/util/sets"
)

func HasFinalizer(obj client.Object) bool {
	objectMeta := getObjectMeta(obj)
	finalizers := sets.New(objectMeta.GetFinalizers()...)
	return finalizers.Contains(common.FinalizerName)
}

func RemoveFinalizer(ctx context.Context, obj client.Object, cl client.Client) error {
	log := logf.FromContext(ctx)
	log.Info("Removing finalizer")

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
	log := logf.FromContext(ctx)
	log.Info("Adding finalizer")

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
