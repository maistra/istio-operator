package bootstrap

import (
	"context"
	"path"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/apis/maistra"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

// InstallCNI makes sure all Istio CNI resources have been created.  CRDs are located from
// files in controller.HelmDir/istio-init/files
func InstallCNI(ctx context.Context, cl client.Client, config common.CNIConfig) error {
	// we should run through this each reconcile to make sure it's there
	return internalInstallCNI(ctx, cl, config)
}

func internalInstallCNI(ctx context.Context, cl client.Client, config common.CNIConfig) error {
	log := common.LogFromContext(ctx)
	log.Info("ensuring Istio CNI has been installed")

	operatorNamespace := common.GetOperatorNamespace()

	log.Info("rendering Istio CNI chart")

	values := make(map[string]interface{})
	values["enabled"] = config.Enabled
	values["image_v1_0"] = config.ImageV1_0
	values["image_v1_1"] = config.ImageV1_1
	values["imagePullSecrets"] = config.ImagePullSecrets
	// TODO: imagePullPolicy, resources

	// always install the latest version of the CNI image
	renderings, _, err := common.RenderHelmChart(path.Join(common.Options.GetChartsDir(maistra.DefaultVersion.String()), "istio_cni"), operatorNamespace, values)
	if err != nil {
		return err
	}

	controllerResources := common.ControllerResources{
		Client:            cl,
		PatchFactory:      common.NewPatchFactory(cl),
		OperatorNamespace: operatorNamespace,
	}

	mp := common.NewManifestProcessor(controllerResources, "istio_cni", "TODO", "maistra-istio-operator", preProcessObject, postProcessObject)
	if err = mp.ProcessManifests(ctx, renderings["istio_cni"], "istio_cni"); err != nil {
		return err
	}

	return nil
}

func preProcessObject(ctx context.Context, obj *unstructured.Unstructured) error {
	return nil
}

func postProcessObject(ctx context.Context, obj *unstructured.Unstructured) error {
	return nil
}
