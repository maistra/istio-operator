package bootstrap

import (
	"path"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/maistra/istio-operator/pkg/controller/common"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var installCNITask sync.Once

// InstallCNI makes sure all Istio CNI resources have been created.  CRDs are located from
// files in controller.HelmDir/istio-init/files
func InstallCNI(mgr manager.Manager) error {
	// we only try to install CNI once.  if there's an error, we should probably
	// panic, as there's no way to recover.  for now, we just pass the error along.
	var err error
	installCNITask.Do(func() {
		err = internalInstallCNI(mgr)
	})
	return err
}

func internalInstallCNI(mgr manager.Manager) error {
	log.Info("ensuring Istio CNI has been installed")

	operatorNamespace, err := common.GetOperatorNamespace()
	if err != nil {
		return err
	}

	log.Info("rendering Istio CNI chart")

	values := make(map[string]interface{})
	values["enabled"] = common.IsCNIEnabled
	values["image"] = common.CNIImage
	values["imagePullSecrets"] = common.CNIImagePullSecrets
	// TODO: imagePullPolicy, resources

	renderings, _, err := common.RenderHelmChart(path.Join(common.GetHelmDir(), "istio_cni"), operatorNamespace, values)
	if err != nil {
		return err
	}

	resourceManager := common.ResourceManager{
		Client:            mgr.GetClient(),
		PatchFactory:      common.NewPatchFactory(mgr.GetClient()),
		Log:               log,
		OperatorNamespace: operatorNamespace,
	}

	mp := common.NewManifestProcessor(resourceManager, "istio_cni", "TODO", "maistra-istio-operator", preProcessObject, postProcessObject)
	if err = mp.ProcessManifests(renderings["istio_cni"], "istio_cni"); err != nil {
		return err
	}

	return nil
}

func preProcessObject(obj *unstructured.Unstructured) error {
	return nil
}

func postProcessObject(obj *unstructured.Unstructured) error {
	return nil
}
