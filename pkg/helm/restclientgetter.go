// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helm

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// restClientGetter is required by helm to instantiate ActionConfig
type restClientGetter struct {
	config          *rest.Config
	discoveryClient discovery.CachedDiscoveryInterface
	restMapper      meta.RESTMapper
}

func NewRESTClientGetter(config *rest.Config) genericclioptions.RESTClientGetter {
	return &restClientGetter{
		config: config,
	}
}

func (c *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.config, nil
}

func (c *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	if c.discoveryClient == nil {
		oldBurst := c.config.Burst
		// use the default (high) burst for discovery
		c.config.Burst = 0
		// write back the old burst value after it has been copied for discovery client creation
		defer func() { c.config.Burst = oldBurst }()

		discoveryClient, _ := discovery.NewDiscoveryClientForConfig(c.config)
		c.discoveryClient = memory.NewMemCacheClient(discoveryClient)
	}
	return c.discoveryClient, nil
}

func (c *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	if c.restMapper == nil {
		// we know err is always nil
		discoveryClient, _ := c.ToDiscoveryClient()

		mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
		c.restMapper = restmapper.NewShortcutExpander(mapper, discoveryClient, nil)
	}
	return c.restMapper, nil
}

func (c *restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// use the standard defaults for this client command
	// DEPRECATED: remove and replace with something more accurate
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}
