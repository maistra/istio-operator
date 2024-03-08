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

package requestmappers

import (
	"context"

	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func MapOwnerToReconcileRequest(ctx context.Context, obj client.Object, ownerKind, ownerAPIGroup string) []reconcile.Request {
	log := logf.FromContext(ctx)

	var requests []reconcile.Request

	for _, ref := range obj.GetOwnerReferences() {
		refGV, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			log.Error(err, "Could not parse OwnerReference APIVersion", "api version", ref.APIVersion)
			continue
		}

		if ref.Kind == ownerKind && refGV.Group == ownerAPIGroup {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ref.Name}})
		}
	}

	annotations := obj.GetAnnotations()
	namespacedName, kind, apiGroup := helm.GetOwnerFromAnnotations(annotations)
	if namespacedName != nil && kind == ownerKind && apiGroup == ownerAPIGroup {
		requests = append(requests, reconcile.Request{NamespacedName: *namespacedName})
	}

	return requests
}
