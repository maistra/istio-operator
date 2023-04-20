package v2

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// MeshConfig TODO: add description
type MeshConfig struct {
	// ExtensionProviders defines a list of extension providers that extend Istio's functionality. For example,
	// the AuthorizationPolicy can be used with an extension provider to delegate the authorization decision
	// to a custom authorization system.
	ExtensionProviders []*ExtensionProviderConfig `json:"extensionProviders,omitempty"`
	// A list of Kubernetes selectors that specify the set of namespaces that Istio considers when
	// computing configuration updates for sidecars. This can be used to reduce Istio's computational load
	// by limiting the number of entities (including services, pods, and endpoints) that are watched and processed.
	// If omitted, Istio will use the default behavior of processing all namespaces in the cluster.
	// Elements in the list are disjunctive (OR semantics), i.e. a namespace will be included if it matches any selector.
	// The following example selects any namespace that matches either below:
	// 1. The namespace has both of these labels: `env: prod` and `region: us-east1`
	// 2. The namespace has label `app` equal to `cassandra` or `spark`.
	// ```yaml
	// discoverySelectors:
	//   - matchLabels:
	//       env: prod
	//       region: us-east1
	//   - matchExpressions:
	//     - key: app
	//       operator: In
	//       values:
	//         - cassandra
	//         - spark
	// ```
	// Refer to the [kubernetes selector docs](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors)
	// for additional detail on selector semantics.
	DiscoverySelectors []*v1.LabelSelector `json:"discoverySelectors,omitempty"`
}
