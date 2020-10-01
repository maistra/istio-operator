package v2

import v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"

// `JaegerAddonConfig` configuration specific to Jaeger integration.
type JaegerAddonConfig struct {
	// Name of Jaeger custom resource, Namespace must match control plane namespace
	Name string `json:"name,omitempty"`
	// `Install` configures a Jaeger installation, which will be created if the
	// named Jaeger resource is not present.  If null, the named Jaeger resource
	// must exist.
	// +optional
	Install *JaegerInstallConfig `json:"install,omitempty"`
}

// `JaegerInstallConfig` configures a Jaeger installation.
type JaegerInstallConfig struct {
	// `Config` represents the configuration of Jaeger behavior.
	// +optional
	Storage *JaegerStorageConfig `json:"storage,omitempty"`
	// `Ingress` configures k8s Ingress or OpenShift Route for Jaeger services
	// .Values.tracing.jaeger.ingress.enabled, false if null
	// +optional
	Ingress *JaegerIngressConfig `json:"ingress,omitempty"`
}

// `JaegerStorageConfig` configures the storage used by the Jaeger installation.
type JaegerStorageConfig struct {
	// Type of storage to use
	Type JaegerStorageType `json:"type,omitempty"`
	// `Memory` represents configuration of in-memory storage
	// implies `.Values.tracing.jaeger.template`=`all-in-one`
	// +optional
	Memory *JaegerMemoryStorageConfig `json:"memory,omitempty"`
	// `Elasticsearch` represents configuration of elasticsearch storage
	// implies .Values.tracing.jaeger.template=production-elasticsearch
	// +optional
	Elasticsearch *JaegerElasticsearchStorageConfig `json:"elasticsearch,omitempty"`
}

// `JaegerStorageType` represents the type of storage configured for Jaeger
type JaegerStorageType string

const (
	// `JaegerStorageTypeMemory` represents in-memory storage
	JaegerStorageTypeMemory JaegerStorageType = "Memory"
	// `JaegerStorageTypeElasticsearch` represents Elasticsearch storage
	JaegerStorageTypeElasticsearch JaegerStorageType = "Elasticsearch"
)

// `JaegerMemoryStorageConfig` configures in-memory storage parameters for Jaeger
type JaegerMemoryStorageConfig struct {
	// `MaxTraces` configures the maximum number of traces to store. Defaults to 100000.
	// `.Values.tracing.jaeger.memory.max_traces`
	// +optional
	MaxTraces *int64 `json:"maxTraces,omitempty"`
}

// `JaegerElasticsearchStorageConfig` configures elasticsearch storage parameters for Jaeger
type JaegerElasticsearchStorageConfig struct {
	// `NodeCount` represents the number of elasticsearch nodes to create. Defaults to 3.
	// `.Values.tracing.jaeger.elasticsearch.nodeCount`
	// +optional
	NodeCount *int32 `json:"nodeCount,omitempty"`
	// Storage represents storage configuration for elasticsearch.
	// `.Values.tracing.jaeger.elasticsearch.storage`, raw yaml
	// +optional
	Storage *v1.HelmValues `json:"storage,omitempty"`
	// `RedundancyPolicy` configures the redundancy policy for elasticsearch
	// `.Values.tracing.jaeger.elasticsearch.redundancyPolicy`, raw YAML
	// +optional
	RedundancyPolicy string `json:"redundancyPolicy,omitempty"`
	// `IndexCleaner` represents the configuration for the elasticsearch index cleaner
	// `.Values.tracing.jaeger.elasticsearch.esIndexCleaner`, raw yaml
	// +optional
	IndexCleaner *v1.HelmValues `json:"indexCleaner,omitempty"`
}

// `JaegerIngressConfig` configures Kubernetes Ingress or OpenShift Route for exposing
// Jaeger services.
type JaegerIngressConfig struct {
	Enablement `json:",inline"`
	// `Metadata` represents addtional annotations/labels to be applied to the ingress/route.
	// +optional
	Metadata *MetadataConfig `json:"metadata,omitempty"`
}

// `ResourceName` returns the resource name for the Jaeger resource. Defaults to "jaeger"
func (c JaegerAddonConfig) ResourceName() string {
	if c.Name == "" {
		return "jaeger"
	}
	return c.Name
}
