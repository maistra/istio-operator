package common

import "k8s.io/client-go/discovery"

type DiscoveryClientProvider interface {
	GetDiscoveryClient() (discovery.DiscoveryInterface, error)
}
