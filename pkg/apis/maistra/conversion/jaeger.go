package conversion

import (
	"fmt"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateJaegerAddonValues(jaeger *v2.JaegerAddonConfig, values map[string]interface{}, ns string) (reterr error) {
	if jaeger == nil {
		return nil
	}

	tracingValues := make(map[string]interface{})
	jaegerValues := make(map[string]interface{})

	defer func() {
		if reterr == nil {
			if len(jaegerValues) > 0 {
				if err := setHelmValue(tracingValues, "jaeger", jaegerValues); err != nil {
					reterr = err
				}
			}
			if err := overwriteHelmValues(values, tracingValues, "tracing"); err != nil {
				reterr = err
			}
		}
	}()

	if jaeger.Name != "" {
		if err := setHelmStringValue(jaegerValues, "resourceName", jaeger.Name); err != nil {
			return err
		}
		collectorAddr := fmt.Sprintf("%s-collector.%s.svc:9411", jaeger.Name, ns)
		if err := setHelmStringValue(values, "meshConfig.defaultConfig.tracing.zipkin.address", collectorAddr); err != nil {
			return err
		}
	}

	if jaeger.Install == nil {
		return nil
	}

	if jaeger.Install.Storage != nil {
		switch jaeger.Install.Storage.Type {
		case v2.JaegerStorageTypeMemory:
			if err := setHelmStringValue(jaegerValues, "template", "all-in-one"); err != nil {
				return err
			}
		case v2.JaegerStorageTypeElasticsearch:
			if err := setHelmStringValue(jaegerValues, "template", "production-elasticsearch"); err != nil {
				return err
			}
		case "":
			// don't configure anything
		}
		// Memory settings - for round tripping
		if jaeger.Install.Storage.Memory != nil {
			if jaeger.Install.Storage.Memory.MaxTraces != nil {
				if err := setHelmIntValue(jaegerValues, "memory.max_traces", *jaeger.Install.Storage.Memory.MaxTraces); err != nil {
					return err
				}
			}
		}
		// Elasticsearch settings -for round tripping
		if jaeger.Install.Storage.Elasticsearch != nil {
			elasticSearchValues := make(map[string]interface{})
			if jaeger.Install.Storage.Elasticsearch.NodeCount != nil {
				if err := setHelmIntValue(elasticSearchValues, "nodeCount", int64(*jaeger.Install.Storage.Elasticsearch.NodeCount)); err != nil {
					return err
				}
			}
			if len(jaeger.Install.Storage.Elasticsearch.Storage.GetContent()) > 0 {
				if storageValues, err := toValues(jaeger.Install.Storage.Elasticsearch.Storage.GetContent()); err == nil {
					if err := setHelmValue(elasticSearchValues, "storage", storageValues); err != nil {
						return err
					}
				} else {
					return err
				}
			}
			if jaeger.Install.Storage.Elasticsearch.RedundancyPolicy != "" {
				if err := setHelmValue(elasticSearchValues, "redundancyPolicy", jaeger.Install.Storage.Elasticsearch.RedundancyPolicy); err != nil {
					return err
				}
			}
			if len(jaeger.Install.Storage.Elasticsearch.IndexCleaner.GetContent()) > 0 {
				if cleanerValues, err := toValues(jaeger.Install.Storage.Elasticsearch.IndexCleaner.GetContent()); err == nil {
					if err := setHelmValue(jaegerValues, "esIndexCleaner", cleanerValues); err != nil {
						return err
					}
				} else {
					return err
				}
			}
			if len(elasticSearchValues) > 0 {
				if err := setHelmValue(jaegerValues, "elasticsearch", elasticSearchValues); err != nil {
					return err
				}
			}
		}
	}

	if jaeger.Install.Ingress != nil {
		if jaeger.Install.Ingress.Enabled != nil {
			if err := setHelmBoolValue(tracingValues, "ingress.enabled", *jaeger.Install.Ingress.Enabled); err != nil {
				return err
			}
		}
		if jaeger.Install.Ingress.Metadata != nil {
			if len(jaeger.Install.Ingress.Metadata.Annotations) > 0 {
				if err := setHelmStringMapValue(tracingValues, "ingress.annotations", jaeger.Install.Ingress.Metadata.Annotations); err != nil {
					return err
				}
			}
			if len(jaeger.Install.Ingress.Metadata.Labels) > 0 {
				if err := setHelmStringMapValue(tracingValues, "ingress.labels", jaeger.Install.Ingress.Metadata.Labels); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func populateJaegerAddonConfig(in *v1.HelmValues, out *v2.JaegerAddonConfig) (bool, error) {
	rawTracingValues, ok, err := in.GetMap("tracing")
	if err != nil {
		return false, err
	} else if !ok || len(rawTracingValues) == 0 {
		return false, nil
	}
	tracingValues := v1.NewHelmValues(rawTracingValues)
	rawJaegerValues, _, err := tracingValues.GetMap("jaeger")
	if err != nil {
		return false, err
	}
	jaegerValues := v1.NewHelmValues(rawJaegerValues)

	jaeger := out
	setJaeger := false
	if resourceName, ok, err := jaegerValues.GetAndRemoveString("resourceName"); ok && resourceName != "" {
		jaeger.Name = resourceName
		setJaeger = true
	} else if err != nil {
		return false, err
	}

	install := &v2.JaegerInstallConfig{}
	setInstall := false

	storage := &v2.JaegerStorageConfig{}
	setStorage := false
	if template, ok, err := jaegerValues.GetAndRemoveString("template"); ok {
		switch template {
		case "all-in-one":
			setStorage = true
			storage.Type = v2.JaegerStorageTypeMemory
		case "production-elasticsearch":
			setStorage = true
			storage.Type = v2.JaegerStorageTypeElasticsearch
		case "":
			// do nothing
		default:
			return false, fmt.Errorf("unknown jaeger.template: %s", template)
		}
	} else if err != nil {
		return false, err
	}
	if maxTraces, ok, err := jaegerValues.GetAndRemoveInt64("memory.max_traces"); ok {
		storage.Memory = &v2.JaegerMemoryStorageConfig{
			MaxTraces: &maxTraces,
		}
		setStorage = true
	} else if err != nil {
		return false, err
	}
	elasticsearch := &v2.JaegerElasticsearchStorageConfig{}
	setElasticsearch := false
	if rawElasticsearchValues, ok, err := jaegerValues.GetMap("elasticsearch"); ok && len(rawElasticsearchValues) > 0 {
		elasticsearchValues := v1.NewHelmValues(rawElasticsearchValues)
		if rawNodeCount, ok, err := elasticsearchValues.GetAndRemoveInt64("nodeCount"); ok {
			nodeCount := int32(rawNodeCount)
			elasticsearch.NodeCount = &nodeCount
			setElasticsearch = true
		} else if err != nil {
			return false, err
		}
		if rawStorage, ok, err := elasticsearchValues.GetMap("storage"); ok && len(rawStorage) > 0 {
			storage := v1.NewHelmValues(nil)
			if err := fromValues(rawStorage, storage); err == nil {
				elasticsearch.Storage = storage
				setElasticsearch = true
			} else {
				return false, err
			}
			elasticsearchValues.RemoveField("storage")
		} else if err != nil {
			return false, err
		}
		if redundancyPolicy, ok, err := elasticsearchValues.GetAndRemoveString("redundancyPolicy"); ok && redundancyPolicy != "" {
			elasticsearch.RedundancyPolicy = redundancyPolicy
			setElasticsearch = true
		} else if err != nil {
			return false, err
		}
		if len(elasticsearchValues.GetContent()) == 0 {
			jaegerValues.RemoveField("elasticsearch")
		} else if err := jaegerValues.SetField("elasticsearch", elasticsearchValues.GetContent()); err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}
	if rawESIndexCleaner, ok, err := jaegerValues.GetMap("esIndexCleaner"); ok && len(rawESIndexCleaner) > 0 {
		esIndexCleaner := v1.NewHelmValues(nil)
		if err := fromValues(rawESIndexCleaner, esIndexCleaner); err == nil {
			elasticsearch.IndexCleaner = esIndexCleaner
			setElasticsearch = true
		} else {
			return false, err
		}
		jaegerValues.RemoveField("esIndexCleaner")
	} else if err != nil {
		return false, err
	}
	if setElasticsearch {
		storage.Elasticsearch = elasticsearch
		setStorage = true
	}
	if setStorage {
		install.Storage = storage
		setInstall = true
	}

	ingressConfig := &v2.JaegerIngressConfig{}
	setIngressConfig := false
	if enabled, ok, err := tracingValues.GetAndRemoveBool("ingress.enabled"); ok {
		ingressConfig.Enabled = &enabled
		setIngressConfig = true
	} else if err != nil {
		return false, err
	}
	metadata := &v2.MetadataConfig{}
	setMetadata := false
	if rawAnnotations, ok, err := tracingValues.GetMap("ingress.annotations"); ok && len(rawAnnotations) > 0 {
		setMetadata = true
		if err := setMetadataAnnotations(rawAnnotations, metadata); err != nil {
			return false, err
		}
		tracingValues.RemoveField("ingress.annotations")
	} else if err != nil {
		return false, err
	}
	if rawLabels, ok, err := tracingValues.GetMap("ingress.labels"); ok && len(rawLabels) > 0 {
		setMetadata = true
		if err := setMetadataLabels(rawLabels, metadata); err != nil {
			return false, err
		}
		tracingValues.RemoveField("ingress.labels")
	} else if err != nil {
		return false, err
	}
	if setMetadata {
		setIngressConfig = true
		ingressConfig.Metadata = metadata
	}
	if setIngressConfig {
		install.Ingress = ingressConfig
		setInstall = true
	}

	if setInstall {
		jaeger.Install = install
		setJaeger = true
	}

	if len(jaegerValues.GetContent()) == 0 {
		tracingValues.RemoveField("jaeger")
	} else if err := tracingValues.SetField("jaeger", jaegerValues.GetContent()); err != nil {
		return false, err
	}
	if len(tracingValues.GetContent()) == 0 {
		in.RemoveField("tracing")
	} else if err := in.SetField("tracing", tracingValues.GetContent()); err != nil {
		return false, err
	}

	return setJaeger, nil
}
