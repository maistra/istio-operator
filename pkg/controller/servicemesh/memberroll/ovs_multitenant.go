package memberroll

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

func newMultitenantStrategy(cl client.Client, baseLogger logr.Logger, meshNamespace string) (*multitenantStrategy, error) {
	return &multitenantStrategy{
		ControllerResources: common.ControllerResources{
			Client: cl,
			Log:    baseLogger.WithValues("NetworkStrategy", "Multitenant"),
		},
		meshNamespace: meshNamespace,
	}, nil
}

func (s *multitenantStrategy) reconcileNamespaceInMesh(namespace string) error {
	s.Log.Info("joining network to mesh", "Namespace", namespace)
	return s.updateNetworkNamespace(namespace, networkapihelpers.JoinPodNetwork, s.meshNamespace)
}

func (s *multitenantStrategy) removeNamespaceFromMesh(namespace string) error {
	s.Log.Info("isolating network", "Namespace", namespace)
	return s.updateNetworkNamespace(namespace, networkapihelpers.IsolatePodNetwork, "")
}

// adapted from github.com/openshift/oc/pkg/cli/admin/network/project_options.go#UpdatePodNetwork()
func (s *multitenantStrategy) updateNetworkNamespace(namespace string, action networkapihelpers.PodNetworkAction, args string) error {
	netns := &network.NetNamespace{}
	err := s.Client.Get(context.TODO(), client.ObjectKey{Name: namespace}, netns)
	if err != nil {
		return err
	}
	networkapihelpers.SetChangePodNetworkAnnotation(netns, action, args)
	err = s.Client.Update(context.TODO(), netns)
	if err != nil {
		return err
	}
	// Validate SDN controller applied or rejected the intent
	return wait.ExponentialBackoff(netNamespaceCheckBackOff, func() (bool, error) {
		updatedNetNs := &network.NetNamespace{}
		err := s.Client.Get(context.TODO(), client.ObjectKey{Name: namespace}, updatedNetNs)
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
