package controlplane

import (
	"context"

	"github.com/maistra/istio-operator/pkg/controller/common"
	"github.com/maistra/istio-operator/pkg/controller/common/helm"
)

func (r *controlPlaneInstanceReconciler) processComponentManifests(ctx context.Context, chartName string) (ready bool, err error) {
	r.lastComponent = ""
	componentName := componentFromChartName(chartName)
	log := common.LogFromContext(ctx).WithValues("Component", componentName)
	ctx = common.NewContextWithLog(ctx, log)

	renderings, hasRenderings := r.renderings[chartName]
	if !hasRenderings {
		log.V(5).Info("no renderings for component")
		return true, nil
	}

	log.Info("reconciling component resources")
	status := r.Status.FindComponentByName(componentName)
	defer func() {
		updateComponentConditions(status, err)
		log.Info("component reconciliation complete")
	}()

	mp := helm.NewManifestProcessor(r.ControllerResources, helm.NewPatchFactory(r.Client), r.Instance.GetNamespace(), r.meshGeneration, r.Instance.GetNamespace(), r.preprocessObject, r.processNewObject)
	if err = mp.ProcessManifests(ctx, renderings, status.Resource); err != nil {
		return false, err
	}
	if err = r.processNewComponent(componentName, status); err != nil {
		log.Error(err, "error during postprocessing of component")
		return false, err
	}

	// if we get here, the component has been successfully installed
	delete(r.renderings, chartName)

	// for reentry into the reconcile loop, if not ready
	readinessMap, readyErr := r.calculateComponentReadiness(ctx)
	if readyErr != nil {
		return false, readyErr
	}

	ready, exists := readinessMap[componentName]
	if exists && !ready {
		r.lastComponent = componentName
		return false, nil
	}
	return true, nil
}
