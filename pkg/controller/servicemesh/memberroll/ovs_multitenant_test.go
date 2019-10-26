package memberroll

import (
	"context"
	"testing"
	"time"

	network "github.com/openshift/api/network/v1"
	"github.com/openshift/library-go/pkg/network/networkapihelpers"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/maistra/istio-operator/pkg/controller/common/test"
	"github.com/maistra/istio-operator/pkg/controller/common/test/assert"
)

// fast backoff to prevent test from running too long
var fastBackoff = wait.Backoff{
	Steps:    15,
	Duration: 50 * time.Millisecond,
	Factor:   1.1,
}

func TestReconcileNamespaceInMeshAddsJoinActionToNetNamespaceObject(t *testing.T) {
	netns := newNetNamespace(appNamespace)
	cl, _ := test.CreateClient(netns)

	fakeNetNamespaceController := fakeNetNamespaceController{}
	go fakeNetNamespaceController.run(cl, t)

	strategy := createAndConfigureMultitenantStrategy(cl, t)
	assert.Success(strategy.reconcileNamespaceInMesh(appNamespace), "reconcileNamespaceInMesh", t)

	assert.Equals(fakeNetNamespaceController.action, networkapihelpers.JoinPodNetwork, "Unexpected action in NetNamespace annotation", t)
	assert.Equals(fakeNetNamespaceController.namespace, controlPlaneNamespace, "Unexpected namespace in NetNamespace annotation", t)
}

func TestRemoveNamespaceFromMeshAddsIsolateActionToNetNamespaceObject(t *testing.T) {
	netns := newNetNamespace(appNamespace)
	cl, _ := test.CreateClient(netns)

	fakeNetNamespaceController := fakeNetNamespaceController{}
	go fakeNetNamespaceController.run(cl, t)

	strategy := createAndConfigureMultitenantStrategy(cl, t)
	assert.Success(strategy.removeNamespaceFromMesh(appNamespace), "removeNamespaceFromMesh", t)

	assert.Equals(fakeNetNamespaceController.action, networkapihelpers.IsolatePodNetwork, "Unexpected action in NetNamespace annotation", t)
}

func TestReconcileNamespaceInMeshFailsIfNetNamespaceControllerDoesntProcessNetNamespaceInTime(t *testing.T) {
	netns := newNetNamespace(appNamespace)
	cl, _ := test.CreateClient(netns)

	// NOTE: this test doesn't run any fake NetNamespace controller

	strategy := createAndConfigureMultitenantStrategy(cl, t)

	assert.Failure(strategy.reconcileNamespaceInMesh(appNamespace), "reconcileNamespaceInMesh", t)
}

func TestRemoveNamespaceFromMeshFailsIfNetNAmespaceControllerDoesntProcessNetNamespaceInTime(t *testing.T) {
	netns := newNetNamespace(appNamespace)
	cl, _ := test.CreateClient(netns)

	// NOTE: this test doesn't run any fake NetNamespace controller

	strategy := createAndConfigureMultitenantStrategy(cl, t)
	assert.Failure(strategy.removeNamespaceFromMesh(appNamespace), "removeNamespaceFromMesh", t)
}

func createAndConfigureMultitenantStrategy(cl client.Client, t *testing.T) *multitenantStrategy {
	// override the backoff in ovs_multitenant.go to speed up tests
	netNamespaceCheckBackOff = fastBackoff
	strategy, err := newMultitenantStrategy(cl, logf.Log, controlPlaneNamespace)
	if err != nil {
		t.Fatalf("Error creating network strategy: %v", err)
	}
	return strategy
}

func newNetNamespace(name string) *network.NetNamespace {
	netns := &network.NetNamespace{
		ObjectMeta: meta.ObjectMeta{
			Name: name,
		},
	}
	return netns
}

type fakeNetNamespaceController struct {
	action    networkapihelpers.PodNetworkAction
	namespace string
}

func (c *fakeNetNamespaceController) run(cl client.Client, t *testing.T) {
	err := wait.ExponentialBackoff(fastBackoff, func() (bool, error) {
		netns := &network.NetNamespace{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: appNamespace}, netns)
		if err != nil {
			return false, err
		}

		c.action, c.namespace, err = networkapihelpers.GetChangePodNetworkAnnotation(netns)
		if err != nil {
			if err == networkapihelpers.ErrorPodNetworkAnnotationNotFound {
				return false, nil
			}
			return false, err
		}

		networkapihelpers.DeleteChangePodNetworkAnnotation(netns)

		err = cl.Update(context.TODO(), netns)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("fakeNetNamespaceController never successfully removed annotation from NetNamespace")
	}
}
