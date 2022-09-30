package bootstrap

import (
	"context"
	"path"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/helm/pkg/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/cni"
	"github.com/maistra/istio-operator/pkg/controller/common/helm"
	"github.com/maistra/istio-operator/pkg/controller/versions"
)

// InstallCNI makes sure all Istio CNI resources have been created.  CRDs are located from
// files in controller.HelmDir/istio-init/files
func InstallCNI(ctx context.Context, cl client.Client, config cni.Config, dc discovery.DiscoveryInterface, ver versions.Version) error {
	// ver is from a SMCP spec version
	if ver == nil {
		ver = versions.DefaultVersion.Version()
	}
	// we should run through this each reconcile to make sure it's there
	return internalInstallCNI(ctx, cl, config, dc, ver)
}

func internalInstallCNI(ctx context.Context, cl client.Client, config cni.Config, dc discovery.DiscoveryInterface, ver versions.Version) error {
	if ver == nil {
		ver = versions.DefaultVersion.Version()
	}
	renderings, err := internalRenderCNI(ctx, cl, config, dc, versions.GetSupportedVersions(), ver)
	if err != nil {
		return err
	}
	return internalProcessManifests(ctx, cl, renderings["istio_cni"])
}

func internalRenderCNI(ctx context.Context, cl client.Client, config cni.Config, dc discovery.DiscoveryInterface,
	supportedVersions []versions.Version, ver versions.Version,
) (renderings map[string][]manifest.Manifest, err error) {
	log := common.LogFromContext(ctx)
	log.Info("ensuring Istio CNI has been installed")

	operatorNamespace := common.GetOperatorNamespace()

	log.Info("rendering Istio CNI chart")

	cni := make(map[string]interface{})
	cni["enabled"] = config.Enabled
	cni["image_v1_1"] = common.Config.OLM.Images.V1_1.CNI
	cni["image_v2_0"] = common.Config.OLM.Images.V2_0.CNI
	cni["image_v2_1"] = common.Config.OLM.Images.V2_1.CNI
	cni["image_v2_2"] = common.Config.OLM.Images.V2_2.CNI
	cni["image_v2_3"] = common.Config.OLM.Images.V2_3.CNI
	cni["imagePullSecrets"] = config.ImagePullSecrets
	// TODO: imagePullPolicy, resources

	cni["logLevel"] = common.Config.OLM.CNILogLevel

	cni["configMap_v1_1"] = "cni_network_config_v1_1"
	cni["configMap_v2_0"] = "cni_network_config_v2_0"
	cni["configMap_v2_1"] = "cni_network_config_v2_1"
	cni["configMap_v2_2"] = "cni_network_config_v2_2"
	cni["configMap_v2_3"] = "cni_network_config_v2_3"

	cni["chained"] = !config.UseMultus
	if config.UseMultus {
		cni["cniBinDir"] = "/opt/multus/bin"
		cni["cniConfDir"] = "/etc/cni/multus/net.d"
		cni["mountedCniConfDir"] = "/host/etc/cni/multus/net.d"

		cni["cniConfFileName_v2_0"] = "v2-0-istio-cni.conf"
		cni["cniConfFileName_v2_1"] = "v2-1-istio-cni.conf"
		cni["cniConfFileName_v2_2"] = "v2-2-istio-cni.conf"
		cni["cniConfFileName_v2_3"] = "v2-3-istio-cni.conf"
	}

	var releases []string
	if config.UseMultus {
		for _, version := range supportedVersions {
			releases = append(releases, version.String())
		}
	} else {
		releases = append(releases, versions.DefaultVersion.String())
	}
	cni["supportedReleases"] = releases
	// v2.3 render the required cni daemonset,
	// instanceVersion is from a SMCP spec version
	if ver == nil {
		ver = versions.DefaultVersion.Version()
	}
	cni["instanceVersion"] = ver.String()

	values := make(map[string]interface{})
	values["cni"] = cni

	serverVersion, err := dc.ServerVersion()
	if err != nil {
		return nil, err
	}

	// the "istio-node" DaemonSet is now called "istio-cni-node", so we must delete the old one
	ds := v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istio-node",
			Namespace: operatorNamespace,
		},
	}
	err = cl.Delete(ctx, &ds)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	// always install the latest version of the CNI image
	renderings, _, err = helm.RenderChart(path.Join(versions.DefaultVersion.GetChartsDir(), "istio_cni"), operatorNamespace, serverVersion.String(), values)
	return
}

func internalProcessManifests(ctx context.Context, cl client.Client, rendering []manifest.Manifest) error {
	operatorNamespace := common.GetOperatorNamespace()

	controllerResources := common.ControllerResources{
		Client:            cl,
		OperatorNamespace: operatorNamespace,
	}

	mp := helm.NewManifestProcessor(controllerResources, helm.NewPatchFactory(cl), "istio_cni", "TODO",
		types.NamespacedName{}, preProcessObject, postProcessObject, preProcessObjectForPatch)
	if _, err := mp.ProcessManifests(ctx, rendering, "istio_cni"); err != nil {
		return err
	}

	return nil
}

func preProcessObject(ctx context.Context, obj *unstructured.Unstructured) (bool, error) {
	return true, nil
}

func postProcessObject(ctx context.Context, obj *unstructured.Unstructured) error {
	return nil
}

func preProcessObjectForPatch(ctx context.Context, oldObj, newObj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return newObj, nil
}
