package common

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/renderutil"
	"k8s.io/helm/pkg/tiller"
	"k8s.io/helm/pkg/timeconv"

	"github.com/maistra/istio-operator/pkg/apis/maistra"
)

func init() {
	// inject OpenShift specific kinds into the ordering list
	serviceIndex := IndexOf(tiller.InstallOrder, "Service")
	// we want route before oauthclient before deployments
	tiller.InstallOrder = append(tiller.InstallOrder[:serviceIndex], append([]string{"Route", "OAuthClient"}, tiller.InstallOrder[serviceIndex:]...)...)
}

type options struct {
	// ResourceDir is the base dir to helm charts and templates files.
	ResourceDir string
	// ChartsDir is the base dir to helm charts.
	ChartsDir string
	// DefaultTemplatesDir is the base dir to default templates files.
	DefaultTemplatesDir string
	// TemplatesDir is the base dir to user supplied templates files.
	UserTemplatesDir string

	// Number of concurrent reconcilers for each controller
	ControlPlaneReconcilers int
	MemberRollReconcilers   int
	MemberReconcilers       int

	// The number of API requests the operator can make before throttling
	Burst int

	// Then maximum rate of API requests when throttling is active
	QPS float32
}

var Options = &options{}

// GetChartsDir returns the location of the Helm charts. Similar layout to istio.io/istio/install/kubernetes/helm.
func (o *options) GetChartsDir(maistraVersion string) string {
	if len(maistraVersion) == 0 {
		maistraVersion = maistra.DefaultVersion.String()
	}
	if len(o.ChartsDir) == 0 {
		return path.Join(o.ResourceDir, "helm", maistraVersion)
	}
	return path.Join(o.ChartsDir, maistraVersion)
}

// GetTemplatesDir returns the location of the Operator templates files
func (o *options) GetUserTemplatesDir() string {
	if len(o.UserTemplatesDir) == 0 {
		return path.Join(o.ResourceDir, "templates")
	}
	return o.UserTemplatesDir
}

// GetDefaultTemplatesDir returns the location of the Default Operator templates files
func (o *options) GetDefaultTemplatesDir(maistraVersion string) string {
	if len(maistraVersion) == 0 {
		maistraVersion = maistra.DefaultVersion.String()
	}
	if len(o.DefaultTemplatesDir) == 0 {
		return path.Join(o.ResourceDir, "default-templates", maistraVersion)
	}
	return path.Join(o.DefaultTemplatesDir, maistraVersion)
}

// RenderHelmChart renders the helm charts, returning a map of rendered templates.
// key names represent the chart from which the template was processed.  Subcharts
// will be keyed as <root-name>/charts/<subchart-name>, e.g. istio/charts/galley.
// The root chart would be simply, istio.
func RenderHelmChart(chartPath string, namespace string, values interface{}) (map[string][]manifest.Manifest, map[string]interface{}, error) {
	rawVals, err := yaml.Marshal(values)
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
		// XXX: hard-code or look this up somehow?
		KubeVersion: fmt.Sprintf("%s.%s", chartutil.DefaultKubeVersion.Major, chartutil.DefaultKubeVersion.Minor),
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
		manifestsByChart[key] = tiller.SortByKind(value)
	}
	return manifestsByChart
}
