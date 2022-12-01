package member

import (
	"testing"
	"time"

	network "github.com/openshift/api/network/v1"
	"github.com/openshift/library-go/pkg/network/networkapihelpers"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/controllers/common/test"
	"github.com/maistra/istio-operator/controllers/common/test/assert"
)

// fast backoff to prevent test from running too long
var fastBackoff = wait.Backoff{
	Steps:    15,
	Duration: 50 * time.Millisecond,
	Factor:   1.1,
}

func TestOVSMultitenantReconcileNamespaceInMesh(t *testing.T) {
	var meshNetID uint32 = 1

	testCases := []struct {
		name                          string
		memberNetID                   uint32
		pendingAction                 networkapihelpers.PodNetworkAction
		pendingNamespace              string
		runFakeNetNamespaceController bool
		expectSuccess                 bool
		expectAction                  bool
	}{
		{
			name:                          "success",
			memberNetID:                   2,
			runFakeNetNamespaceController: true,
			expectSuccess:                 true,
			expectAction:                  true,
		},
		{
			name:          "already joined",
			memberNetID:   meshNetID,
			expectSuccess: true,
			expectAction:  false,
		},
		{
			name:                          "joined to another pod network",
			memberNetID:                   3,
			runFakeNetNamespaceController: true,
			expectSuccess:                 true,
			expectAction:                  true,
		},
		{
			name:             "join pending",
			memberNetID:      2,
			pendingAction:    networkapihelpers.JoinPodNetwork,
			pendingNamespace: controlPlaneNamespace,
			expectSuccess:    true,
			expectAction:     false,
		},
		{
			name:                          "timeout",
			memberNetID:                   2,
			runFakeNetNamespaceController: false,
			expectSuccess:                 false,
			expectAction:                  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			meshNetns := newNetNamespace(controlPlaneNamespace, meshNetID)
			netns := newNetNamespace(appNamespace, tc.memberNetID)
			if tc.pendingAction != networkapihelpers.PodNetworkAction("") {
				networkapihelpers.SetChangePodNetworkAnnotation(netns, tc.pendingAction, tc.pendingNamespace)
			}
			cl, tracker := test.CreateClient(meshNetns, netns)

			fakeNetNamespaceController := fakeNetNamespaceController{}
			if tc.runFakeNetNamespaceController {
				go fakeNetNamespaceController.run(cl, t)
			}

			strategy := createAndConfigureMultitenantStrategy(cl)
			if tc.expectSuccess {
				assert.Success(strategy.reconcileNamespaceInMesh(ctx, appNamespace), "reconcileNamespaceInMesh", t)
			} else {
				assert.Failure(strategy.reconcileNamespaceInMesh(ctx, appNamespace), "reconcileNamespaceInMesh", t)
			}

			if tc.expectAction {
				if tc.runFakeNetNamespaceController {
					test.AssertNumberOfWriteActions(t, tracker.Actions(), 2) // 2 = annotation added by strategy + removed by fake controller
					assert.Equals(fakeNetNamespaceController.action, networkapihelpers.JoinPodNetwork, "Unexpected action in NetNamespace annotation", t)
					assert.Equals(fakeNetNamespaceController.namespace, controlPlaneNamespace, "Unexpected namespace in NetNamespace annotation", t)
				} else {
					test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)
				}
			} else {
				test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
			}
		})
	}
}

func TestOVSMultitenantRemoveNamespaceFromMesh(t *testing.T) {
	var meshNetID uint32 = 1

	testCases := []struct {
		name                          string
		memberNetID                   uint32
		pendingAction                 networkapihelpers.PodNetworkAction
		pendingNamespace              string
		runFakeNetNamespaceController bool
		expectSuccess                 bool
		expectAction                  bool
	}{
		{
			name:                          "success",
			memberNetID:                   meshNetID,
			runFakeNetNamespaceController: true,
			expectSuccess:                 true,
			expectAction:                  true,
		},
		{
			name:          "already isolated",
			memberNetID:   2,
			expectSuccess: true,
			expectAction:  false,
		},
		{
			name:          "isolation pending",
			memberNetID:   meshNetID,
			pendingAction: networkapihelpers.IsolatePodNetwork,
			expectSuccess: true,
			expectAction:  false,
		},
		{
			name:                          "timeout",
			memberNetID:                   meshNetID,
			runFakeNetNamespaceController: false,
			expectSuccess:                 false,
			expectAction:                  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			meshNetns := newNetNamespace(controlPlaneNamespace, meshNetID)
			netns := newNetNamespace(appNamespace, tc.memberNetID)
			if tc.pendingAction != networkapihelpers.PodNetworkAction("") {
				networkapihelpers.SetChangePodNetworkAnnotation(netns, tc.pendingAction, tc.pendingNamespace)
			}
			cl, tracker := test.CreateClient(meshNetns, netns)

			fakeNetNamespaceController := fakeNetNamespaceController{}
			if tc.runFakeNetNamespaceController {
				go fakeNetNamespaceController.run(cl, t)
			}

			strategy := createAndConfigureMultitenantStrategy(cl)
			if tc.expectSuccess {
				assert.Success(strategy.removeNamespaceFromMesh(ctx, appNamespace), "removeNamespaceFromMesh", t)
			} else {
				assert.Failure(strategy.removeNamespaceFromMesh(ctx, appNamespace), "removeNamespaceFromMesh", t)
			}

			if tc.expectAction {
				if tc.runFakeNetNamespaceController {
					test.AssertNumberOfWriteActions(t, tracker.Actions(), 2) // 2 = annotation added by strategy + removed by fake controller
					assert.Equals(fakeNetNamespaceController.action, networkapihelpers.IsolatePodNetwork, "Unexpected action in NetNamespace annotation", t)
				} else {
					test.AssertNumberOfWriteActions(t, tracker.Actions(), 1)
				}
			} else {
				test.AssertNumberOfWriteActions(t, tracker.Actions(), 0)
			}
		})
	}
}

func createAndConfigureMultitenantStrategy(cl client.Client) *multitenantStrategy {
	// override the backoff in ovs_multitenant.go to speed up tests
	netNamespaceCheckBackOff = fastBackoff
	return newMultitenantStrategy(cl, controlPlaneNamespace)
}

func newNetNamespace(name string, netID uint32) *network.NetNamespace {
	netns := &network.NetNamespace{
		ObjectMeta: meta.ObjectMeta{
			Name: name,
		},
		NetID: netID,
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
		err := cl.Get(ctx, types.NamespacedName{Name: appNamespace}, netns)
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

		err = cl.Update(ctx, netns)
		if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		t.Errorf("fakeNetNamespaceController never found valid annotation in NetNamespace")
	}
}
