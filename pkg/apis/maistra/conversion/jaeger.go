package conversion

import (
	"fmt"
	"strings"

	v1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	v2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
)

func populateJaegerAddonValues(jaeger *v2.JaegerTracerConfig, values map[string]interface{}) (reterr error) {

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
			for key, value := range tracingValues {
				if err := setHelmValue(values, "tracing."+key, value); err != nil {
					reterr = err
				}
			}
		}
	}()

	if jaeger.Name != "" {
		if err := setHelmStringValue(jaegerValues, "resourceName", jaeger.Name); err != nil {
			return err
		}
	}

	if jaeger.Install == nil {
		// XXX: do we need to be setting global.tracer.zipkin.address?
		if err := setHelmBoolValue(jaegerValues, "install", false); err != nil {
			return err
		}
		return nil
	}

	if err := setHelmStringValue(tracingValues, "provider", "jaeger"); err != nil {
		return err
	}

	if err := setHelmBoolValue(jaegerValues, "install", true); err != nil {
		return err
	}

	if jaeger.Install.Storage != nil {
		switch jaeger.Install.Storage.Type {
		case v2.JaegerStorageTypeMemory:
			if err := setHelmStringValue(jaegerValues, "template", "all-in-one"); err != nil {
				return err
			}
			if jaeger.Install.Storage.Memory != nil {
				if jaeger.Install.Storage.Memory.MaxTraces != nil {
					if err := setHelmIntValue(jaegerValues, "memory.max_traces", int64(*jaeger.Install.Storage.Memory.MaxTraces)); err != nil {
						return err
					}
				}
			}
		case v2.JaegerStorageTypeElasticsearch:
			if err := setHelmStringValue(jaegerValues, "template", "production-elasticsearch"); err != nil {
				return err
			}
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
						if err := setHelmValue(elasticSearchValues, "esIndexCleaner", cleanerValues); err != nil {
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
		case "":
			// don't configure anything
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
		if len(jaeger.Install.Ingress.Metadata.Labels) > 0 {
			if err := setHelmStringMapValue(tracingValues, "ingress.labels", jaeger.Install.Ingress.Metadata.Labels); err != nil {
				return err
			}
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
	} else if err != nil {
		return err
	} else if traceEnabled, ok, err := in.GetBool("tracing.enabled"); ok {
		if traceEnabled {
			// default to jaeger if enabled and no proxy.tracer specified
			out.Tracing.Type = v2.TracerTypeJaeger
		} else {
			out.Tracing.Type = v2.TracerTypeNone
		}
	} else if err != nil {
		return err
	}

	switch out.Tracing.Type {
	case v2.TracerTypeJaeger:
		return populateJaegerAddonConfig(in, out)
	case v2.TracerTypeNone:
		return nil
	case "":
		return nil
	}
	return fmt.Errorf("unknown tracer type: %s", out.Tracing.Type)
}

func populateJaegerAddonConfig(in *v1.HelmValues, out *v2.AddonsConfig) error {
	rawTracingValues, ok, err := in.GetMap("tracing")
	if err != nil {
		return err
	} else if !ok || len(rawTracingValues) == 0 {
		return nil
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
	}

	if enabled, ok, err := tracingValues.GetBool("enabled"); ok && !enabled {
		// no install for this case.  tracer settings will be configured from
		// referenced resource
		return nil
	} else if err != nil {
		return nil
	}

	setInstall := false
	if shouldInstall, ok, err := jaegerValues.GetBool("install"); ok {
		if shouldInstall {
			setInstall = true
		} else {
			return nil
		}
	} else if err != nil {
		return nil
	}

	install := &v2.JaegerInstallConfig{}
	storage := &v2.JaegerStorageConfig{}
	setStorage := false
	if template, ok, err := jaegerValues.GetString("template"); ok {
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
			return fmt.Errorf("unknown jaeger.template: %s", template)
		}
	} else if err != nil {
		return err
	}
	if maxTraces, ok, err := jaegerValues.GetInt64("memory.max_traces"); ok {
		storage.Memory = &v2.JaegerMemoryStorageConfig{
			MaxTraces: &maxTraces,
		}
		setStorage = true
	} else if err != nil {
		return err
	}
	if rawElasticsearchValues, ok, err := jaegerValues.GetMap("elasticsearch"); ok && len(rawElasticsearchValues) > 0 {
		elasticsearchValues := v1.NewHelmValues(rawElasticsearchValues)
		elasticsearch := &v2.JaegerElasticsearchStorageConfig{}
		setElasticsearch := false
		if rawNodeCount, ok, err := elasticsearchValues.GetInt64("nodeCount"); ok {
			nodeCount := int32(rawNodeCount)
			elasticsearch.NodeCount = &nodeCount
			setElasticsearch = true
		} else if err != nil {
			return err
		}
		if rawStorage, ok, err := elasticsearchValues.GetMap("storage"); ok && len(rawStorage) > 0 {
			storage := v1.NewHelmValues(nil)
			if err := fromValues(rawStorage, storage); err == nil {
				elasticsearch.Storage = storage
				setElasticsearch = true
			} else {
				return err
			}
		} else if err != nil {
			return err
		}
		if redundancyPolicy, ok, err := elasticsearchValues.GetString("redundancyPolicy"); ok && redundancyPolicy != "" {
			elasticsearch.RedundancyPolicy = redundancyPolicy
			setElasticsearch = true
		} else if err != nil {
			return err
		}
		if rawESIndexCleaner, ok, err := elasticsearchValues.GetMap("esIndexCleaner"); ok && len(rawESIndexCleaner) > 0 {
			esIndexCleaner := v1.NewHelmValues(nil)
			if err := fromValues(rawESIndexCleaner, esIndexCleaner); err == nil {
				elasticsearch.IndexCleaner = esIndexCleaner
				setElasticsearch = true
			} else {
				return err
			}
		} else if err != nil {
			return err
		}
		if setElasticsearch {
			storage.Elasticsearch = elasticsearch
			setStorage = true
		}
	} else if err != nil {
		return err
	}
	if setStorage {
		install.Storage = storage
		setInstall = true
	}

	ingressConfig := &v2.JaegerIngressConfig{}
	setIngressConfig := false
	if enabled, ok, err := tracingValues.GetBool("ingress.enabled"); ok {
		ingressConfig.Enabled = &enabled
		setIngressConfig = true
	} else if err != nil {
		return err
	}
	if rawAnnotations, ok, err := tracingValues.GetMap("ingress.annotations"); ok && len(rawAnnotations) > 0 {
		setIngressConfig = true
		if err := setMetadataAnnotations(rawAnnotations, &ingressConfig.Metadata); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if rawLabels, ok, err := tracingValues.GetMap("ingress.labels"); ok && len(rawLabels) > 0 {
		setIngressConfig = true
		if err := setMetadataLabels(rawLabels, &ingressConfig.Metadata); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if setIngressConfig {
		install.Ingress = ingressConfig
		setInstall = true
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
	// need to move jaeger.podLables to tracing.podLabels
	if podLabels, ok, err := jaegerValues.GetMap("podLabels"); ok && len(podLabels) > 0 {
		tracingValues = tracingValues.DeepCopy()
		if err := tracingValues.SetField("podLabels", podLabels); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if setInstall {
		jaeger.Install = install
	}

	return nil
}

func tracerTypeFromString(tracer string) (v2.TracerType, error) {
	switch strings.ToLower(tracer) {
	case strings.ToLower(string(v2.TracerTypeJaeger)):
		return v2.TracerTypeJaeger, nil
	case strings.ToLower(string(v2.TracerTypeNone)):
		return v2.TracerTypeNone, nil
	}
	return v2.TracerTypeNone, fmt.Errorf("unknown tracer type %s", tracer)
}
