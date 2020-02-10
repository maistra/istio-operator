package hacks

import (
	"context"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

// ReduceLikelihoodOfRepeatedReconciliation simply performs a 2 second delay. Call this function after you post an
// update to a resource if you want to reduce the likelihood of the reconcile() function being called again before
// the update comes back into the operator (until it does, any invocation of reconcile() will perform reconciliation on
// a stale version of the resource). Calling this function prevents the next reconcile() from being invoked immediately,
// allowing the watch event more time to come back and update the cache.
//
// For the complete explanation, see https://issues.jboss.org/projects/MAISTRA/issues/MAISTRA-830
func ReduceLikelihoodOfRepeatedReconciliation(ctx context.Context) {
	log := common.LogFromContext(ctx)
	log.Info("Waiting 2 seconds to give the cache a chance to sync after updating resource")
	time.Sleep(2 * time.Second)
}

// WorkAroundTypeObjectProblemInCRDSchemas works around the problem where OpenShift 3.11 doesn't like "type: object"
// in CRD OpenAPI schemas. This function removes all occurrences from the schema.
func WorkAroundTypeObjectProblemInCRDSchemas(ctx context.Context, err error, cl client.Client, crd *unstructured.Unstructured) error {
	log := common.LogFromContext(ctx)
	if err != nil && strings.Contains(err.Error(), "must only have \"properties\", \"required\" or \"description\" at the root if the status subresource is enabled") {
		log.Info("The API server rejected the CRD. Removing type:object fields from the CRD schema and trying again.")

		schema, found, err := unstructured.NestedFieldNoCopy(crd.UnstructuredContent(), "spec", "validation", "openAPIV3Schema")
		if err != nil {
			log.Error(err, "Could not remove type:object from CRD schema")
			return err
		} else if found {
			removeTypeObjectField(schema)
			return cl.Create(ctx, crd)
		}
	}
	return err
}

func removeTypeObjectField(val interface{}) {
	if m, isMap := val.(map[string]interface{}); isMap {
		if m["type"] == "object" {
			delete(m, "type")
		}
		for _, childVal := range m {
			removeTypeObjectField(childVal)
		}
	} else if a, isArrayOfMap := val.([]map[string]interface{}); isArrayOfMap {
		for i, _ := range a {
			removeTypeObjectField(a[i])
		}
	}
}
