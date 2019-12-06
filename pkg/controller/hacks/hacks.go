package hacks

import (
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("hack")

// ReduceLikelihoodOfRepeatedReconciliation simply performs a 2 second delay. Call this function after you post an
// update to a resource if you want to reduce the likelihood of the reconcile() function being called again before
// the update comes back into the operator (until it does, any invocation of reconcile() will perform reconciliation on
// a stale version of the resource). Calling this function prevents the next reconcile() from being invoked immediately,
// allowing the watch event more time to come back and update the cache.
//
// For the complete explanation, see https://issues.jboss.org/projects/MAISTRA/issues/MAISTRA-830
func ReduceLikelihoodOfRepeatedReconciliation() {
	log.Info("Waiting 2 seconds to give the cache a chance to sync after updating resource")
	time.Sleep(2 * time.Second)
}

// ReduceLikelihoodOfReconcilingStaleObjectAfterStatusUpdate should be called whenever you update the status
// and return an error from the Reconcile function. If you update the status and return an error, the function
// gets called almost immediately, before the status update updates the local cache. The second Reconcile will
// do the right thing, since the object's spec hasn't changed, but when it tries to do another status update,
// the update will fail, because the object is stale due to the previous status update. This means the Reconcile
// function needs to return an error again, which means the cycle will repeat until the first status update
// makes it into the local cache. ReduceLikelihoodOfReconcilingStaleObjectAfterStatusUpdate tries to reduce
// the probability of that happening by simply waiting 2 seconds to give the cache a chance to sync.
func ReduceLikelihoodOfReconcilingStaleObjectAfterStatusUpdate() {
	log.Info("Waiting 2 seconds to give the cache a chance to sync after updating resource")
	time.Sleep(2 * time.Second)
}
