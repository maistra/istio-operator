/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm

import (
	"os"
	"path"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	chartLoader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	logger               = log.Log.WithName("helm")
	ResourceDirectory, _ = filepath.Abs("resources/charts") // "/var/lib/istio-operator/resources"
)

// GetActionConfig Get the Helm action config from in cluster service account
func GetActionConfig(namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	var kubeConfig *genericclioptions.ConfigFlags
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// Set properties manually from official rest config
	kubeConfig = genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &config.Host
	kubeConfig.BearerToken = &config.BearerToken
	kubeConfig.CAFile = &config.CAFile
	kubeConfig.Namespace = &namespace
	if err := actionConfig.Init(kubeConfig, namespace, os.Getenv("HELM_DRIVER"), logger.V(2).Info); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

// UpgradeOrInstallChart upgrades a chart in cluster or installs it new if it does not already exist
// TODO: use context
func UpgradeOrInstallChart(chartName, chartVersion, namespace, releaseName string, values map[string]interface{}) (*release.Release, error) {
	// Helm List Action
	cfg, err := GetActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	listAction := action.NewList(cfg)
	releases, err := listAction.Run()
	if err != nil {
		return nil, err
	}

	toUpgrade := false
	for _, release := range releases {
		if release.Name == releaseName && release.Namespace == namespace {
			toUpgrade = true
		}
	}

	chart, err := chartLoader.Load(path.Join(ResourceDirectory, chartVersion, chartName))
	if err != nil {
		return nil, err
	}
	var rel *release.Release
	if toUpgrade {
		logger.V(2).Info("Performing helm upgrade", "chartName", chart.Name())
		updateAction := action.NewUpgrade(cfg)
		rel, err = updateAction.Run(releaseName, chart, values)
		if err != nil {
			return nil, err
		}

	} else {
		logger.V(2).Info("Performing helm install", "chartName", chart.Name())
		installAction := action.NewInstall(cfg)
		installAction.Namespace = namespace
		installAction.ReleaseName = releaseName
		rel, err = installAction.Run(chart, values)
		if err != nil {
			return nil, err
		}
	}
	return rel, nil
}

// UninstallChart removes a chart from the cluster
func UninstallChart(namespace, releaseName string) (*release.UninstallReleaseResponse, error) {
	// Helm List Action
	cfg, err := GetActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	listAction := action.NewList(cfg)
	releases, err := listAction.Run()
	if err != nil {
		return nil, err
	}

	found := false
	for _, release := range releases {
		if release.Name == releaseName && release.Namespace == namespace {
			found = true
		}
	}
	if !found {
		return nil, nil
	}

	uninstallAction := action.NewUninstall(cfg)
	response, err := uninstallAction.Run(releaseName)
	if err != nil {
		return nil, err
	}

	return response, nil
}
