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
	"context"
	"fmt"
	"os"
	"path"

	"helm.sh/helm/v3/pkg/action"
	chartLoader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Client struct {
	ResourceDirectory string
	restClientGetter  genericclioptions.RESTClientGetter
}

func NewClient(resourceDirectory string, cfg *rest.Config) *Client {
	return &Client{
		ResourceDirectory: resourceDirectory,
		restClientGetter:  NewRESTClientGetter(cfg),
	}
}

func (h *Client) UninstallCharts(ctx context.Context, charts []string, releaseNameBase, ns string) error {
	actionConfig, err := h.newActionConfig(ctx, ns)
	if err != nil {
		return err
	}
	for _, chartName := range charts {
		releaseName := fmt.Sprintf("%s-%s", releaseNameBase, chartName)
		_, err = h.uninstallChart(actionConfig, ns, releaseName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *Client) UpgradeOrInstallCharts(ctx context.Context, charts []string, values HelmValues, chartVersion, releaseNameBase, ns string, ownerReference metav1.OwnerReference) error {
	actionConfig, err := h.newActionConfig(ctx, ns)
	if err != nil {
		return err
	}
	for _, chartName := range charts {
		releaseName := fmt.Sprintf("%s-%s", releaseNameBase, chartName)
		_, err = h.upgradeOrInstallChart(ctx, actionConfig, chartName, chartVersion, ns, releaseName, ownerReference, values)
		if err != nil {
			return err
		}
	}
	return nil
}

// newActionConfig Create a new Helm action config from in-cluster service account
func (h *Client) newActionConfig(ctx context.Context, namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	logAdapter := func(format string, v ...interface{}) {
		log := logf.FromContext(ctx)
		logv2 := log.V(2)
		if logv2.Enabled() {
			logv2.Info(fmt.Sprintf(format, v...))
		}
	}
	if err := actionConfig.Init(h.restClientGetter, namespace, os.Getenv("HELM_DRIVER"), logAdapter); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

// upgradeOrInstallChart upgrades a chart in cluster or installs it new if it does not already exist
func (h *Client) upgradeOrInstallChart(ctx context.Context, cfg *action.Configuration,
	chartName, chartVersion, namespace, releaseName string,
	ownerReference metav1.OwnerReference, values HelmValues,
) (*release.Release, error) {
	log := logf.FromContext(ctx)

	// Helm List Action
	listAction := action.NewList(cfg)
	releases, err := listAction.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list installed helm releases: %v", err)
	}

	toUpgrade := false
	for _, rel := range releases {
		if rel.Name == releaseName && rel.Namespace == namespace {
			toUpgrade = true
		}
	}

	chart, err := chartLoader.Load(path.Join(h.ResourceDirectory, chartVersion, "charts", chartName))
	if err != nil {
		return nil, err
	}
	var rel *release.Release
	if toUpgrade {
		log.V(2).Info("Performing helm upgrade", "chartName", chart.Name())
		updateAction := action.NewUpgrade(cfg)
		updateAction.PostRenderer = NewOwnerReferencePostRenderer(ownerReference, "")
		updateAction.MaxHistory = 1
		updateAction.SkipCRDs = true
		rel, err = updateAction.RunWithContext(ctx, releaseName, chart, values)
		if err != nil {
			return nil, fmt.Errorf("failed to update helm chart %s: %v", chart.Name(), err)
		}

	} else {
		log.V(2).Info("Performing helm install", "chartName", chart.Name())
		installAction := action.NewInstall(cfg)
		installAction.PostRenderer = NewOwnerReferencePostRenderer(ownerReference, "")
		installAction.Namespace = namespace
		installAction.ReleaseName = releaseName
		installAction.SkipCRDs = true
		rel, err = installAction.RunWithContext(ctx, chart, values)
		if err != nil {
			return nil, fmt.Errorf("failed to install helm chart %s: %v", chart.Name(), err)
		}
	}
	return rel, nil
}

// uninstallChart removes a chart from the cluster
func (h *Client) uninstallChart(cfg *action.Configuration, namespace, releaseName string) (*release.UninstallReleaseResponse, error) {
	// Helm List Action
	listAction := action.NewList(cfg)
	releases, err := listAction.Run()
	if err != nil {
		return nil, err
	}

	found := false
	for _, rel := range releases {
		if rel.Name == releaseName && rel.Namespace == namespace {
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
