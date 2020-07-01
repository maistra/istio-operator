package conversion

import (
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateJaegerAddonValues(jaeger *v2.JaegerTracerConfig, values map[string]interface{}) error {
	if jaeger == nil {
		return setHelmValue(values, "tracing.enabled", false)
	}
	if jaeger.Install == nil {
		// XXX: not sure if this is correct. we don't want the charts processed,
		// but other aspects might not be configured correctly
		if err := setHelmValue(values, "tracing.enabled", false); err != nil {
			return err
		}
		if err := setHelmValue(values, "global.proxy.tracer", "jaeger"); err != nil {
			return err
		}
		if err := setHelmValue(values, "tracing.provider", "jaeger"); err != nil {
			return err
		}
		return nil
		// XXX: do we need to be setting global.tracer.zipkin.address?
	}

	tracingValues := make(map[string]interface{})
	jaegerValues := make(map[string]interface{})

	if err := setHelmValue(tracingValues, "provider", "jaeger"); err != nil {
		return err
	}
	if err := setHelmValue(tracingValues, "enabled", true); err != nil {
		return err
	}

	if jaeger.Install.Config.Storage == nil {
		// set in-memory as default
		jaeger.Install.Config.Storage.Type = v2.JaegerStorageTypeMemory
	}
	switch jaeger.Install.Config.Storage.Type {
	case v2.JaegerStorageTypeMemory:
		if err := setHelmValue(jaegerValues, "template", "all-in-one"); err != nil {
			return err
		}
		if jaeger.Install.Config.Storage.Memory != nil {
			if jaeger.Install.Config.Storage.Memory.MaxTraces != nil {
				if err := setHelmValue(jaegerValues, "memory.max_traces", *jaeger.Install.Config.Storage.Memory.MaxTraces); err != nil {
					return err
				}
			}
		}
	case v2.JaegerStorageTypeElasticsearch:
		if err := setHelmValue(jaegerValues, "template", "production-elasticsearch"); err != nil {
			return err
		}
		if jaeger.Install.Config.Storage.Elasticsearch != nil {
			if jaeger.Install.Config.Storage.Elasticsearch.NodeCount != nil {
				if err := setHelmValue(jaegerValues, "elasticsearch.nodeCount", *jaeger.Install.Config.Storage.Elasticsearch.NodeCount); err != nil {
					return err
				}
			}
			if len(jaeger.Install.Config.Storage.Elasticsearch.Storage) > 0 {
				if err := setHelmValue(jaegerValues, "elasticsearch.storage", jaeger.Install.Config.Storage.Elasticsearch.Storage); err != nil {
					return err
				}
			}
			if len(jaeger.Install.Config.Storage.Elasticsearch.RedundancyPolicy) > 0 {
				if err := setHelmValue(jaegerValues, "elasticsearch.redundancyPolicy", jaeger.Install.Config.Storage.Elasticsearch.RedundancyPolicy); err != nil {
					return err
				}
			}
			if len(jaeger.Install.Config.Storage.Elasticsearch.IndexCleaner) > 0 {
				if err := setHelmValue(jaegerValues, "elasticsearch.esIndexCleaner", jaeger.Install.Config.Storage.Elasticsearch.IndexCleaner); err != nil {
					return err
				}
			}
			if jaeger.Install.Config.Storage.Elasticsearch.Runtime != nil {
				esRuntimeValues := make(map[string]interface{})
				if err := populatePodHelmValues(jaeger.Install.Config.Storage.Elasticsearch.Runtime, esRuntimeValues); err == nil {
					for key, value := range esRuntimeValues {
						if err := setHelmValue(jaegerValues, "elasticsearch."+key, value); err != nil {
							return err
						}
					}
				} else {
					return err
				}
			}
		}
	}

	if jaeger.Install.Ingress != nil {
		if err := setHelmValue(tracingValues, "ingress.enabled", true); err != nil {
			return err
		}
		if len(jaeger.Install.Ingress.Metadata.Annotations) > 0 {
			if err := setHelmValue(tracingValues, "ingress.annotations", jaeger.Install.Ingress.Metadata.Annotations); err != nil {
				return err
			}
		}
	}

	if err := populateRuntimeValues(jaeger.Install.Runtime, tracingValues); err != nil {
		return err
	}

	// need to move some of these into tracing.jaeger
	if podAnnotations, ok := tracingValues["podAnnotations"]; ok {
		if err := setHelmValue(jaegerValues, "podAnnotations", podAnnotations); err != nil {
			return err
		}
	}

    if jaeger.Install.Runtime != nil {
        if defaultResources, ok := jaeger.Install.Runtime.Pod.Containers["default"]; ok {
            if resourcesValues, err := toValues(defaultResources); err == nil {
                if err := setHelmValue(jaegerValues, "resources", resourcesValues); err != nil {
                    return err
                }
            } else {
                return err
            }
        }
    }

    if err := setHelmValue(tracingValues, "jaeger", jaegerValues); err != nil {
        return err
    }
    if err := setHelmValue(values, "podAnnotations", tracingValues); err != nil {
        return err
    }

    return nil
}
