package bootstrap

import (
	"context"
	"path"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/maistra/istio-operator/pkg/controller/common"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// InstallCNI makes sure all Istio CNI resources have been created.  CRDs are located from
// files in controller.HelmDir/istio-init/files
func InstallCNI(ctx context.Context, cl client.Client) error {
	// we should run through this each reconcile to make sure it's there
	return internalInstallCNI(ctx, cl)
}

func internalInstallCNI(ctx context.Context, cl client.Client) error {
	log := common.LogFromContext(ctx)
	log.Info("ensuring Istio CNI has been installed")

	operatorNamespace := common.GetOperatorNamespace()

	log.Info("rendering Istio CNI chart")

	values := make(map[string]interface{})
	values["enabled"] = common.IsCNIEnabled
	values["image_v1_0"] = common.CNIImageV1_0
	values["image_v1_1"] = common.CNIImageV1_1
	values["imagePullSecrets"] = common.CNIImagePullSecrets
	// TODO: imagePullPolicy, resources

	// always install the latest version of the CNI image
	renderings, _, err := common.RenderHelmChart(path.Join(common.GetHelmDir(common.DefaultMaistraVersion), "istio_cni"), operatorNamespace, values)
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
