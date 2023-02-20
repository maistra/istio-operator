package memberroll

import (
	"context"
	"fmt"

	pkgerrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

const (
	prometheusConfigMapName         = "prometheus"
	prometheusConfigurationFilename = "prometheus.yml"

	prometheusScrapeConfigKeyName   = "scrape_configs"
	prometheusScrapeSDConfigKeyName = "kubernetes_sd_configs"
)

type PrometheusReconciler interface {
	reconcilePrometheus(ctx context.Context, prometheusCMName, prometheusNamespace string, configuredMembers []string) error
}

type defaultPrometheusReconciler struct {
	Client client.Client
}

func (r *defaultPrometheusReconciler) reconcilePrometheus(ctx context.Context, prometheusCMName, prometheusNamespace string, members []string) error {
	reqLogger := common.LogFromContext(ctx)
	reqLogger.Info("Attempting to get Prometheus ConfigMap", "prometheusNamespace", prometheusNamespace, "prometheusCMName", prometheusCMName)

	cm := &corev1.ConfigMap{}

	err := r.Client.Get(ctx, client.ObjectKey{Name: prometheusCMName, Namespace: prometheusNamespace}, cm)
	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) || errors.IsGone(err) {
			reqLogger.Info("Prometheus ConfigMap '%s/%s' does not exist", prometheusNamespace, prometheusCMName)
			return nil
		}
		return pkgerrors.Wrap(err, "error retrieving Prometheus ConfigMap from mesh")
	}

	data := make(map[string]interface{})

	err = yaml.Unmarshal([]byte(cm.Data[prometheusConfigurationFilename]), &data)

	if err != nil {
		return pkgerrors.Wrap(err, "error unmarshaling prometheus.yml")
	}

	f, ok, err := unstructured.NestedFieldNoCopy(data, prometheusScrapeConfigKeyName)

	var scrapes []interface{}

	if err == nil && ok {
		scrapes, ok = f.([]interface{})

		if !ok {
			return fmt.Errorf("error no scrape_configs found from %v", f)
		}
	} else {
		return pkgerrors.Wrap(err, "error getting scrape_configs from ConfigMap")
	}

	for _, s := range scrapes {
		scrape, ok := s.(map[string]interface{})

		if !ok {
			return fmt.Errorf("error converting scrape_config from %v", s)
		}

		if scrape["job_name"] == "pilot" {
			continue
		}

		f, ok, err := unstructured.NestedFieldNoCopy(scrape, prometheusScrapeSDConfigKeyName)

		if err == nil {
			if ok {
				sds, ok := f.([]interface{})

				if ok {
					for _, v := range sds {
						sd, ok := v.(map[string]interface{})

						if ok {
							reqLogger.Info(fmt.Sprintf("Updating sd %v", v))

							err = unstructured.SetNestedStringSlice(sd, members, "namespaces", "names")

							if err != nil {
								return pkgerrors.Wrap(err, fmt.Sprintf("error setting sd %v", v))
							}
						}
					}
				} else {
					return fmt.Errorf("error can not process sd %v", f)
				}
			} else {
				reqLogger.Info(fmt.Sprintf("Ignoring scrape %v", s))
			}
		} else {
			return pkgerrors.Wrap(err, fmt.Sprintf("error getting sd from %v", s))
		}
	}

	updatedPrometheus := &corev1.ConfigMap{}

	updatedPrometheus.SetName(prometheusCMName)
	updatedPrometheus.SetNamespace(prometheusNamespace)

	updatedConfigurationFileData, err := yaml.Marshal(data)
	if err != nil {
		return pkgerrors.Wrap(err, "error marshaling updated prometheus configuration")
	}

	updatedConfigurationFile := string(updatedConfigurationFileData)

	reqLogger.Info(fmt.Sprintf("Prometheus updated configuration file %v", updatedConfigurationFile))

	updatedPrometheus.Data = map[string]string{
		prometheusConfigurationFilename: updatedConfigurationFile,
	}

	err = r.Client.Patch(ctx, updatedPrometheus, client.Merge)

	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) || errors.IsGone(err) {
			reqLogger.Info(fmt.Sprintf("skipping Prometheus update, %s/%s is no longer available", prometheusNamespace, prometheusCMName))
			return nil
		}
		return pkgerrors.Wrapf(err, "cannot update Prometheus ConfigMap %s/%s with namespaces %v", prometheusNamespace, prometheusCMName, members)
	}

	reqLogger.Info("Prometheus ConfigMap scraping namespaces updated", "namespaces", members)

	return nil
}
