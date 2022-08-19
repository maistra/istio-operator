package member

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	network "github.com/openshift/api/network/v1"
	"github.com/openshift/library-go/pkg/network/networkapihelpers"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

type multitenantStrategy struct {
	common.ControllerResources
	meshNamespace string
}

var _ NamespaceReconciler = (*multitenantStrategy)(nil)

var netNamespaceCheckBackOff = wait.Backoff{
	Steps:    15,
	Duration: 500 * time.Millisecond,
	Factor:   1.1,
}

func newMultitenantStrategy(cl client.Client, meshNamespace string) *multitenantStrategy {
	return &multitenantStrategy{
		ControllerResources: common.ControllerResources{
			Client: cl,
		},
		meshNamespace: meshNamespace,
	}
}

func (s *multitenantStrategy) reconcileNamespaceInMesh(ctx context.Context, namespace string) error {
	return s.updateNetworkNamespace(ctx, namespace, networkapihelpers.JoinPodNetwork, s.meshNamespace)
}

func (s *multitenantStrategy) removeNamespaceFromMesh(ctx context.Context, namespace string) error {
	return s.updateNetworkNamespace(ctx, namespace, networkapihelpers.IsolatePodNetwork, "")
}

// adapted from github.com/openshift/oc/pkg/cli/admin/network/project_options.go#UpdatePodNetwork()
func (s *multitenantStrategy) updateNetworkNamespace(ctx context.Context, namespace string, action networkapihelpers.PodNetworkAction, args string) error {
	log := s.getLogger(ctx)

	netns := &network.NetNamespace{}
	err := s.Client.Get(ctx, client.ObjectKey{Name: namespace}, netns)
	if err != nil {
		return err
	}

	meshNetns := &network.NetNamespace{}
	err = s.Client.Get(ctx, client.ObjectKey{Name: s.meshNamespace}, meshNetns)
	if err != nil {
		return err
	}

	pendingAction, pendingNamespace, _ := networkapihelpers.GetChangePodNetworkAnnotation(netns)

	isJoinedToMesh := netns.NetID == meshNetns.NetID
	if action == networkapihelpers.JoinPodNetwork {
		// if the pod network is already joined to the mesh or pending to be, we don't need to do anything
		if isJoinedToMesh || (pendingAction == networkapihelpers.JoinPodNetwork && pendingNamespace == s.meshNamespace) {
			log.V(2).Info("Not joining pod network as it is already joined or pending")
			return nil
		}
		log.Info("Joining pod network to mesh")
	} else if action == networkapihelpers.IsolatePodNetwork {
		// if the pod network is already isolated or pending to be, we don't need to do anything
		if !isJoinedToMesh || pendingAction == networkapihelpers.IsolatePodNetwork {
			log.V(2).Info("Not isolating pod network as it is already isolated or pending")
			return nil
		}
		log.Info("Isolating pod network")
	}

	networkapihelpers.SetChangePodNetworkAnnotation(netns, action, args)
	err = s.Client.Update(ctx, netns)
	if err != nil {
		return err
	}
	// Validate SDN controller applied or rejected the intent
	return wait.ExponentialBackoff(netNamespaceCheckBackOff, func() (bool, error) {
		updatedNetNs := &network.NetNamespace{}
		err := s.Client.Get(ctx, client.ObjectKey{Name: namespace}, updatedNetNs)
		if err != nil {
			return false, err
		}

		if _, _, err = networkapihelpers.GetChangePodNetworkAnnotation(updatedNetNs); err == networkapihelpers.ErrorPodNetworkAnnotationNotFound {
			return true, nil
		}
		// Pod network change not applied yet
		return false, nil
	})
}

func (s *multitenantStrategy) getLogger(ctx context.Context) logr.Logger {
	return common.LogFromContext(ctx).WithValues("NetworkStrategy", "Multitenant")
}
