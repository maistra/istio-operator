package conversion

import (
	"fmt"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateJaegerAddonValues(jaeger *v2.JaegerTracerConfig, values map[string]interface{}) error {
	if err := setHelmStringValue(values, "global.proxy.tracer", "jaeger"); err != nil {
		return err
	}
	if err := setHelmStringValue(values, "tracing.provider", "jaeger"); err != nil {
		return err
	}

	if jaeger == nil {
		return nil
	}

	if err := setHelmBoolValue(values, "tracing.enabled", true); err != nil {
		return err
	}
	if err := setHelmStringValue(values, "tracing.jaeger.resourceName", jaeger.Name); err != nil {
		return err
	}

	if jaeger.Install == nil {
		// XXX: do we need to be setting global.tracer.zipkin.address?
		if err := setHelmBoolValue(values, "tracing.jaeger.install", false); err != nil {
			return err
		}
		return nil
	}

	tracingValues := make(map[string]interface{})
	jaegerValues := make(map[string]interface{})

	if err := setHelmStringValue(tracingValues, "provider", "jaeger"); err != nil {
		return err
	}

	if err := setHelmBoolValue(jaegerValues, "install", true); err != nil {
		return err
	}

	if jaeger.Install.Config.Storage == nil {
		// set in-memory as default
		jaeger.Install.Config.Storage = &v2.JaegerStorageConfig{
			Type: v2.JaegerStorageTypeMemory,
		}
	}
	switch jaeger.Install.Config.Storage.Type {
	case v2.JaegerStorageTypeMemory:
		if err := setHelmStringValue(jaegerValues, "template", "all-in-one"); err != nil {
			return err
		}
		if jaeger.Install.Config.Storage.Memory != nil {
			if jaeger.Install.Config.Storage.Memory.MaxTraces != nil {
				if err := setHelmIntValue(jaegerValues, "memory.max_traces", int64(*jaeger.Install.Config.Storage.Memory.MaxTraces)); err != nil {
					return err
				}
			}
		}
	case v2.JaegerStorageTypeElasticsearch:
		if err := setHelmStringValue(jaegerValues, "template", "production-elasticsearch"); err != nil {
			return err
		}
		if jaeger.Install.Config.Storage.Elasticsearch != nil {
			if jaeger.Install.Config.Storage.Elasticsearch.NodeCount != nil {
				if err := setHelmIntValue(jaegerValues, "elasticsearch.nodeCount", int64(*jaeger.Install.Config.Storage.Elasticsearch.NodeCount)); err != nil {
					return err
				}
			}
			if len(jaeger.Install.Config.Storage.Elasticsearch.Storage.GetContent()) > 0 {
				if err := setHelmValue(jaegerValues, "elasticsearch.storage", jaeger.Install.Config.Storage.Elasticsearch.Storage.GetContent()); err != nil {
					return err
				}
			}
			if len(jaeger.Install.Config.Storage.Elasticsearch.RedundancyPolicy.GetContent()) > 0 {
				if err := setHelmValue(jaegerValues, "elasticsearch.redundancyPolicy", jaeger.Install.Config.Storage.Elasticsearch.RedundancyPolicy.GetContent()); err != nil {
					return err
				}
			}
			if len(jaeger.Install.Config.Storage.Elasticsearch.IndexCleaner.GetContent()) > 0 {
				if err := setHelmValue(jaegerValues, "elasticsearch.esIndexCleaner", jaeger.Install.Config.Storage.Elasticsearch.IndexCleaner.GetContent()); err != nil {
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
		if jaeger.Install.Ingress.Enabled != nil {
			if err := setHelmBoolValue(tracingValues, "ingress.enabled", *jaeger.Install.Ingress.Enabled); err != nil {
				return err
			}
		}
		if len(jaeger.Install.Ingress.Metadata.Annotations) > 0 {
			if err := setHelmStringMapValue(tracingValues, "ingress.annotations", jaeger.Install.Ingress.Metadata.Annotations); err != nil {
				return err
			}
		}
	}

	if jaeger.Install.Runtime != nil {
		runtime := jaeger.Install.Runtime
		if err := populateRuntimeValues(runtime, tracingValues); err != nil {
			return err
		}

		// need to move some of these into tracing.jaeger
		if podAnnotations, ok := tracingValues["podAnnotations"]; ok {
			if err := setHelmValue(jaegerValues, "podAnnotations", podAnnotations); err != nil {
				return err
			}
		}

		if runtime.Pod.Containers != nil {
			if defaultContainer, ok := jaeger.Install.Runtime.Pod.Containers["default"]; ok && defaultContainer.Resources != nil {
				if resourcesValues, err := toValues(defaultContainer.Resources); err == nil {
					if len(resourcesValues) > 0 {
						if err := setHelmValue(jaegerValues, "resources", resourcesValues); err != nil {
							return err
						}
					}
				} else {
					return err
				}
			}
		}
	}

	if len(jaegerValues) > 0 {
		if err := setHelmValue(tracingValues, "jaeger", jaegerValues); err != nil {
			return err
		}
	}
	if len(tracingValues) > 0 {
		if err := setHelmValue(values, "tracing", tracingValues); err != nil {
			return err
		}
	}

	return nil
}

func populateTracingAddonConfig(in *v1.HelmValues, out *v2.AddonsConfig) error {
	if tracer, ok, err := in.GetString("tracing.provider"); ok && tracer != "" {
		if out.Tracing.Type, err = tracerTypeFromString(tracer); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if tracer, ok, err := in.GetString("global.proxy.tracer"); ok && tracer != "" {
		if out.Tracing.Type, err = tracerTypeFromString(tracer); err != nil {
			return err
		}
	} else {
		out.Tracing.Type = v2.TracerTypeNone
	}

	switch out.Tracing.Type {
	case v2.TracerTypeJaeger:
		return populateJaegerAddonConfig(in, out)
	case v2.TracerTypeNone:
		return nil
	}
	return fmt.Errorf("unknown tracer type: %s", out.Tracing.Type)
}

func populateJaegerAddonConfig(in *v1.HelmValues, out *v2.AddonsConfig) error {
	rawTracingValues, ok, err := in.GetMap("tracing")
	if err != nil {
		return err
	}
	tracingValues := v1.NewHelmValues(rawTracingValues)
	rawJaegerValues, ok, err := tracingValues.GetMap("jaeger")
	if err != nil {
		return err
	} else if !ok || len(rawJaegerValues) == 0 {
		return nil
	}
	jaegerValues := v1.NewHelmValues(rawJaegerValues)

	out.Tracing.Jaeger = &v2.JaegerTracerConfig{}
	jaeger := out.Tracing.Jaeger

	if resourceName, ok, err := jaegerValues.GetString("resourceName"); ok && resourceName != "" {
		jaeger.Name = resourceName
	} else if err != nil {
		return err
	} else {
		jaeger.Name = "jaeger"
	}

	if enabled, ok, err := tracingValues.GetBool("enabled"); ok && !enabled {
		// no install for this case.  tracer settings will be configured from
		// referenced resource
		return nil
	} else if err != nil {
		return nil
	}

	jaeger.Install = &v2.JaegerInstallConfig{}

	if template, ok, err := jaegerValues.GetString("template"); ok {
		switch template {
		case "all-in-one":
			jaeger.Install.Config.Storage = &v2.JaegerStorageConfig{
				Type:   v2.JaegerStorageTypeMemory,
				Memory: &v2.JaegerMemoryStorageConfig{},
			}
			if maxTraces, ok, err := jaegerValues.GetInt64("memory.max_traces"); ok {
				jaeger.Install.Config.Storage.Memory.MaxTraces = &maxTraces
			} else if err != nil {
				return err
			}
		case "production-elasticsearch":
			jaeger.Install.Config.Storage = &v2.JaegerStorageConfig{
				Type:          v2.JaegerStorageTypeElasticsearch,
				Elasticsearch: &v2.JaegerElasticsearchStorageConfig{},
			}
			if rawElasticsearchValues, ok, err := jaegerValues.GetMap("elasticsearch"); ok && len(rawElasticsearchValues) > 0 {
				elasticsearchValues := v1.NewHelmValues(rawElasticsearchValues)
				if rawNodeCount, ok, err := elasticsearchValues.GetInt64("nodeCount"); ok {
					nodeCount := int32(rawNodeCount)
					jaeger.Install.Config.Storage.Elasticsearch.NodeCount = &nodeCount
				} else if err != nil {
					return err
				}
				if storage, ok, err := elasticsearchValues.GetMap("storage"); ok && len(storage) > 0 {
					jaeger.Install.Config.Storage.Elasticsearch.Storage = v1.NewHelmValues(storage)
				} else if err != nil {
					return err
				}
				if redundancyPolicy, ok, err := elasticsearchValues.GetMap("redundancyPolicy"); ok && len(redundancyPolicy) > 0 {
					jaeger.Install.Config.Storage.Elasticsearch.RedundancyPolicy = v1.NewHelmValues(redundancyPolicy)
				} else if err != nil {
					return err
				}
				if esIndexCleaner, ok, err := elasticsearchValues.GetMap("esIndexCleaner"); ok && len(esIndexCleaner) > 0 {
					jaeger.Install.Config.Storage.Elasticsearch.RedundancyPolicy = v1.NewHelmValues(esIndexCleaner)
				} else if err != nil {
					return err
				}
				podRuntime := &v2.PodRuntimeConfig{}
				if applied, err := runtimeValuesToPodRuntimeConfig(elasticsearchValues, podRuntime); err != nil {
					return err
				} else if applied {
					jaeger.Install.Config.Storage.Elasticsearch.Runtime = podRuntime
				}
			} else if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown jaeger.template: %s", template)
		}
	} else if err != nil {
		return err
	}

	ingressConfig := &v2.JaegerIngressConfig{}
	setIngressConfig := false
	if enabled, ok, err := tracingValues.GetBool("ingress.enabled"); ok {
		ingressConfig.Enabled = &enabled
		setIngressConfig = true
	} else if err != nil {
		return err
	}
	if rawAnnotations, ok, err := tracingValues.GetMap("ingress.annotations"); ok {
		setIngressConfig = true
		if err := setMetadataAnnotations(rawAnnotations, &ingressConfig.Metadata); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if setIngressConfig {
		jaeger.Install.Ingress = ingressConfig
	}

	// need to move jaeger.podAnnotations to tracing.podAnnotations
	if podAnnotations, ok, err := jaegerValues.GetMap("podAnnotations"); ok && len(podAnnotations) > 0 {
		tracingValues = tracingValues.DeepCopy()
		if err := tracingValues.SetField("podAnnotations", podAnnotations); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	runtime := &v2.ComponentRuntimeConfig{}
	if applied, err := runtimeValuesToComponentRuntimeConfig(tracingValues, runtime); err != nil {
		return err
	} else if applied {
		jaeger.Install.Runtime = runtime
	}

	defaultContainer := v2.ContainerConfig{}
	if applied, err := populateContainerConfig(jaegerValues, &defaultContainer); err != nil {
		return err
	} else if applied {
		jaeger.Install.Runtime.Pod.Containers = map[string]v2.ContainerConfig{
			"default": defaultContainer,
		}
	}

	return nil
}

func tracerTypeFromString(tracer string) (v2.TracerType, error) {
	switch strings.ToLower(tracer) {
	case strings.ToLower(string(v2.TracerTypeJaeger)):
		return v2.TracerTypeJaeger, nil
	case "":
		return v2.TracerTypeNone, nil
	}
	return v2.TracerTypeNone, fmt.Errorf("unknown tracer type %s", tracer)
}
