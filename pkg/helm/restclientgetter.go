package helm

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// restClientGetter is required by helm to instantiate ActionConfig
type restClientGetter struct {
	config *rest.Config
}

func newRESTClientGetter(config *rest.Config) *restClientGetter {
	return &restClientGetter{
		config: config,
	}
}

func (c *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.config, nil
}

func (c *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	oldBurst := c.config.Burst
	// use the default (high) burst for discovery
	c.config.Burst = 0
	// write back the old burst value after it has been copied for discovery client creation
	defer func() { c.config.Burst = oldBurst }()

	discoveryClient, _ := discovery.NewDiscoveryClientForConfig(c.config)
	return memory.NewMemCacheClient(discoveryClient), nil
}

func (c *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	// we know err is always nil
	discoveryClient, _ := c.ToDiscoveryClient()

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

func (c *restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// use the standard defaults for this client command
	// DEPRECATED: remove and replace with something more accurate
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}
