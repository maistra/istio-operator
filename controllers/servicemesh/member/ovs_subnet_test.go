package member

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/controllers/common/test"
	"github.com/maistra/istio-operator/controllers/common/test/assert"
)

func TestSubnetReconcileNamespaceInMeshDoesNothing(t *testing.T) {
	cl, tracker := test.CreateClient()

	strategy := createAndConfigureSubnetStrategy(cl, t)
	assert.Success(strategy.reconcileNamespaceInMesh(ctx, appNamespace), "reconcileNamespaceInMesh", t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func TestSubnetRemoveNamespaceFromMeshDoesNothing(t *testing.T) {
	cl, tracker := test.CreateClient()

	strategy := createAndConfigureSubnetStrategy(cl, t)
	assert.Success(strategy.removeNamespaceFromMesh(ctx, appNamespace), "removeNamespaceFromMesh", t)
	test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
}

func createAndConfigureSubnetStrategy(cl client.Client, t *testing.T) *subnetStrategy {
	strategy := &subnetStrategy{}
	return strategy
}
