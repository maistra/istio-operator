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

	scrapingMembers := append(members, prometheusNamespace)

	for _, s := range scrapes {
		scrape, ok := s.(map[string]interface{})

		if !ok {
			return fmt.Errorf("error converting scrape_config from %v", s)
		}

		if scrape["job_name"] == "pilot" {
			continue
		}

		reqLogger.V(2).Info("Processing Prometheus scrape target: %v", scrape)
		err := processScrapeConfig(scrape, scrapingMembers)
		if err != nil {
			return err
		}
	}

	updatedConfigMap := &corev1.ConfigMap{}

	updatedConfigMap.SetName(prometheusCMName)
	updatedConfigMap.SetNamespace(prometheusNamespace)

	updatedConfigurationFileData, err := yaml.Marshal(data)
	if err != nil {
		return pkgerrors.Wrap(err, "error marshaling updated prometheus configuration")
	}

	updatedConfigurationFile := string(updatedConfigurationFileData)

	reqLogger.V(2).Info(fmt.Sprintf("Prometheus updated configuration file %v", updatedConfigurationFile))

	updatedConfigMap.Data = map[string]string{
		prometheusConfigurationFilename: updatedConfigurationFile,
	}

	err = r.Client.Patch(ctx, updatedConfigMap, client.Merge)

	if err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) || errors.IsGone(err) {
			reqLogger.Info(fmt.Sprintf("skipping Prometheus update, %s/%s is no longer available", prometheusNamespace, prometheusCMName))
			return nil
		}
		return pkgerrors.Wrapf(err, "cannot update Prometheus ConfigMap %s/%s with namespaces %v", prometheusNamespace, prometheusCMName, scrapingMembers)
	}

	reqLogger.Info("Prometheus ConfigMap scraping namespaces updated", "namespaces", scrapingMembers)

	return nil
}

func processScrapeConfig(scrape map[string]interface{}, scrapingMembers []string) error {
	f, ok, err := unstructured.NestedFieldNoCopy(scrape, prometheusScrapeSDConfigKeyName)
	if err != nil {
		return pkgerrors.Wrap(err, fmt.Sprintf("error getting sd from %v", scrape))
	}

	if !ok {
		return nil
	}

	sds, ok := f.([]interface{})
	if !ok {
		return fmt.Errorf("error can not process sd %v", f)
	}

	errors := []error{}

	for _, v := range sds {
		if sd, ok := v.(map[string]interface{}); ok {
			err = setServiceDiscoveryNamespaces(sd, scrapingMembers)
			if err != nil {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) != 0 {
		msg := ""
		for _, e := range errors {
			msg += fmt.Sprintf("%s\n", e.Error())
		}

		return pkgerrors.New(fmt.Sprintf("error processing scrape %v%s", scrape, msg))
	}

	return nil
}

func setServiceDiscoveryNamespaces(sd map[string]interface{}, scrapingMembers []string) error {
	err := unstructured.SetNestedStringSlice(sd, scrapingMembers, "namespaces", "names")
	if err != nil {
		return pkgerrors.Wrap(err, fmt.Sprintf("error setting sd %v", sd))
	}

	return nil
}
