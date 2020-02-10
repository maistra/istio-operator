package hacks

import (
	"context"
	"time"

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
