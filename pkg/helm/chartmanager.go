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
	"errors"
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	chartLoader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type ChartManager struct {
	restClientGetter genericclioptions.RESTClientGetter
	driver           string
}

// NewChartManager creates a new Helm chart manager using cfg as the configuration
// that Helm will use to connect to the cluster when installing or uninstalling
// charts, and using the specified driver to store information about releases
// (one of: memory, secret, configmap, sql, or "" (same as "secret")).
func NewChartManager(cfg *rest.Config, driver string) *ChartManager {
	return &ChartManager{
		restClientGetter: NewRESTClientGetter(cfg),
		driver:           driver,
	}
}

// newActionConfig Create a new Helm action config from in-cluster service account
func (h *ChartManager) newActionConfig(ctx context.Context, namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	logAdapter := func(format string, v ...interface{}) {
		log := logf.FromContext(ctx)
		logv2 := log.V(2)
		if logv2.Enabled() {
			logv2.Info(fmt.Sprintf(format, v...))
		}
	}
	if err := actionConfig.Init(h.restClientGetter, namespace, h.driver, logAdapter); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

// UpgradeOrInstallChart upgrades a chart in cluster or installs it new if it does not already exist
func (h *ChartManager) UpgradeOrInstallChart(
	ctx context.Context, chartDir string, values Values,
	namespace, releaseName string, ownerReference metav1.OwnerReference,
) (*release.Release, error) {
	log := logf.FromContext(ctx)

	cfg, err := h.newActionConfig(ctx, namespace)
	if err != nil {
		return nil, err
	}

	chart, err := chartLoader.Load(chartDir)
	if err != nil {
		return nil, err
	}

	rel, err := getRelease(cfg, releaseName)
	if err != nil {
		return rel, err
	}

	var releaseExists bool

	if rel == nil {
		releaseExists = false
	} else if rel.Info.Status == release.StatusDeployed {
		releaseExists = true
	} else if rel.Info.Status == release.StatusPendingUpgrade || (rel.Info.Status == release.StatusFailed && rel.Version > 1) {
		log.V(2).Info("Performing helm rollback", "release", releaseName)
		if err := action.NewRollback(cfg).Run(releaseName); err != nil {
			return nil, fmt.Errorf("failed to roll back helm release %s: %v", releaseName, err)
		}
		releaseExists = true
	} else if rel.Info.Status == release.StatusPendingInstall || (rel.Info.Status == release.StatusFailed && rel.Version <= 1) {
		log.V(2).Info("Performing helm uninstall", "release", releaseName)
		if _, err := action.NewUninstall(cfg).Run(releaseName); err != nil {
			return nil, fmt.Errorf("failed to uninstall failed helm release %s: %v", releaseName, err)
		}
		releaseExists = false
	} else if rel.Info.Status == release.StatusPendingRollback {
		return nil, fmt.Errorf("unrecoverable helm release status %s", rel.Info.Status)
	} else {
		return nil, fmt.Errorf("unexpected helm release status %s", rel.Info.Status)
	}

	if releaseExists {
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

// UninstallChart removes a chart from the cluster
func (h *ChartManager) UninstallChart(ctx context.Context, releaseName, namespace string) (*release.UninstallReleaseResponse, error) {
	cfg, err := h.newActionConfig(ctx, namespace)
	if err != nil {
		return nil, err
	}

	if rel, err := getRelease(cfg, releaseName); err != nil {
		return nil, err
	} else if rel == nil {
		// release does not exist; no need for uninstall
		return &release.UninstallReleaseResponse{Info: "release not found"}, nil
	}

	return action.NewUninstall(cfg).Run(releaseName)
}

func getRelease(cfg *action.Configuration, releaseName string) (*release.Release, error) {
	getAction := action.NewGet(cfg)
	rel, err := getAction.Run(releaseName)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, fmt.Errorf("failed to get helm release %s: %v", releaseName, err)
	}
	return rel, nil
}
