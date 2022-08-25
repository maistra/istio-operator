package controlplane

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/maistra/istio-operator/pkg/apis/external"
	kialiv1alpha1 "github.com/maistra/istio-operator/pkg/apis/external/kiali/v1alpha1"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	"github.com/maistra/istio-operator/pkg/controller/common"
)

func (r *controlPlaneInstanceReconciler) PatchAddons(ctx context.Context, spec *maistrav2.ControlPlaneSpec) (reconcile.Result, error) {
	// so far, only need to patch kiali
	return r.patchKiali(ctx, spec.IsGrafanaEnabled(), spec.IsJaegerEnabled())
}

func (r *controlPlaneInstanceReconciler) patchKiali(ctx context.Context, grafanaEnabled, jaegerEnabled bool) (reconcile.Result, error) {
	if r.Instance == nil || !r.Instance.Status.AppliedSpec.IsKialiEnabled() {
		return common.Reconciled()
	}

	log := common.LogFromContext(ctx)

	kialiConfig := r.Instance.Status.AppliedSpec.Addons.Kiali

	// get the kiali resource
	kiali := &kialiv1alpha1.Kiali{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: kialiConfig.ResourceName(), Namespace: r.Instance.Namespace}, kiali); err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) {
			log.Info(fmt.Sprintf("requeue patching Kiali after %s update, because %s/%s is not available",
				patchKialiRequeueInterval, kiali.GetNamespace(), kiali.GetName()))
		}
		return common.RequeueAfter(patchKialiRequeueInterval)
	}

	log.Info("patching kiali CR", kiali.Kind, kiali.GetName())

	if kiali.Spec == nil {
		kiali.Spec = maistrav1.NewHelmValues(make(map[string]interface{}))
	}

	updatedKiali := &kialiv1alpha1.Kiali{
		Base: external.Base{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kiali.Name,
				Namespace: kiali.Namespace,
			},
			Spec: maistrav1.NewHelmValues(make(map[string]interface{})),
		},
	}

	// grafana
	grafanaURL := r.grafanaURL(ctx, log)
	if grafanaURL == "" {
		// disable grafana
		if err := updatedKiali.Spec.SetField("external_services.grafana.enabled", false); err != nil {
			return common.RequeueWithError(errorOnSettingValueInKialiCR("external_services.grafana.enabled", err))
		}
	} else {
		// enable grafana and set URL
		// XXX: should we also configure the in_cluster_url
		if err := updatedKiali.Spec.SetField("external_services.grafana.url", grafanaURL); err != nil {
			return common.RequeueWithError(errorOnSettingValueInKialiCR("external_services.grafana.url", err))
		}
		if err := updatedKiali.Spec.SetField("external_services.grafana.enabled", true); err != nil {
			return common.RequeueWithError(errorOnSettingValueInKialiCR("external_services.grafana.enabled", err))
		}
	}

	// jaeger
	jaegerURL := r.jaegerURL(ctx, log)
	if jaegerURL == "" {
		// disable jaeger
		if err := updatedKiali.Spec.SetField("external_services.tracing.enabled", false); err != nil {
			return common.RequeueWithError(errorOnSettingValueInKialiCR("external_services.tracing.enabled", err))
		}
	} else {
		// enable jaeger and set URL
		// XXX: should we also configure the in_cluster_url
		if err := updatedKiali.Spec.SetField("external_services.tracing.url", jaegerURL); err != nil {
			return common.RequeueWithError(errorOnSettingValueInKialiCR("external_services.tracing.url", err))
		}
		if err := updatedKiali.Spec.SetField("external_services.tracing.enabled", true); err != nil {
			return common.RequeueWithError(errorOnSettingValueInKialiCR("external_services.tracing.enabled", err))
		}
	}

	// XXX: should we also configure prometheus?

	// credentials
	rawPassword, err := r.getRawHtPasswd(ctx)
	if err != nil {
		return common.RequeueWithError(fmt.Errorf("could not get htpasswd required for kiali external_serivces: %s", err))
	}
	if err := updatedKiali.Spec.SetField("external_services.grafana.auth.password", rawPassword); err != nil {
		return common.RequeueWithError(errorOnSettingValueInKialiCR("external_services.grafana.auth.password", err))
	}
	if err := updatedKiali.Spec.SetField("external_services.prometheus.auth.password", rawPassword); err != nil {
		return common.RequeueWithError(errorOnSettingValueInKialiCR("external_services.prometheus.auth.password", err))
	}
	if err := updatedKiali.Spec.SetField("external_services.tracing.auth.password", rawPassword); err != nil {
		return common.RequeueWithError(errorOnSettingValueInKialiCR("external_services.tracing.auth.password", err))
	}

	// FUTURE: add support for synchronizing kiali version with control plane version

	if err := r.Client.Patch(ctx, updatedKiali, client.Merge); err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) {
			log.Info(fmt.Sprintf("requeue patching Kiali after %s update, because %s/%s is no longer available",
				patchKialiRequeueInterval, kiali.GetNamespace(), kiali.GetName()))
			return common.RequeueAfter(patchKialiRequeueInterval)
		}
		return common.RequeueWithError(err)
	}

	if (grafanaEnabled && grafanaURL == "") || (jaegerEnabled && jaegerURL == "") {
		log.Info(fmt.Sprintf(
			"requeue patching Kiali after %s, because %s", patchKialiRequeueInterval,
			getReconciliationCause(grafanaEnabled, jaegerEnabled, grafanaURL, jaegerURL)))
		return common.RequeueAfter(patchKialiRequeueInterval)
	}

	return common.Reconciled()
}

func (r *controlPlaneInstanceReconciler) grafanaURL(ctx context.Context, log logr.Logger) string {
	log.Info("attempting to auto-detect Grafana for Kiali")
	grafanaRoute := &routev1.Route{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: "grafana", Namespace: r.Instance.GetNamespace()}, grafanaRoute)
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "error retrieving Grafana route - will disable Grafana in Kiali")
			// we aren't going to return here - Grafana is optional for Kiali; Kiali can still run without it
		}
		return ""
	}
	return getURLForRoute(grafanaRoute)
}

func (r *controlPlaneInstanceReconciler) jaegerURL(ctx context.Context, log logr.Logger) string {
	log.Info("attempting to auto-detect Jaeger for Kiali")
	if r.Instance.Status.AppliedSpec.Addons == nil || !r.Instance.Status.AppliedSpec.IsJaegerEnabled() {
		log.Info("Jaeger is not installed, disabling tracing in Kiali")
		return ""
	}
	jaegerName := "jaeger"
	if r.Instance.Status.AppliedSpec.Addons.Jaeger != nil {
		jaegerName = r.Instance.Status.AppliedSpec.Addons.Jaeger.ResourceName()
	}
	jaegerRoutes := &routev1.RouteList{}
	err := r.Client.List(ctx, jaegerRoutes,
		client.InNamespace(r.Instance.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance":  jaegerName,
			"app.kubernetes.io/component": "query-route",
		})
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "error retrieving Jaeger route - will disable it in Kiali")
			// we aren't going to return here - Grafana is optional for Kiali; Kiali can still run without it
		}
		return ""
	} else if len(jaegerRoutes.Items) == 0 {
		// no routes
		log.Info("could not locate Jaeger query route resource, disabling tracing in Kiali")
		return ""
	}
	// we'll just use the first.  there should only ever be one route
	return getURLForRoute(&jaegerRoutes.Items[0])
}

func getURLForRoute(route *routev1.Route) string {
	routeURL := route.Spec.Host
	if routeURL != "" {
		routeScheme := "http"
		if route.Spec.TLS != nil {
			routeScheme = "https"
		}
		routeURL = fmt.Sprintf("%s://%s", routeScheme, routeURL)
	}
	return routeURL
}

func errorOnSettingValueInKialiCR(fieldPath string, err error) error {
	return fmt.Errorf("could not set %s in kiali CR: %s", fieldPath, err)
}

func getReconciliationCause(grafanaEnabled, jaegerEnabled bool, grafanaURL, jaegerURL string) string {
	if grafanaEnabled && grafanaURL == "" && jaegerEnabled && jaegerURL == "" {
		return "Grafana and Jaeger routes do not exist"
	}
	if grafanaEnabled && grafanaURL == "" {
		return "Grafana route does not exist"
	}
	if jaegerEnabled && jaegerURL == "" {
		return "Jaeger route does not exist"
	}
	return ""
}
