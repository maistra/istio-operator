package controlplane

import (
	"context"

	"github.com/maistra/istio-operator/controllers/common"
	"github.com/maistra/istio-operator/controllers/common/helm"
)

func (r *controlPlaneInstanceReconciler) processComponentManifests(ctx context.Context, chartName string) (madeChanges bool, err error) {
	componentName := componentFromChartName(chartName)
	log := common.LogFromContext(ctx).WithValues("Component", componentName)
	ctx = common.NewContextWithLog(ctx, log)

	renderings, found := r.renderings[chartName]
	if !found {
		log.V(5).Info("no renderings for component")
		return false, nil
	}

	log.Info("reconciling component resources")
	status := r.Status.FindComponentByName(componentName)
	defer func() {
		updateComponentConditions(status, err)
		log.Info("component reconciliation complete")
	}()

	mp := helm.NewManifestProcessor(r.ControllerResources, helm.NewPatchFactory(r.Client), r.Instance.GetNamespace(),
		r.meshGeneration, common.ToNamespacedName(r.Instance), r.preprocessObject, r.processNewObject, r.preprocessObjectForPatch)
	if madeChanges, err = mp.ProcessManifests(ctx, renderings, status.Resource); err != nil {
		return madeChanges, err
	}
	if err = r.processNewComponent(componentName, status); err != nil {
		log.Error(err, "error during postprocessing of component")
		return madeChanges, err
	}

	return madeChanges, nil
}

func (r *controlPlaneInstanceReconciler) anyComponentHasReadiness(chartName string) bool {
	for _, rendering := range r.renderings[chartName] {
		if r.hasReadiness(rendering.Head.Kind) {
			return true
		}
	}
	return false
}
