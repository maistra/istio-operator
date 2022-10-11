package helm

import (
	"encoding/json"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/renderutil"
	"k8s.io/helm/pkg/timeconv"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

func init() {
	// inject OpenShift specific kinds into the ordering list
	serviceIndex := common.IndexOf(InstallOrder, "Service")
	// we want route before oauthclient before deployments
	InstallOrder = append(InstallOrder[:serviceIndex], append([]string{"Route", "OAuthClient"}, InstallOrder[serviceIndex:]...)...)
}

// RenderChart renders the helm charts, returning a map of rendered templates.
// key names represent the chart from which the template was processed.  Subcharts
// will be keyed as <root-name>/charts/<subchart-name>, e.g. istio/charts/galley.
// The root chart would be simply, istio.
func RenderChart(chartPath, namespace, kubeVersion string, values interface{}) (map[string][]manifest.Manifest, map[string]interface{}, error) {
	rawVals, err := yaml.Marshal(values)
	if err != nil {
		return map[string][]manifest.Manifest{}, nil, err
	}
	config := &chart.Config{Raw: string(rawVals), Values: map[string]*chart.Value{}}

	c, err := chartutil.Load(chartPath)
	if err != nil {
		return map[string][]manifest.Manifest{}, nil, err
	}

	renderOpts := renderutil.Options{
		ReleaseOptions: chartutil.ReleaseOptions{
			// XXX: hard code or use icp.GetName()
			Name:      "istio",
			IsInstall: true,
			IsUpgrade: false,
			Time:      timeconv.Now(),
			Namespace: namespace,
		},
		KubeVersion: kubeVersion,
	}
	renderedTemplates, err := renderutil.Render(c, config, renderOpts)
	if err != nil {
		return map[string][]manifest.Manifest{}, nil, err
	}

	rel := &release.Release{
		Name:      renderOpts.ReleaseOptions.Name,
		Chart:     c,
		Config:    config,
		Namespace: namespace,
		Info:      &release.Info{LastDeployed: renderOpts.ReleaseOptions.Time},
	}
	rawRel := map[string]interface{}{}
	data, err := json.Marshal(rel)
	if err == nil {
		err = json.Unmarshal(data, &rawRel)
	}
	return sortManifestsByChart(manifest.SplitManifests(renderedTemplates)), rawRel, err
}

// sortManifestsByChart returns a map of chart->[]manifest.  names for subcharts
// will be of the form <root-name>/charts/<subchart-name>, e.g. istio/charts/galley
func sortManifestsByChart(manifests []manifest.Manifest) map[string][]manifest.Manifest {
	manifestsByChart := make(map[string][]manifest.Manifest)
	for _, chartManifest := range manifests {
		pathSegments := strings.Split(chartManifest.Name, "/")
		chartName := pathSegments[0]
		// paths always start with the root chart name and always have a template
		// name, so we should be safe not to check length
		if pathSegments[1] == "charts" {
			// subcharts will have names like <root-name>/charts/<subchart-name>/...
			chartName = strings.Join(pathSegments[:3], "/")
		}
		if _, ok := manifestsByChart[chartName]; !ok {
			manifestsByChart[chartName] = make([]manifest.Manifest, 0, 10)
		}
		manifestsByChart[chartName] = append(manifestsByChart[chartName], chartManifest)
	}
	for key, value := range manifestsByChart {
		manifestsByChart[key] = SortByKind(value)
	}
	return manifestsByChart
}
