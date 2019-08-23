package controlplane

import (
	"github.com/maistra/istio-operator/pkg/controller/common"
)

func (r *ControlPlaneReconciler) processComponentManifests(chartName string) (ready bool, err error) {
	r.lastComponent = ""
	componentName := componentFromChartName(chartName)
	origLogger := r.Log
	r.Log = r.Log.WithValues("Component", componentName)
	defer func() { r.Log = origLogger }()

	renderings, hasRenderings := r.renderings[chartName]
	if !hasRenderings {
		r.Log.V(5).Info("no renderings for component")
		ready = true
		return
	}

	r.Log.Info("reconciling component resources")
	status := r.Status.FindComponentByName(componentName)
	defer func() {
		updateReconcileStatus(&status.StatusType, err)
		r.Log.Info("component reconciliation complete")
	}()

	mp := common.NewManifestProcessor(r.ResourceManager, r.Instance.GetNamespace(), r.meshGeneration, r.Instance.GetNamespace(), r.preprocessObject, r.processNewObject)
	if err = mp.ProcessManifests(renderings, status.Resource); err != nil {
		return
	}
	if err = r.processNewComponent(componentName, status); err != nil {
		r.Log.Error(err, "error during postprocessing of component")
		return
	}

	// if we get here, the component has been successfully installed
	delete(r.renderings, chartName)

	// for reentry into the reconcile loop, if not ready
	if notReadyMap, readyErr := r.calculateNotReadyState(); readyErr == nil {
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
