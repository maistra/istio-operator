package controlplane

import (
	"context"

	"github.com/maistra/istio-operator/pkg/controller/common"
)

func (r *controlPlaneInstanceReconciler) processComponentManifests(ctx context.Context, chartName string) (ready bool, err error) {
	r.lastComponent = ""
	componentName := componentFromChartName(chartName)
	log := common.LogFromContext(ctx).WithValues("Component", componentName)
	ctx = common.NewContextWithLog(ctx, log)

	renderings, hasRenderings := r.renderings[chartName]
	if !hasRenderings {
		log.V(5).Info("no renderings for component")
		ready = true
		return
	}

	log.Info("reconciling component resources")
	status := r.Status.FindComponentByName(componentName)
	defer func() {
		updateReconcileStatus(&status.StatusType, err)
		log.Info("component reconciliation complete")
	}()

	mp := common.NewManifestProcessor(r.ControllerResources, r.Instance.GetNamespace(), r.meshGeneration, r.Instance.GetNamespace(), r.preprocessObject, r.processNewObject)
	if err = mp.ProcessManifests(ctx, renderings, status.Resource); err != nil {
		return
	}
	if err = r.processNewComponent(componentName, status); err != nil {
		log.Error(err, "error during postprocessing of component")
		return
	}

	// if we get here, the component has been successfully installed
	delete(r.renderings, chartName)

	// for reentry into the reconcile loop, if not ready
	if notReadyMap, readyErr := r.calculateNotReadyState(ctx); readyErr == nil {
		if notReadyMap[componentName] {
			r.lastComponent = componentName
		} else {
			ready = true
		}
	} else {
		err = readyErr
	}
	return
}
